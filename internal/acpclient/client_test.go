package acpclient

import (
	"context"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func TestNew_EmptyCommand(t *testing.T) {
	_, err := New(context.Background(), Config{})
	if err == nil {
		t.Fatalf("expected error for empty command")
	}
}

func TestToolTracker(t *testing.T) {
	tracker := NewToolTracker()
	t0 := time.Now()

	callID := acp.ToolCallId("call-1")
	tracker.RecordToolStart(&acp.SessionUpdateToolCall{
		ToolCallId: callID,
		Title:      "read_file",
		Kind:       acp.ToolKindRead,
		Status:     acp.ToolCallStatusInProgress,
	})

	time.Sleep(10 * time.Millisecond)

	statusCompleted := acp.ToolCallStatusCompleted
	tracker.RecordToolUpdate(&acp.SessionToolCallUpdate{
		ToolCallId: callID,
		Status:     &statusCompleted,
	})

	history := tracker.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 history item, got %d", len(history))
	}
	if history[0].ToolName != "read_file" {
		t.Errorf("expected tool name 'read_file', got %s", history[0].ToolName)
	}
	if history[0].Duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", history[0].Duration)
	}

	wallTime := tracker.CalculateWallTime()
	if wallTime < 10*time.Millisecond {
		t.Errorf("expected wall time >= 10ms, got %v", wallTime)
	}

	// Test finalization
	callID2 := acp.ToolCallId("call-2")
	tracker.RecordToolStart(&acp.SessionUpdateToolCall{
		ToolCallId: callID2,
		Title:      "bash",
		Kind:       acp.ToolKindExecute,
		Status:     acp.ToolCallStatusInProgress,
	})
	tracker.Finalize(t0.Add(50 * time.Millisecond))

	history2 := tracker.GetHistory()
	if len(history2) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(history2))
	}
}
