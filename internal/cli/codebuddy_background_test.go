package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
)

func boolPtr(v bool) *bool { return &v }

func TestAgentRoutingGuidanceStepCodeBuddyBackgroundPolicy(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	agentPath := filepath.Join(workspace, ".codebuddy", "agents", "gentle-ai-orchestrator.md")

	run := func(enabled *bool) {
		t.Helper()
		step := agentRoutingGuidanceStep{
			id:                  "agent-guidance:codebuddy",
			agent:               model.AgentCodeBuddy,
			homeDir:             home,
			workspaceDir:        workspace,
			scope:               ScopeWorkspace,
			codebuddyBackground: enabled,
		}
		if err := step.Run(); err != nil {
			t.Fatalf("step.Run() error = %v", err)
		}
	}

	run(boolPtr(true))
	raw, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("orchestrator agent missing: %v", err)
	}
	if !strings.Contains(string(raw), "gentle-ai:codebuddy-background-subagents") {
		t.Fatalf("background policy missing from the orchestrator agent:\n%s", raw)
	}
	if !strings.Contains(string(raw), "run_in_background: true") {
		t.Fatalf("background policy has no delegation guidance:\n%s", raw)
	}

	run(boolPtr(false))
	raw, err = os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("orchestrator agent missing after disable: %v", err)
	}
	if strings.Contains(string(raw), "gentle-ai:codebuddy-background-subagents") {
		t.Fatalf("disable left the background policy behind:\n%s", raw)
	}

	// An absent intent (nil) defaults to enabled: a plain install or sync turns
	// background delegation on unless the user explicitly opted out.
	run(nil)
	raw, err = os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("orchestrator agent missing after default run: %v", err)
	}
	if !strings.Contains(string(raw), "gentle-ai:codebuddy-background-subagents") {
		t.Fatalf("nil intent did not default to an enabled policy:\n%s", raw)
	}

	// Re-running with the default stays idempotent: the block is present once.
	run(nil)
	raw, err = os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("orchestrator agent missing after second default run: %v", err)
	}
	if strings.Count(string(raw), "gentle-ai:codebuddy-background-subagents") != 2 {
		t.Fatalf("default run duplicated the managed block:\n%s", raw)
	}
}

func TestParseInstallFlagsCodeBuddyBackground(t *testing.T) {
	flags, err := ParseInstallFlags([]string{"--codebuddy-background-subagents=on"})
	if err != nil {
		t.Fatalf("ParseInstallFlags() error = %v", err)
	}
	if !flags.CodeBuddyBackgroundSubagentsSet || flags.CodeBuddyBackgroundSubagents != "on" {
		t.Fatalf("parsed flags = %#v", flags)
	}

	var help strings.Builder
	PrintInstallHelp(&help)
	if !strings.Contains(help.String(), "--codebuddy-background-subagents=on|off") || !strings.Contains(help.String(), CodeBuddyBackgroundSubagentsEnv) {
		t.Fatalf("install help does not document the flag:\n%s", help.String())
	}

	if _, err := ParseInstallFlags([]string{"--codebuddy-background-subagents=maybe"}); err != nil {
		t.Fatalf("parse accepts any string; resolution must reject it, got %v", err)
	}
	if _, err := resolveCodeBuddyBackgroundCLI(true, "maybe"); err == nil {
		t.Fatal("expected an error for an invalid intent")
	}
}

func TestResolveCodeBuddyBackgroundPrecedence(t *testing.T) {
	t.Setenv(CodeBuddyBackgroundSubagentsEnv, "on")
	got, err := resolveCodeBuddyBackgroundCLI(false, "")
	if err != nil || got == nil || !*got {
		t.Fatalf("environment resolution = %v, %v", got, err)
	}

	got, err = resolveCodeBuddyBackgroundCLI(true, "off")
	if err != nil || got == nil || *got {
		t.Fatalf("CLI must outrank the environment: %v, %v", got, err)
	}

	t.Setenv(CodeBuddyBackgroundSubagentsEnv, "")
	got, err = resolveCodeBuddyBackgroundCLI(false, "")
	if err != nil || got != nil {
		t.Fatalf("unset intent must stay nil: %v, %v", got, err)
	}
}

func TestRunInstallCodeBuddyBackgroundFlag(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()

	originalCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current wd: %v", err)
	}
	restoreHome := osUserHomeDir
	restoreCommand := runCommand
	restoreLookPath := cmdLookPath
	t.Cleanup(func() {
		osUserHomeDir = restoreHome
		runCommand = restoreCommand
		cmdLookPath = restoreLookPath
		if err := os.Chdir(originalCwd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	})
	osUserHomeDir = func() (string, error) { return home, nil }
	runCommand = func(string, ...string) error { return nil }
	cmdLookPath = func(name string) (string, error) { return "/usr/local/bin/" + name, nil }
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}

	agentPath := filepath.Join(workspace, ".codebuddy", "agents", "gentle-ai-orchestrator.md")
	install := func(extra ...string) {
		t.Helper()
		args := append([]string{
			"--scope", "workspace",
			"--agent", "codebuddy",
			"--component", "permissions",
			"--preset", "custom",
		}, extra...)
		result, err := RunInstall(args, system.DetectionResult{})
		if err != nil {
			t.Fatalf("RunInstall(%v) error = %v", extra, err)
		}
		if !result.Verify.Ready {
			t.Fatalf("verification failed for %v: %#v", extra, result.Verify)
		}
	}

	install("--codebuddy-background-subagents=on")
	raw, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("orchestrator agent missing: %v", err)
	}
	if !strings.Contains(string(raw), "gentle-ai:codebuddy-background-subagents") {
		t.Fatalf("install did not apply the background policy:\n%s", raw)
	}

	install("--codebuddy-background-subagents=off")
	raw, err = os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("orchestrator agent missing: %v", err)
	}
	if strings.Contains(string(raw), "gentle-ai:codebuddy-background-subagents") {
		t.Fatalf("install --codebuddy-background-subagents=off left the policy behind:\n%s", raw)
	}

	if _, err := RunInstall([]string{
		"--scope", "workspace",
		"--agent", "codebuddy",
		"--component", "permissions",
		"--preset", "custom",
		"--codebuddy-background-subagents=maybe",
	}, system.DetectionResult{}); err == nil {
		t.Fatal("expected an error for an invalid intent")
	}
}
