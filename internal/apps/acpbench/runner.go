package acpbench

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/baldaworks/acpbench/internal/acpclient"
	"github.com/baldaworks/acpbench/internal/apps/appio"
	"github.com/baldaworks/acpbench/internal/logging"
	"github.com/rs/zerolog"
)

// BenchmarkConfig defines options for a benchmark execution.
type BenchmarkConfig struct {
	Command         []string
	WorkingDir      string
	Prompt          string
	Model           string
	ReasoningEffort string
	WarmupRuns      int
	Iterations      int
	Timeout         time.Duration
	JSONOutput      bool
	Stdout          io.Writer
	Stderr          io.Writer
}

// RunBenchmark executes warmup runs followed by measured iterations and returns the complete report.
func RunBenchmark(ctx context.Context, cfg BenchmarkConfig) (*BenchmarkReport, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("acp server command is required")
	}
	if cfg.Iterations <= 0 {
		cfg.Iterations = 1
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if strings.TrimSpace(cfg.Prompt) == "" {
		cfg.Prompt = "Hello! Reply with 'ready' and nothing else."
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}

	lockedStderr := appio.NewSyncWriter(cfg.Stderr)
	logger := logging.Ctx(ctx)
	if logging.DebugEnabled() {
		stderrLogger := logger.Output(lockedStderr)
		logger = &stderrLogger
	}

	report := &BenchmarkReport{
		Command:         append([]string(nil), cfg.Command...),
		Prompt:          cfg.Prompt,
		Model:           cfg.Model,
		ReasoningEffort: cfg.ReasoningEffort,
		WarmupRuns:      cfg.WarmupRuns,
		Iterations:      cfg.Iterations,
		Runs:            make([]IterationMetrics, 0, cfg.Iterations),
	}

	// 1. Warmup runs (not saved in report.Runs)
	for w := 1; w <= cfg.WarmupRuns; w++ {
		logger.Debug().Int("warmup_run", w).Msg("starting warmup run")
		_, err := executeIteration(ctx, cfg, w, lockedStderr, logger)
		if err != nil {
			logger.Warn().Err(err).Int("warmup_run", w).Msg("warmup run failed")
		}
	}

	// 2. Measured benchmark runs
	for i := 1; i <= cfg.Iterations; i++ {
		logger.Debug().Int("iteration", i).Msg("starting measured benchmark run")
		metrics, err := executeIteration(ctx, cfg, i, lockedStderr, logger)
		if err != nil {
			metrics.Error = err.Error()
			logger.Error().Err(err).Int("iteration", i).Msg("benchmark run failed")
		}
		report.Runs = append(report.Runs, metrics)
	}

	report.Summary = ComputeSummary(report.Runs)
	report.ToolsSummary = ComputeToolsSummary(report.Runs)

	return report, nil
}

func executeIteration(ctx context.Context, cfg BenchmarkConfig, iterationNum int, stderr io.Writer, logger *zerolog.Logger) (metrics IterationMetrics, err error) {
	metrics.Iteration = iterationNum
	iterCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	var (
		mu           sync.Mutex
		firstChunk   time.Time
		chunkCount   int
		totalChars   int
	)

	t0 := time.Now()

	// 1. Spawn Process & Setup Client
	spawnStart := time.Now()
	client, err := acpclient.New(iterCtx, acpclient.Config{
		Command:    cfg.Command,
		WorkingDir: cfg.WorkingDir,
		Stderr:     stderr,
		Debug:      logging.DebugEnabled(),
		OnSessionUpdate: func(notif acp.SessionNotification) {
			mu.Lock()
			defer mu.Unlock()

			if notif.Update.AgentMessageChunk != nil {
				chunkCount++
				if firstChunk.IsZero() {
					firstChunk = time.Now()
				}
				if notif.Update.AgentMessageChunk.Content.Text != nil {
					totalChars += len(notif.Update.AgentMessageChunk.Content.Text.Text)
				}
			} else if notif.Update.AgentThoughtChunk != nil {
				chunkCount++
				if firstChunk.IsZero() {
					firstChunk = time.Now()
				}
				if notif.Update.AgentThoughtChunk.Content.Text != nil {
					totalChars += len(notif.Update.AgentThoughtChunk.Content.Text.Text)
				}
			}
		},
	})
	if err != nil {
		return metrics, fmt.Errorf("spawn acp client: %w", err)
	}
	metrics.SpawnDuration = DurationMillis(time.Since(spawnStart))

	defer func() {
		_ = client.Close()
	}()

	// 2. ACP Initialize Handshake
	initStart := time.Now()
	_, err = client.Initialize(iterCtx)
	if err != nil {
		return metrics, fmt.Errorf("initialize acp client: %w", err)
	}
	metrics.InitDuration = DurationMillis(time.Since(initStart))

	// 3. ACP NewSession
	sessionStart := time.Now()
	sessionResp, err := client.NewSession(iterCtx, cfg.WorkingDir)
	if err != nil {
		return metrics, fmt.Errorf("create acp session: %w", err)
	}
	metrics.SessionDuration = DurationMillis(time.Since(sessionStart))

	// 4. Session Configuration (Model & Reasoning Effort)
	configStart := time.Now()
	if cfg.Model != "" || cfg.ReasoningEffort != "" {
		if err := applySessionConfig(iterCtx, client, sessionResp, cfg.Model, cfg.ReasoningEffort); err != nil {
			logger.Warn().Err(err).Msg("failed to apply session config option")
		}
		metrics.ConfigDuration = DurationMillis(time.Since(configStart))
	}

	// 5. Prompt Turn
	var meta map[string]any
	if cfg.ReasoningEffort != "" {
		meta = map[string]any{
			"reasoning_effort": cfg.ReasoningEffort,
		}
	}

	promptStart := time.Now()
	promptResp, err := client.Prompt(iterCtx, sessionResp.SessionId, cfg.Prompt, meta)
	promptEnd := time.Now()
	promptDur := promptEnd.Sub(promptStart)
	metrics.PromptDuration = DurationMillis(promptDur)

	// Finalize tool tracker to measure completion
	client.ToolTracker().Finalize(promptEnd)

	mu.Lock()
	metrics.ChunkCount = chunkCount
	if !firstChunk.IsZero() {
		metrics.TTFT = DurationMillis(firstChunk.Sub(promptStart))
	}
	// Determine output token count
	tokens := 0
	switch {
	case promptResp.Usage != nil && promptResp.Usage.OutputTokens > 0:
		tokens = promptResp.Usage.OutputTokens
		if promptResp.Usage.ThoughtTokens != nil {
			tokens += *promptResp.Usage.ThoughtTokens
		}
	case totalChars > 0:
		// Estimate ~4 chars per token if server doesn't report per-turn output tokens
		tokens = int(math.Ceil(float64(totalChars) / 4.0))
	}
	metrics.OutputTokens = tokens

	// Calculate generation duration: prompt duration minus TTFT (or total prompt if TTFT is 0)
	generationDur := promptDur
	if !firstChunk.IsZero() && promptEnd.After(firstChunk) {
		generationDur = promptEnd.Sub(firstChunk)
	}
	if tokens > 0 && generationDur > 0 {
		metrics.TPS = float64(tokens) / generationDur.Seconds()
	}
	mu.Unlock()

	// Tool Wall Time & breakdown
	metrics.ToolWallTime = DurationMillis(client.ToolTracker().CalculateWallTime())
	metrics.ToolMetrics = aggregateToolExecutions(client.ToolTracker().GetHistory())

	if err != nil {
		return metrics, fmt.Errorf("prompt acp agent: %w", err)
	}

	metrics.StopReason = string(promptResp.StopReason)
	metrics.TotalDuration = DurationMillis(time.Since(t0))

	return metrics, nil
}

func applySessionConfig(ctx context.Context, client *acpclient.Client, sessionResp acp.NewSessionResponse, model, reasoningEffort string) error {
	for _, opt := range sessionResp.ConfigOptions {
		if opt.Select == nil {
			continue
		}
		optID := strings.ToLower(string(opt.Select.Id))
		optName := strings.ToLower(opt.Select.Name)

		// Model match
		if model != "" && (strings.Contains(optID, "model") || strings.Contains(optName, "model")) {
			valueID := findConfigValue(opt.Select.Options, model)
			if _, err := client.SetSessionConfigOption(ctx, sessionResp.SessionId, opt.Select.Id, valueID); err != nil {
				return fmt.Errorf("set model option: %w", err)
			}
		}

		// Reasoning effort match
		if reasoningEffort != "" && (strings.Contains(optID, "reasoning") || strings.Contains(optID, "thinking") || strings.Contains(optID, "effort") ||
			strings.Contains(optName, "reasoning") || strings.Contains(optName, "thinking") || strings.Contains(optName, "effort")) {
			valueID := findConfigValue(opt.Select.Options, reasoningEffort)
			if _, err := client.SetSessionConfigOption(ctx, sessionResp.SessionId, opt.Select.Id, valueID); err != nil {
				return fmt.Errorf("set reasoning effort option: %w", err)
			}
		}
	}
	return nil
}

func findConfigValue(options acp.SessionConfigSelectOptions, target string) acp.SessionConfigValueId {
	targetLower := strings.ToLower(strings.TrimSpace(target))

	if options.Ungrouped != nil {
		for _, item := range *options.Ungrouped {
			if strings.ToLower(string(item.Value)) == targetLower || strings.ToLower(item.Name) == targetLower {
				return item.Value
			}
		}
	}

	if options.Grouped != nil {
		for _, group := range *options.Grouped {
			for _, item := range group.Options {
				if strings.ToLower(string(item.Value)) == targetLower || strings.ToLower(item.Name) == targetLower {
					return item.Value
				}
			}
		}
	}

	return acp.SessionConfigValueId(target)
}

func aggregateToolExecutions(history []acpclient.ToolExecution) map[string]ToolExecutionMetric {
	if len(history) == 0 {
		return nil
	}
	res := make(map[string]ToolExecutionMetric)
	for _, h := range history {
		existing, ok := res[h.ToolName]
		if !ok {
			existing = ToolExecutionMetric{
				ToolName: h.ToolName,
			}
		}
		existing.CallCount++
		existing.TotalDuration += DurationMillis(h.Duration)
		res[h.ToolName] = existing
	}

	for k, v := range res {
		if v.CallCount > 0 {
			v.AvgDuration = DurationMillis(time.Duration(v.TotalDuration) / time.Duration(v.CallCount))
			res[k] = v
		}
	}
	return res
}
