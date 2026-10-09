package testmock

import (
	"context"
	"io"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// MockAgent implements acp.Agent for integration testing.
type MockAgent struct {
	conn *acp.AgentSideConnection
}

func (m *MockAgent) SetConn(conn *acp.AgentSideConnection) {
	m.conn = conn
}

func (m *MockAgent) Initialize(_ context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	title := "Mock Agent"
	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo: &acp.Implementation{
			Name:    "mock-agent",
			Version: "1.0.0",
			Title:   &title,
		},
		AgentCapabilities: acp.AgentCapabilities{},
	}, nil
}

func (m *MockAgent) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (m *MockAgent) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, nil
}

func (m *MockAgent) NewSession(_ context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	sessionID := acp.SessionId("test-session-1")
	return acp.NewSessionResponse{
		SessionId: sessionID,
		ConfigOptions: []acp.SessionConfigOption{
			acp.NewSessionConfigOptionSelect("model-a", acp.SessionConfigSelectOptions{
				Ungrouped: &acp.SessionConfigSelectOptionsUngrouped{
					{Value: "model-a", Name: "Model A"},
					{Value: "model-b", Name: "Model B"},
				},
			}),
		},
	}, nil
}

func (m *MockAgent) LoadSession(_ context.Context, _ acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	return acp.LoadSessionResponse{}, nil
}

func (m *MockAgent) ListSessions(_ context.Context, _ acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, nil
}

func (m *MockAgent) ResumeSession(_ context.Context, _ acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, nil
}

func (m *MockAgent) CloseSession(_ context.Context, _ acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}

func (m *MockAgent) SetSessionMode(_ context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func (m *MockAgent) SetSessionConfigOption(_ context.Context, _ acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

func (m *MockAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	if m.conn != nil {
		// Stream an agent thought chunk
		time.Sleep(5 * time.Millisecond)
		_ = m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: req.SessionId,
			Update:    acp.UpdateAgentThoughtText("Thinking..."),
		})

		// Stream a tool call
		callID := acp.ToolCallId("call-mock-1")
		time.Sleep(5 * time.Millisecond)
		_ = m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: req.SessionId,
			Update: acp.SessionUpdate{
				ToolCall: &acp.SessionUpdateToolCall{
					ToolCallId: callID,
					Title:      "mock_tool",
					Kind:       acp.ToolKindExecute,
					Status:     acp.ToolCallStatusInProgress,
				},
			},
		})

		time.Sleep(10 * time.Millisecond)
		statusComp := acp.ToolCallStatusCompleted
		_ = m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: req.SessionId,
			Update: acp.SessionUpdate{
				ToolCallUpdate: &acp.SessionToolCallUpdate{
					ToolCallId: callID,
					Status:     &statusComp,
				},
			},
		})

		// Stream agent message chunk
		time.Sleep(5 * time.Millisecond)
		_ = m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: req.SessionId,
			Update:    acp.UpdateAgentMessageText("Hello! ready"),
		})
	}

	return acp.PromptResponse{
		StopReason: acp.StopReasonEndTurn,
	}, nil
}

func (m *MockAgent) Cancel(_ context.Context, _ acp.CancelNotification) error {
	return nil
}

// RunStdioMock runs the mock agent on stdin and stdout until EOF.
func RunStdioMock(r io.Reader, w io.Writer) error {
	agent := &MockAgent{}
	conn := acp.NewAgentSideConnection(agent, w, r)
	agent.SetConn(conn)
	<-conn.Done()
	return nil
}
