# acpbench

[![test](https://github.com/baldaworks/acpbench/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/baldaworks/acpbench/actions/workflows/test.yml)
[![lint](https://github.com/baldaworks/acpbench/actions/workflows/lint.yml/badge.svg?branch=main)](https://github.com/baldaworks/acpbench/actions/workflows/lint.yml)
[![security](https://github.com/baldaworks/acpbench/actions/workflows/security.yml/badge.svg?branch=main)](https://github.com/baldaworks/acpbench/actions/workflows/security.yml)
[![License](https://img.shields.io/github/license/baldaworks/acpbench)](LICENSE)

**Timing and performance benchmark for stdio ACP (Agent Client Protocol) servers.**

`acpbench` starts any stdio Agent Client Protocol (ACP) server command, establishes a session, applies optional model and reasoning configurations, sends benchmark prompt turn(s), gathers timing and throughput metrics, and outputs structured performance tables or JSON.

## What You Get

| Metric / Capability | Description |
| --- | --- |
| **Process Spawn Latency** | Time taken to spawn and connect stdio pipes to the ACP server process. |
| **Initialize Latency** | Duration of the ACP `Initialize` handshake RPC. |
| **Session Creation** | Duration of the `NewSession` RPC call. |
| **Session Configuration** | Configures `--model` and `--reasoning-effort` via `SetSessionConfigOption`. |
| **TTFT (Time To First Token)** | Latency from prompt dispatch until the first message or thought chunk arrives. |
| **Tool Wall Time** | Aggregated wall-clock time spent executing tools (with overlapping interval merging). |
| **Per-Tool Breakdown** | Total elapsed wall time and call counts per tool (e.g., `bash`, `read_file`). |
| **Throughput (TPS)** | Tokens Per Second generation rate (`tokens / (PromptDuration - TTFT)`). |
| **Summary Statistics** | Calculates Min, Mean, Max, and Standard Deviation across measured iterations. |
| **Output Formats** | Clean human-readable CLI tables or machine-readable JSON (`--json`). |

## Installation

### Via `go install`

```sh
go install github.com/baldaworks/acpbench/cmd/acpbench@latest
```

### Build from source

Using [Task](https://taskfile.dev):

```sh
task default
# Binary will be placed in bin/acpbench
```

## Quick Start

Benchmark an ACP server:

```sh
# Basic run with default prompt
acpbench -- opencode acp

# Set iterations, warmup runs, and model
acpbench -n 3 -w 1 --model claude-3-7-sonnet -- npx -y @zed-industries/claude-code-acp@latest

# Custom prompt and high reasoning effort
acpbench -p "Explain goroutines with code examples" -r high -- opencode acp

# Output JSON for CI / dashboard tracking
acpbench --json -n 5 -- npx -y @normahq/codex-acp-bridge@latest
```

The `--` separator is required. Arguments before `--` are treated as `acpbench` flags; arguments after `--` are executed as the ACP server command.

## CLI Flags

| Flag | Shorthand | Default | Description |
| --- | --- | --- | --- |
| `--prompt` | `-p` | `"Hello! Reply with 'ready' and nothing else."` | Benchmark prompt text sent to the agent. |
| `--model` | `-m` | `""` | Model identifier to select on the ACP session. |
| `--reasoning-effort` | `-r` | `""` | Reasoning/thinking effort level (e.g. `low`, `medium`, `high`). |
| `--iterations` | `-n` | `1` | Number of measured benchmark runs. |
| `--warmup` | `-w` | `0` | Number of unmeasured warmup runs. |
| `--timeout` | `-t` | `1m0s` | Maximum duration allowed per iteration. |
| `--json` | | `false` | Output results in JSON format. |
| `--debug` | | `false` | Enable debug logging to stderr. |

## Example Output

### Human Summary (Default)

```text
Target:           opencode acp
Model:            claude-3-7-sonnet
Reasoning Effort: high
Prompt:           "Hello! Reply with 'ready' and nothing else."
Warmup:           1 run(s)
Iterations:       2 run(s)

Benchmark Runs:
Run  Spawn (ms)  Init (ms)  Session (ms)  Config (ms)  TTFT (ms)  Tool Wall (ms)  Prompt (ms)  Total (ms)  Tokens     TPS  Chunks  Stop Reason
1           1.6        5.8           0.5          0.0        6.0            10.6         28.2        36.2       6   270.0       2  end_turn
2           1.7        0.9           0.3          0.0        5.8            13.2         29.3        32.2       6   255.6       2  end_turn

Summary Statistics (2 measured runs):
Metric                Min (ms)     Mean (ms)      Max (ms)    StdDev (ms)
Spawn                     1.58          1.65          1.72           0.07
Initialize                0.89          3.36          5.84           2.48
New Session               0.28          0.40          0.52           0.12
Config                    0.00          0.00          0.00           0.00
Time to First Token       5.80          5.89          5.98           0.09
Tool Wall Time           10.61         11.92         13.23           1.31
Prompt Duration          28.20         28.74         29.27           0.54
Total Duration           32.17         34.17         36.17           2.00
Tokens / Sec (TPS)      255.58        262.81        270.03           7.22

Tool Execution Breakdown:
Tool Name             Calls    Total Wall (ms)    Avg (ms)
mock_tool                 2              23.84       11.92
```

## Repository & Development

- Repository: <https://github.com/baldaworks/acpbench>
- Issues: <https://github.com/baldaworks/acpbench/issues>
- Operations: Run `task test`, `task lint`, `task security`, or `task default`.

## License

MIT. See [LICENSE](LICENSE).
