package acpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

const (
	defaultClientName    = "acpbench"
	defaultClientVersion = "dev"
	closeTimeout         = 500 * time.Millisecond
)

// ToolExecution represents timing and information for a single tool call.
type ToolExecution struct {
	ToolCallID string
	ToolName   string
	StartedAt  time.Time
	EndedAt    time.Time
	Duration   time.Duration
	Status     string
}

// ToolTimeInterval represents a start and end time interval for tool activity.
type ToolTimeInterval struct {
	Start time.Time
	End   time.Time
}

// ToolTracker tracks active and completed tool calls thread-safely.
type ToolTracker struct {
	mu           sync.Mutex
	activeTools  map[acp.ToolCallId]*ToolExecution
	history      []ToolExecution
	rawIntervals []ToolTimeInterval
}

func NewToolTracker() *ToolTracker {
	return &ToolTracker{
		activeTools: make(map[acp.ToolCallId]*ToolExecution),
	}
}

func (t *ToolTracker) RecordToolStart(call *acp.SessionUpdateToolCall) {
	if call == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	name := strings.TrimSpace(call.Title)
	if name == "" {
		name = strings.TrimSpace(string(call.Kind))
	}
	if name == "" {
		name = "unknown_tool"
	}

	exec := &ToolExecution{
		ToolCallID: string(call.ToolCallId),
		ToolName:   name,
		StartedAt:  now,
		Status:     string(call.Status),
	}
	t.activeTools[call.ToolCallId] = exec
}

func (t *ToolTracker) RecordToolUpdate(update *acp.SessionToolCallUpdate) {
	if update == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	exec, exists := t.activeTools[update.ToolCallId]
	if !exists {
		return
	}

	if update.Title != nil && strings.TrimSpace(*update.Title) != "" {
		exec.ToolName = strings.TrimSpace(*update.Title)
	}

	if update.Status != nil {
		exec.Status = string(*update.Status)
		statusLower := strings.ToLower(string(*update.Status))
		if statusLower == "completed" || statusLower == "failed" || statusLower == "cancelled" {
			now := time.Now()
			exec.EndedAt = now
			exec.Duration = now.Sub(exec.StartedAt)
			delete(t.activeTools, update.ToolCallId)
			t.history = append(t.history, *exec)
			t.rawIntervals = append(t.rawIntervals, ToolTimeInterval{Start: exec.StartedAt, End: now})
		}
	}
}

// Finalize closes any remaining active tool calls at the given completion time.
func (t *ToolTracker) Finalize(completionTime time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for id, exec := range t.activeTools {
		exec.EndedAt = completionTime
		exec.Duration = completionTime.Sub(exec.StartedAt)
		exec.Status = "ended_with_turn"
		delete(t.activeTools, id)
		t.history = append(t.history, *exec)
		t.rawIntervals = append(t.rawIntervals, ToolTimeInterval{Start: exec.StartedAt, End: completionTime})
	}
}

// GetHistory returns a copy of all recorded tool executions.
func (t *ToolTracker) GetHistory() []ToolExecution {
	t.mu.Lock()
	defer t.mu.Unlock()

	res := make([]ToolExecution, len(t.history))
	copy(res, t.history)
	return res
}

// CalculateWallTime computes the merged union duration of all tool execution intervals.
func (t *ToolTracker) CalculateWallTime() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.rawIntervals) == 0 {
		return 0
	}

	// Make a copy of intervals
	intervals := make([]ToolTimeInterval, len(t.rawIntervals))
	copy(intervals, t.rawIntervals)

	// Sort intervals by start time
	for i := 0; i < len(intervals)-1; i++ {
		for j := i + 1; j < len(intervals); j++ {
			if intervals[i].Start.After(intervals[j].Start) {
				intervals[i], intervals[j] = intervals[j], intervals[i]
			}
		}
	}

	// Merge overlapping intervals
	var total time.Duration
	currentStart := intervals[0].Start
	currentEnd := intervals[0].End

	for _, interval := range intervals[1:] {
		if !interval.Start.After(currentEnd) {
			// Overlaps or touches
			if interval.End.After(currentEnd) {
				currentEnd = interval.End
			}
		} else {
			// Gap, add duration
			total += currentEnd.Sub(currentStart)
			currentStart = interval.Start
			currentEnd = interval.End
		}
	}
	total += currentEnd.Sub(currentStart)
	return total
}

// Config describes how to start and configure the ACP client.
type Config struct {
	Command         []string
	WorkingDir      string
	ClientName      string
	ClientVersion   string
	Stderr          io.Writer
	Debug           bool
	OnSessionUpdate func(notif acp.SessionNotification)
}

// Client manages an ACP subprocess and communicates via stdio using acp-go-sdk.
type Client struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	conn          *acp.ClientSideConnection
	clientName    string
	clientVersion string

	toolTracker *ToolTracker

	closeOnce sync.Once
	closeErr  error
}

// New starts an ACP subprocess and establishes an ACP connection.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if len(cfg.Command) == 0 {
		return nil, errors.New("acp command is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	clientName := strings.TrimSpace(cfg.ClientName)
	if clientName == "" {
		clientName = defaultClientName
	}
	clientVersion := strings.TrimSpace(cfg.ClientVersion)
	if clientVersion == "" {
		clientVersion = defaultClientVersion
	}

	stderr := cfg.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	cmd := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	cmd.Dir = cfg.WorkingDir
	cmd.Stderr = stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("acp stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("acp stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start acp process: %w", err)
	}

	tracker := NewToolTracker()
	clientHandler := &benchClientHandler{
		onSessionUpdate: cfg.OnSessionUpdate,
		toolTracker:     tracker,
	}

	conn := acp.NewClientSideConnection(clientHandler, stdin, stdout)
	conn.SetLogger(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})))

	return &Client{
		cmd:           cmd,
		stdin:         stdin,
		conn:          conn,
		clientName:    clientName,
		clientVersion: clientVersion,
		toolTracker:   tracker,
	}, nil
}

// ToolTracker returns the client's tool tracker.
func (c *Client) ToolTracker() *ToolTracker {
	return c.toolTracker
}

// Initialize performs ACP initialization.
func (c *Client) Initialize(ctx context.Context) (acp.InitializeResponse, error) {
	return c.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientInfo: &acp.Implementation{
			Name:    c.clientName,
			Version: c.clientVersion,
		},
	})
}

// NewSession creates a new session in the specified working directory.
func (c *Client) NewSession(ctx context.Context, cwd string) (acp.NewSessionResponse, error) {
	return c.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: []acp.McpServer{},
	})
}

// SetSessionConfigOption sets a configuration option on the session.
func (c *Client) SetSessionConfigOption(ctx context.Context, sessionID acp.SessionId, configID acp.SessionConfigId, valueID acp.SessionConfigValueId) (acp.SetSessionConfigOptionResponse, error) {
	req := acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{
			SessionId: sessionID,
			ConfigId:  configID,
			Value:     valueID,
		},
	}
	return c.conn.SetSessionConfigOption(ctx, req)
}

// Prompt dispatches a user prompt to the ACP agent session.
func (c *Client) Prompt(ctx context.Context, sessionID acp.SessionId, promptText string, meta map[string]any) (acp.PromptResponse, error) {
	req := acp.PromptRequest{
		SessionId: sessionID,
		Prompt: []acp.ContentBlock{
			acp.TextBlock(promptText),
		},
		Meta: meta,
	}
	return c.conn.Prompt(ctx, req)
}

// Close gracefully closes the subprocess and connection.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		if c.stdin != nil {
			_ = c.stdin.Close()
		}

		done := make(chan struct{})
		go func() {
			if c.cmd != nil {
				_ = c.cmd.Wait()
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(closeTimeout):
			if c.cmd != nil && c.cmd.Process != nil {
				_ = c.cmd.Process.Kill()
			}
			<-done
		}
	})
	return c.closeErr
}

type benchClientHandler struct {
	onSessionUpdate func(notif acp.SessionNotification)
	toolTracker     *ToolTracker
}

var _ acp.Client = (*benchClientHandler)(nil)

func (h *benchClientHandler) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsReadTextFile)
}

func (h *benchClientHandler) WriteTextFile(_ context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, acp.NewMethodNotFound(acp.ClientMethodFsWriteTextFile)
}

func (h *benchClientHandler) RequestPermission(_ context.Context, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

func (h *benchClientHandler) SessionUpdate(_ context.Context, notif acp.SessionNotification) error {
	if notif.Update.ToolCall != nil && h.toolTracker != nil {
		h.toolTracker.RecordToolStart(notif.Update.ToolCall)
	}
	if notif.Update.ToolCallUpdate != nil && h.toolTracker != nil {
		h.toolTracker.RecordToolUpdate(notif.Update.ToolCallUpdate)
	}
	if h.onSessionUpdate != nil {
		h.onSessionUpdate(notif)
	}
	return nil
}

func (h *benchClientHandler) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}

func (h *benchClientHandler) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}

func (h *benchClientHandler) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}

func (h *benchClientHandler) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}

func (h *benchClientHandler) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}
