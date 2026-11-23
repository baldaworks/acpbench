package acpbench_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/baldaworks/acpbench/internal/apps/acpbench"
)

// Helper subprocess: one marker per provider session, removed only by session/delete.
func TestCleanupAgent(t *testing.T) {
	mode := os.Getenv("ACPBENCH_TEST_CLEANUP_AGENT")
	if mode == "" {
		return
	}
	id := fmt.Sprintf("session-%d", os.Getpid())
	input := bufio.NewScanner(os.Stdin)
	output := json.NewEncoder(os.Stdout)
	for input.Scan() {
		var req struct {
			ID     json.RawMessage
			Method string
			Params struct {
				SessionID string
				Meta      json.RawMessage `json:"_meta"`
			}
		}
		if err := json.Unmarshal(input.Bytes(), &req); err != nil {
			t.Fatal(err)
		}
		if len(req.ID) == 0 {
			continue
		}
		reply := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		switch req.Method {
		case "initialize":
			caps := map[string]any{}
			switch mode {
			case "close-only":
				caps["close"] = struct{}{}
			case "legacy":
			default:
				caps["delete"] = struct{}{}
			}
			reply["result"] = map[string]any{"protocolVersion": 1, "agentInfo": map[string]string{"name": "fixture", "version": "test"}, "agentCapabilities": map[string]any{"sessionCapabilities": caps}}
		case "session/new":
			if req.Params.Meta != nil {
				t.Fatal("unexpected session metadata")
			}
			if err := os.WriteFile(id, nil, 0600); err != nil {
				t.Fatal(err)
			}
			reply["result"] = map[string]string{"sessionId": id}
		case "session/prompt":
			if mode == "timeout" {
				continue
			}
			if mode == "failure" {
				reply["error"] = map[string]any{"code": -32603, "message": "prompt failed"}
			} else {
				reply["result"] = map[string]string{"stopReason": "end_turn"}
			}
		case "session/delete", "session/close":
			if req.Params.SessionID != id {
				t.Fatalf("unexpected ID %q", req.Params.SessionID)
			}
			if req.Method == "session/delete" {
				if mode == "delete-error" {
					reply["error"] = map[string]any{"code": -32603, "message": "delete failed"}
					break
				}
				if err := os.Remove(id); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile("released-"+id, []byte(req.Method), 0600); err != nil {
				t.Fatal(err)
			}
			reply["result"] = struct{}{}
		default:
			t.Fatalf("unexpected RPC %q", req.Method)
		}
		if err := output.Encode(reply); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBenchmarkSessionCleanup(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		mode, method    string
		remains, failed bool
	}{
		{"delete", "session/delete", false, false}, {"failure", "session/delete", false, true},
		{"timeout", "session/delete", false, true}, {"delete-error", "", true, true},
		{"close-only", "session/close", true, false}, {"legacy", "", true, false},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("ACPBENCH_TEST_CLEANUP_AGENT", tc.mode)
			dir := t.TempDir()
			var warnings bytes.Buffer
			report, err := acpbench.RunBenchmark(context.Background(), acpbench.BenchmarkConfig{
				Command: []string{executable, "-test.run=^TestCleanupAgent$"}, WorkingDir: dir,
				Stderr: &warnings, WarmupRuns: 1, Iterations: 1, Timeout: time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := report.Runs[0].Error; (got != "") != tc.failed {
				t.Fatalf("unexpected run error %q", got)
			}
			if strings.Contains(warnings.String(), "may remain") != (tc.mode == "legacy" || tc.mode == "close-only") {
				t.Fatalf("unexpected warnings: %s", warnings.String())
			}
			sessions, _ := filepath.Glob(filepath.Join(dir, "session-*"))
			if (tc.remains && len(sessions) != 2) || (!tc.remains && len(sessions) != 0) {
				t.Fatalf("remaining sessions: %v", sessions)
			}
			released, _ := filepath.Glob(filepath.Join(dir, "released-*"))
			if tc.method != "" && len(released) != 2 {
				t.Fatalf("released sessions: %v", released)
			}
			for _, path := range released {
				method, err := os.ReadFile(path)
				if err != nil || string(method) != tc.method {
					t.Fatalf("release %s: %q, %v", path, method, err)
				}
			}
		})
	}
}
