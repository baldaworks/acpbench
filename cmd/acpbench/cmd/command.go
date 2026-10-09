package command

import (
	"fmt"
	"os"
	"time"

	"github.com/baldaworks/acpbench/internal/apps/acpbench"
	"github.com/baldaworks/acpbench/internal/apps/acpbench/report"
	"github.com/baldaworks/acpbench/internal/logging"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// Command builds and returns the root cobra command for acpbench.
func Command() *cobra.Command {
	var (
		prompt          string
		model           string
		reasoningEffort string
		iterations      int
		warmupRuns      int
		timeout         time.Duration
		jsonOutput      bool
		debugLogs       bool
	)

	cmd := &cobra.Command{
		Use:          "acpbench [flags] -- <acp-server-cmd> [args...]",
		Short:        "Benchmark any stdio ACP server command",
		Long:         "Start a stdio ACP server command, establish a session, run benchmark prompt turn(s), measure phase timings (spawn, init, session, TTFT, tool wall time, prompt), and output structured performance metrics.",
		SilenceUsage: true,
		Args:         cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			serverCommand, err := requireACPCommandAfterDash(cmd, args)
			if err != nil {
				return err
			}

			workingDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			logLevel := logging.LevelInfo
			if debugLogs {
				logLevel = logging.LevelDebug
			}
			if err := logging.Init(logging.WithLevel(logLevel)); err != nil {
				return fmt.Errorf("initialize logging: %w", err)
			}
			ctx := log.Logger.With().Str("component", "tool.acpbench").Logger().WithContext(cmd.Context())

			rep, err := acpbench.RunBenchmark(ctx, acpbench.BenchmarkConfig{
				Command:         serverCommand,
				WorkingDir:      workingDir,
				Prompt:          prompt,
				Model:           model,
				ReasoningEffort: reasoningEffort,
				WarmupRuns:      warmupRuns,
				Iterations:      iterations,
				Timeout:         timeout,
				JSONOutput:      jsonOutput,
				Stdout:          cmd.OutOrStdout(),
				Stderr:          cmd.ErrOrStderr(),
			})
			if err != nil {
				return err
			}

			if jsonOutput {
				return report.WriteJSON(cmd.OutOrStdout(), rep)
			}
			return report.WriteHuman(cmd.OutOrStdout(), rep)
		},
	}

	cmd.Flags().StringVarP(&prompt, "prompt", "p", "Hello! Reply with 'ready' and nothing else.", "prompt string sent to the ACP agent")
	cmd.Flags().StringVarP(&model, "model", "m", "", "model identifier to configure on the session")
	cmd.Flags().StringVarP(&reasoningEffort, "reasoning-effort", "r", "", "reasoning effort level (e.g. low, medium, high)")
	cmd.Flags().IntVarP(&iterations, "iterations", "n", 1, "number of measured benchmark iterations")
	cmd.Flags().IntVarP(&warmupRuns, "warmup", "w", 0, "number of unmeasured warmup iterations")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 60*time.Second, "timeout per benchmark iteration")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print output as JSON")
	cmd.Flags().BoolVar(&debugLogs, "debug", false, "enable debug logging to stderr")

	cmd.Example = "  acpbench -- opencode acp\n  acpbench -n 3 -w 1 --model claude-3-7-sonnet -- npx -y @zed-industries/claude-code-acp@latest\n  acpbench --json -n 5 --reasoning-effort high -- npx -y @normahq/codex-acp-bridge@latest"

	return cmd
}

func requireACPCommandAfterDash(cmd *cobra.Command, args []string) ([]string, error) {
	dashIndex := cmd.ArgsLenAtDash()
	if dashIndex < 0 {
		return nil, fmt.Errorf("missing command delimiter --; pass ACP server command after --")
	}
	if dashIndex > 0 {
		return nil, fmt.Errorf("arguments before -- are not allowed; pass ACP server command only after --")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("acp server command is required after --")
	}
	return append([]string(nil), args...), nil
}
