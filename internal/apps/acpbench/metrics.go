package acpbench

import (
	"encoding/json"
	"math"
	"time"
)

// DurationMillis wraps time.Duration to serialize as float milliseconds in JSON.
type DurationMillis time.Duration

func (d DurationMillis) MarshalJSON() ([]byte, error) {
	ms := float64(time.Duration(d).Microseconds()) / 1000.0
	return json.Marshal(ms)
}

func (d *DurationMillis) UnmarshalJSON(b []byte) error {
	var ms float64
	if err := json.Unmarshal(b, &ms); err != nil {
		return err
	}
	*d = DurationMillis(time.Duration(ms * float64(time.Millisecond)))
	return nil
}

func (d DurationMillis) Duration() time.Duration {
	return time.Duration(d)
}

func (d DurationMillis) Microseconds() int64 {
	return time.Duration(d).Microseconds()
}

// ToolExecutionMetric stores aggregated metrics for a specific tool.
type ToolExecutionMetric struct {
	ToolName      string         `json:"tool_name"`
	CallCount     int            `json:"call_count"`
	TotalDuration DurationMillis `json:"total_duration_ms"`
	AvgDuration   DurationMillis `json:"avg_duration_ms"`
}

// IterationMetrics stores timings, tokens, and outcome for one benchmark run.
type IterationMetrics struct {
	Iteration       int                            `json:"iteration"`
	SpawnDuration   DurationMillis                 `json:"spawn_duration_ms"`
	InitDuration    DurationMillis                 `json:"init_duration_ms"`
	SessionDuration DurationMillis                 `json:"session_duration_ms"`
	ConfigDuration  DurationMillis                 `json:"config_duration_ms,omitempty"`
	TTFT            DurationMillis                 `json:"ttft_ms"` // Time To First Token / Chunk
	ToolWallTime    DurationMillis                 `json:"tool_wall_duration_ms"`
	ToolMetrics     map[string]ToolExecutionMetric `json:"tool_metrics,omitempty"`
	PromptDuration  DurationMillis                 `json:"prompt_duration_ms"`
	TotalDuration   DurationMillis                 `json:"total_duration_ms"`
	ChunkCount      int                            `json:"chunk_count"`
	OutputTokens    int                            `json:"output_tokens"`
	TPS             float64                        `json:"tps"` // Tokens Per Second
	StopReason      string                         `json:"stop_reason,omitempty"`
	Error           string                         `json:"error,omitempty"`
}

// SummaryStats contains min, mean, max, and standard deviation in milliseconds or scalar value.
type SummaryStats struct {
	Min    float64 `json:"min"`
	Mean   float64 `json:"mean"`
	Max    float64 `json:"max"`
	StdDev float64 `json:"stddev"`
}

// BenchmarkReport encapsulates complete benchmark results.
type BenchmarkReport struct {
	Command         []string                `json:"command"`
	Prompt          string                  `json:"prompt"`
	Model           string                  `json:"model,omitempty"`
	ReasoningEffort string                  `json:"reasoning_effort,omitempty"`
	WarmupRuns      int                     `json:"warmup_runs"`
	Iterations      int                     `json:"iterations"`
	Runs            []IterationMetrics      `json:"runs"`
	Summary         map[string]SummaryStats `json:"summary,omitempty"`
	ToolsSummary    map[string]SummaryStats `json:"tools_summary,omitempty"`
}

// ComputeSummary calculates statistics across measured runs.
func ComputeSummary(runs []IterationMetrics) map[string]SummaryStats {
	if len(runs) == 0 {
		return nil
	}

	spawnValues := make([]float64, 0, len(runs))
	initValues := make([]float64, 0, len(runs))
	sessionValues := make([]float64, 0, len(runs))
	configValues := make([]float64, 0, len(runs))
	ttftValues := make([]float64, 0, len(runs))
	toolWallValues := make([]float64, 0, len(runs))
	promptValues := make([]float64, 0, len(runs))
	totalValues := make([]float64, 0, len(runs))
	tpsValues := make([]float64, 0, len(runs))

	for _, r := range runs {
		if r.Error != "" {
			continue
		}
		spawnValues = append(spawnValues, toMillis(r.SpawnDuration))
		initValues = append(initValues, toMillis(r.InitDuration))
		sessionValues = append(sessionValues, toMillis(r.SessionDuration))
		if r.ConfigDuration > 0 {
			configValues = append(configValues, toMillis(r.ConfigDuration))
		}
		ttftValues = append(ttftValues, toMillis(r.TTFT))
		toolWallValues = append(toolWallValues, toMillis(r.ToolWallTime))
		promptValues = append(promptValues, toMillis(r.PromptDuration))
		totalValues = append(totalValues, toMillis(r.TotalDuration))
		if r.TPS > 0 {
			tpsValues = append(tpsValues, r.TPS)
		}
	}

	if len(totalValues) == 0 {
		return nil
	}

	summary := make(map[string]SummaryStats)
	summary["spawn"] = calcStats(spawnValues)
	summary["initialize"] = calcStats(initValues)
	summary["new_session"] = calcStats(sessionValues)
	if len(configValues) > 0 {
		summary["config"] = calcStats(configValues)
	}
	summary["ttft"] = calcStats(ttftValues)
	summary["tool_wall"] = calcStats(toolWallValues)
	summary["prompt"] = calcStats(promptValues)
	summary["total"] = calcStats(totalValues)
	if len(tpsValues) > 0 {
		summary["tps"] = calcStats(tpsValues)
	}

	return summary
}

// ComputeToolsSummary aggregates total tool wall duration per tool across runs.
func ComputeToolsSummary(runs []IterationMetrics) map[string]SummaryStats {
	toolMap := make(map[string][]float64)
	for _, r := range runs {
		if r.Error != "" {
			continue
		}
		for name, metric := range r.ToolMetrics {
			toolMap[name] = append(toolMap[name], toMillis(metric.TotalDuration))
		}
	}

	if len(toolMap) == 0 {
		return nil
	}

	res := make(map[string]SummaryStats)
	for name, values := range toolMap {
		res[name] = calcStats(values)
	}
	return res
}

func toMillis(d DurationMillis) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func calcStats(values []float64) SummaryStats {
	if len(values) == 0 {
		return SummaryStats{}
	}
	minVal := values[0]
	maxVal := values[0]
	sum := 0.0

	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		sum += v
	}

	mean := sum / float64(len(values))
	if len(values) == 1 {
		return SummaryStats{
			Min:    minVal,
			Mean:   mean,
			Max:    maxVal,
			StdDev: 0.0,
		}
	}

	varianceSum := 0.0
	for _, v := range values {
		diff := v - mean
		varianceSum += diff * diff
	}
	stdDev := math.Sqrt(varianceSum / float64(len(values)))

	return SummaryStats{
		Min:    minVal,
		Mean:   mean,
		Max:    maxVal,
		StdDev: stdDev,
	}
}
