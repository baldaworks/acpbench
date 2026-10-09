package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/baldaworks/acpbench/internal/apps/acpbench"
)

func TestWriteJSONAndHuman(t *testing.T) {
	rep := &acpbench.BenchmarkReport{
		Command:         []string{"opencode", "acp"},
		Prompt:          "Hello! Reply with ready.",
		Model:           "claude-3-7-sonnet",
		ReasoningEffort: "high",
		WarmupRuns:      1,
		Iterations:      2,
		Runs: []acpbench.IterationMetrics{
			{
				Iteration:       1,
				SpawnDuration:   acpbench.DurationMillis(12 * time.Millisecond),
				InitDuration:    acpbench.DurationMillis(45 * time.Millisecond),
				SessionDuration: acpbench.DurationMillis(18 * time.Millisecond),
				ConfigDuration:  acpbench.DurationMillis(5 * time.Millisecond),
				TTFT:            acpbench.DurationMillis(160 * time.Millisecond),
				ToolWallTime:    acpbench.DurationMillis(95 * time.Millisecond),
				PromptDuration:  acpbench.DurationMillis(340 * time.Millisecond),
				TotalDuration:   acpbench.DurationMillis(421 * time.Millisecond),
				ChunkCount:      16,
				StopReason:      "end_turn",
				ToolMetrics: map[string]acpbench.ToolExecutionMetric{
					"read_file": {
						ToolName:      "read_file",
						CallCount:     2,
						TotalDuration: acpbench.DurationMillis(64 * time.Millisecond),
						AvgDuration:   acpbench.DurationMillis(32 * time.Millisecond),
					},
				},
			},
		},
		Summary: map[string]acpbench.SummaryStats{
			"spawn": {Min: 12.0, Mean: 12.0, Max: 12.0, StdDev: 0.0},
		},
	}

	// 1. JSON test
	var jsonBuf bytes.Buffer
	if err := WriteJSON(&jsonBuf, rep); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
	jsonStr := jsonBuf.String()
	if !strings.Contains(jsonStr, `"opencode"`) || !strings.Contains(jsonStr, `"claude-3-7-sonnet"`) {
		t.Errorf("expected json content, got %s", jsonStr)
	}

	// 2. Human test
	var humanBuf bytes.Buffer
	if err := WriteHuman(&humanBuf, rep); err != nil {
		t.Fatalf("WriteHuman failed: %v", err)
	}
	humanStr := humanBuf.String()
	if !strings.Contains(humanStr, "Target:           opencode acp") {
		t.Errorf("missing target in human output: %s", humanStr)
	}
	if !strings.Contains(humanStr, "Model:            claude-3-7-sonnet") {
		t.Errorf("missing model in human output: %s", humanStr)
	}
	if !strings.Contains(humanStr, "read_file") {
		t.Errorf("missing tool breakdown in human output: %s", humanStr)
	}
}
