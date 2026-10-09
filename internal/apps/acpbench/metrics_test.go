package acpbench

import (
	"testing"
	"time"
)

func TestComputeSummary(t *testing.T) {
	runs := []IterationMetrics{
		{
			Iteration:       1,
			SpawnDuration:   DurationMillis(10 * time.Millisecond),
			InitDuration:    DurationMillis(40 * time.Millisecond),
			SessionDuration: DurationMillis(20 * time.Millisecond),
			ConfigDuration:  DurationMillis(5 * time.Millisecond),
			TTFT:            DurationMillis(100 * time.Millisecond),
			ToolWallTime:    DurationMillis(50 * time.Millisecond),
			PromptDuration:  DurationMillis(300 * time.Millisecond),
			TotalDuration:   DurationMillis(370 * time.Millisecond),
		},
		{
			Iteration:       2,
			SpawnDuration:   DurationMillis(20 * time.Millisecond),
			InitDuration:    DurationMillis(50 * time.Millisecond),
			SessionDuration: DurationMillis(30 * time.Millisecond),
			ConfigDuration:  DurationMillis(7 * time.Millisecond),
			TTFT:            DurationMillis(150 * time.Millisecond),
			ToolWallTime:    DurationMillis(70 * time.Millisecond),
			PromptDuration:  DurationMillis(400 * time.Millisecond),
			TotalDuration:   DurationMillis(500 * time.Millisecond),
		},
	}

	summary := ComputeSummary(runs)
	if summary == nil {
		t.Fatalf("expected summary, got nil")
	}

	spawn := summary["spawn"]
	if spawn.Min != 10.0 || spawn.Max != 20.0 || spawn.Mean != 15.0 {
		t.Errorf("unexpected spawn stats: %+v", spawn)
	}

	toolWall := summary["tool_wall"]
	if toolWall.Min != 50.0 || toolWall.Max != 70.0 || toolWall.Mean != 60.0 {
		t.Errorf("unexpected tool_wall stats: %+v", toolWall)
	}

	config := summary["config"]
	if config.Min != 5.0 || config.Max != 7.0 || config.Mean != 6.0 {
		t.Errorf("unexpected config stats: %+v", config)
	}
}

func TestComputeToolsSummary(t *testing.T) {
	runs := []IterationMetrics{
		{
			Iteration: 1,
			ToolMetrics: map[string]ToolExecutionMetric{
				"read_file": {
					ToolName:      "read_file",
					CallCount:     2,
					TotalDuration: DurationMillis(20 * time.Millisecond),
				},
			},
		},
		{
			Iteration: 2,
			ToolMetrics: map[string]ToolExecutionMetric{
				"read_file": {
					ToolName:      "read_file",
					CallCount:     1,
					TotalDuration: DurationMillis(40 * time.Millisecond),
				},
			},
		},
	}

	toolsSummary := ComputeToolsSummary(runs)
	if toolsSummary == nil {
		t.Fatalf("expected tools summary, got nil")
	}
	rf := toolsSummary["read_file"]
	if rf.Min != 20.0 || rf.Max != 40.0 || rf.Mean != 30.0 {
		t.Errorf("unexpected read_file stats: %+v", rf)
	}
}
