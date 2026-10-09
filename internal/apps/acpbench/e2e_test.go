package acpbench_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/baldaworks/acpbench/internal/apps/acpbench"
)

func TestEndToEndBenchmark(t *testing.T) {
	tempDir := t.TempDir()
	mockBin := filepath.Join(tempDir, "mockagent")

	// Build mockagent binary
	buildCmd := exec.Command("go", "build", "-o", mockBin, "github.com/baldaworks/acpbench/cmd/mockagent")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build mockagent: %v, out: %s", err, string(out))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rep, err := acpbench.RunBenchmark(ctx, acpbench.BenchmarkConfig{
		Command:         []string{mockBin},
		WorkingDir:      tempDir,
		Prompt:          "Hello mock agent",
		Model:           "model-a",
		ReasoningEffort: "high",
		WarmupRuns:      1,
		Iterations:      2,
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunBenchmark failed: %v", err)
	}

	if len(rep.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(rep.Runs))
	}

	for i, r := range rep.Runs {
		if r.Error != "" {
			t.Fatalf("run %d had error: %s", i+1, r.Error)
		}
		if r.StopReason != "end_turn" {
			t.Errorf("run %d unexpected stop reason: %s", i+1, r.StopReason)
		}
		if r.ChunkCount < 2 {
			t.Errorf("run %d expected >= 2 chunks, got %d", i+1, r.ChunkCount)
		}
		if r.TTFT <= 0 {
			t.Errorf("run %d expected positive TTFT, got %v", i+1, r.TTFT)
		}
		if r.ToolWallTime <= 0 {
			t.Errorf("run %d expected positive ToolWallTime, got %v", i+1, r.ToolWallTime)
		}
		if toolMetric, ok := r.ToolMetrics["mock_tool"]; !ok || toolMetric.CallCount != 1 {
			t.Errorf("run %d expected 1 call for mock_tool, got %+v", i+1, toolMetric)
		}
	}

	if rep.Summary == nil {
		t.Fatalf("expected summary to be computed")
	}
	if rep.ToolsSummary == nil || len(rep.ToolsSummary) == 0 {
		t.Fatalf("expected tools summary to be computed")
	}
}
