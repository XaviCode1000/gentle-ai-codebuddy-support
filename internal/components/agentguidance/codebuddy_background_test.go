package agentguidance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCodeBuddyAgent(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gentle-ai-orchestrator.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestApplyCodeBuddyBackgroundPolicy(t *testing.T) {
	const base = "---\nname: gentle-ai-orchestrator\n---\n\n# Orchestrator\n\nBody text.\n"
	path := writeCodeBuddyAgent(t, base)

	result, err := ApplyCodeBuddyBackgroundPolicy(path, true)
	if err != nil {
		t.Fatalf("enable error = %v", err)
	}
	if !result.Changed {
		t.Fatal("enable reported no change")
	}
	enabled, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(enabled), codebuddyBackgroundStart) || !strings.Contains(string(enabled), codebuddyBackgroundEnd) {
		t.Fatalf("enabled policy missing markers:\n%s", enabled)
	}
	if !strings.Contains(string(enabled), "run_in_background: true") {
		t.Fatalf("enabled policy missing delegation guidance:\n%s", enabled)
	}
	if !strings.HasPrefix(string(enabled), base) {
		t.Fatalf("enable rewrote the agent prefix:\n%s", enabled)
	}

	again, err := ApplyCodeBuddyBackgroundPolicy(path, true)
	if err != nil {
		t.Fatalf("second enable error = %v", err)
	}
	if again.Changed {
		t.Fatal("second enable reported a change")
	}
	if strings.Count(string(enabled), codebuddyBackgroundStart) != 1 {
		t.Fatalf("policy block duplicated:\n%s", enabled)
	}

	if _, err := ApplyCodeBuddyBackgroundPolicy(path, false); err != nil {
		t.Fatalf("disable error = %v", err)
	}
	disabled, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(disabled), codebuddyBackgroundStart) {
		t.Fatalf("disable left the policy block behind:\n%s", disabled)
	}
	if !strings.Contains(string(disabled), "Body text.") {
		t.Fatalf("disable removed user content:\n%s", disabled)
	}

	offAgain, err := ApplyCodeBuddyBackgroundPolicy(path, false)
	if err != nil {
		t.Fatalf("second disable error = %v", err)
	}
	if offAgain.Changed {
		t.Fatal("second disable reported a change")
	}
}

func TestApplyCodeBuddyBackgroundPolicyMissingFileIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.md")
	result, err := ApplyCodeBuddyBackgroundPolicy(path, true)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Changed || len(result.Files) != 0 {
		t.Fatalf("missing file reported a change: %#v", result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("missing file was created: %v", err)
	}
}

func TestApplyCodeBuddyBackgroundPolicyRejectsBrokenMarkers(t *testing.T) {
	for name, content := range map[string]string{
		"start only": "body\n" + codebuddyBackgroundStart + "\npolicy\n",
		"end only":   "body\npolicy\n" + codebuddyBackgroundEnd + "\n",
		"reversed":   codebuddyBackgroundEnd + "\npolicy\n" + codebuddyBackgroundStart + "\n",
		"duplicated": codebuddyBackgroundStart + "\na\n" + codebuddyBackgroundEnd + "\n" + codebuddyBackgroundStart + "\nb\n" + codebuddyBackgroundEnd + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeCodeBuddyAgent(t, content)
			if _, err := ApplyCodeBuddyBackgroundPolicy(path, true); err == nil {
				t.Fatal("expected an error for broken markers")
			}
		})
	}
}
