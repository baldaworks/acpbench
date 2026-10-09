package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/baldaworks/acpbench/internal/apps/acpbench"
)

// WriteJSON formats the benchmark report as indented JSON.
func WriteJSON(w io.Writer, rep *acpbench.BenchmarkReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// WriteHuman formats the benchmark report in clean human-readable tables.
func WriteHuman(w io.Writer, rep *acpbench.BenchmarkReport) error {
	if rep == nil {
		_, err := fmt.Fprintln(w, "Benchmark report is empty.")
		return err
	}

	// 1. Header Information
	if _, err := fmt.Fprintf(w, "Target:           %s\n", strings.Join(rep.Command, " ")); err != nil {
		return err
	}
	if rep.Model != "" {
		if _, err := fmt.Fprintf(w, "Model:            %s\n", rep.Model); err != nil {
			return err
		}
	}
	if rep.ReasoningEffort != "" {
		if _, err := fmt.Fprintf(w, "Reasoning Effort: %s\n", rep.ReasoningEffort); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Prompt:           %q\n", rep.Prompt); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Warmup:           %d run(s)\n", rep.WarmupRuns); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Iterations:       %d run(s)\n\n", rep.Iterations); err != nil {
		return err
	}

	// 2. Benchmark Runs Table
	hasConfig := false
	for _, r := range rep.Runs {
		if r.ConfigDuration > 0 {
			hasConfig = true
			break
		}
	}

	if _, err := fmt.Fprintln(w, "Benchmark Runs:"); err != nil {
		return err
	}
	if hasConfig {
		if _, err := fmt.Fprintln(w, "Run  Spawn (ms)  Init (ms)  Session (ms)  Config (ms)  TTFT (ms)  Tool Wall (ms)  Prompt (ms)  Total (ms)  Tokens     TPS  Chunks  Stop Reason"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "Run  Spawn (ms)  Init (ms)  Session (ms)  TTFT (ms)  Tool Wall (ms)  Prompt (ms)  Total (ms)  Tokens     TPS  Chunks  Stop Reason"); err != nil {
			return err
		}
	}

	for _, r := range rep.Runs {
		if r.Error != "" {
			if _, err := fmt.Fprintf(w, "%-3d  ERROR: %s\n", r.Iteration, r.Error); err != nil {
				return err
			}
			continue
		}
		stopReason := r.StopReason
		if stopReason == "" {
			stopReason = "-"
		}

		if hasConfig {
			if _, err := fmt.Fprintf(w, "%-3d  %10.1f  %9.1f  %12.1f  %11.1f  %9.1f  %14.1f  %11.1f  %10.1f  %6d  %6.1f  %6d  %s\n",
				r.Iteration,
				toMillis(r.SpawnDuration),
				toMillis(r.InitDuration),
				toMillis(r.SessionDuration),
				toMillis(r.ConfigDuration),
				toMillis(r.TTFT),
				toMillis(r.ToolWallTime),
				toMillis(r.PromptDuration),
				toMillis(r.TotalDuration),
				r.OutputTokens,
				r.TPS,
				r.ChunkCount,
				stopReason,
			); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "%-3d  %10.1f  %9.1f  %12.1f  %9.1f  %14.1f  %11.1f  %10.1f  %6d  %6.1f  %6d  %s\n",
				r.Iteration,
				toMillis(r.SpawnDuration),
				toMillis(r.InitDuration),
				toMillis(r.SessionDuration),
				toMillis(r.TTFT),
				toMillis(r.ToolWallTime),
				toMillis(r.PromptDuration),
				toMillis(r.TotalDuration),
				r.OutputTokens,
				r.TPS,
				r.ChunkCount,
				stopReason,
			); err != nil {
				return err
			}
		}
	}

	// 3. Summary Statistics Table
	if rep.Summary != nil {
		if _, err := fmt.Fprintf(w, "\nSummary Statistics (%d measured runs):\n", len(rep.Runs)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Metric                Min (ms)     Mean (ms)      Max (ms)    StdDev (ms)"); err != nil {
			return err
		}

		orderedKeys := []struct {
			key   string
			label string
		}{
			{"spawn", "Spawn"},
			{"initialize", "Initialize"},
			{"new_session", "New Session"},
			{"config", "Config"},
			{"ttft", "Time to First Token"},
			{"tool_wall", "Tool Wall Time"},
			{"prompt", "Prompt Duration"},
			{"total", "Total Duration"},
			{"tps", "Tokens / Sec (TPS)"},
		}

		for _, item := range orderedKeys {
			stats, exists := rep.Summary[item.key]
			if !exists {
				continue
			}
			if _, err := fmt.Fprintf(w, "%-20s  %8.2f      %8.2f      %8.2f      %9.2f\n",
				item.label, stats.Min, stats.Mean, stats.Max, stats.StdDev); err != nil {
				return err
			}
		}
	}

	// 4. Per-Tool Execution Breakdown Table
	// Collect distinct tool calls across all runs
	toolAgg := make(map[string]struct {
		calls int
		wall  float64
	})
	for _, r := range rep.Runs {
		for name, m := range r.ToolMetrics {
			curr := toolAgg[name]
			curr.calls += m.CallCount
			curr.wall += toMillis(m.TotalDuration)
			toolAgg[name] = curr
		}
	}

	if len(toolAgg) > 0 {
		if _, err := fmt.Fprintln(w, "\nTool Execution Breakdown:"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "Tool Name             Calls    Total Wall (ms)    Avg (ms)"); err != nil {
			return err
		}

		names := make([]string, 0, len(toolAgg))
		for name := range toolAgg {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			agg := toolAgg[name]
			avg := 0.0
			if agg.calls > 0 {
				avg = agg.wall / float64(agg.calls)
			}
			if _, err := fmt.Fprintf(w, "%-20s  %5d         %10.2f  %10.2f\n",
				name, agg.calls, agg.wall, avg); err != nil {
				return err
			}
		}
	}

	return nil
}

func toMillis(d interface{ Microseconds() int64 }) float64 {
	return float64(d.Microseconds()) / 1000.0
}
