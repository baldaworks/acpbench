package command

import (
	"bytes"
	"strings"
	"testing"
)

func TestRequireACPCommandAfterDash(t *testing.T) {
	cmd := Command()

	// Missing dash
	cmd.SetArgs([]string{"my-command"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "missing command delimiter --") {
		t.Fatalf("expected missing delimiter error, got: %v", err)
	}

	// Args before dash
	cmd.SetArgs([]string{"foo", "--", "my-command"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "arguments before -- are not allowed") {
		t.Fatalf("expected args before dash error, got: %v", err)
	}

	// Empty after dash
	cmd.SetArgs([]string{"--"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "acp server command is required after --") {
		t.Fatalf("expected command required error, got: %v", err)
	}
}

func TestFlagsParsing(t *testing.T) {
	cmd := Command()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	args := []string{
		"--prompt", "custom prompt",
		"--model", "gpt-4o",
		"--reasoning-effort", "low",
		"-n", "2",
		"-w", "1",
		"-t", "30s",
		"--json",
		"--debug",
		"--", "echo", "test",
	}

	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	promptFlag, _ := cmd.Flags().GetString("prompt")
	if promptFlag != "custom prompt" {
		t.Errorf("expected prompt 'custom prompt', got %q", promptFlag)
	}

	modelFlag, _ := cmd.Flags().GetString("model")
	if modelFlag != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", modelFlag)
	}

	reasoningFlag, _ := cmd.Flags().GetString("reasoning-effort")
	if reasoningFlag != "low" {
		t.Errorf("expected reasoning effort 'low', got %q", reasoningFlag)
	}

	iterationsFlag, _ := cmd.Flags().GetInt("iterations")
	if iterationsFlag != 2 {
		t.Errorf("expected iterations 2, got %d", iterationsFlag)
	}

	warmupFlag, _ := cmd.Flags().GetInt("warmup")
	if warmupFlag != 1 {
		t.Errorf("expected warmup 1, got %d", warmupFlag)
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")
	if !jsonFlag {
		t.Errorf("expected json true, got %v", jsonFlag)
	}

	debugFlag, _ := cmd.Flags().GetBool("debug")
	if !debugFlag {
		t.Errorf("expected debug true, got %v", debugFlag)
	}
}
