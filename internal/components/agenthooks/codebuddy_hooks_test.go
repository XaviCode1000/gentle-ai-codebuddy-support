package agenthooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
)

const fakeInstalledBinary = "/opt/tools/gentle-ai"

func codeBuddyAdapter(t *testing.T) agents.Adapter {
	t.Helper()
	adapter, err := agents.NewAdapter(model.AgentCodeBuddy)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func claudeAdapter(t *testing.T) agents.Adapter {
	t.Helper()
	adapter, err := agents.NewAdapter(model.AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

// hookCommands returns every hook command string in the settings document,
// keyed by event name.
func hookCommands(t *testing.T, path string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse settings %q: %v\n%s", path, err, data)
	}
	commands := map[string][]string{}
	for event, entries := range document.Hooks {
		for _, entry := range entries {
			for _, hook := range entry.Hooks {
				commands[event] = append(commands[event], hook.Command)
			}
		}
	}
	return commands
}

func countCommand(commands []string, want string) int {
	count := 0
	for _, command := range commands {
		if command == want {
			count++
		}
	}
	return count
}

// TestCodeBuddyHookCommandsBakeTheInstallingBinary pins that CodeBuddy hook
// commands invoke the absolute binary that performed the install on POSIX
// hosts, while Claude keeps plain PATH resolution and CodeBuddy on Windows
// keeps the plain name (both shells there need per-shell call syntax).
func TestCodeBuddyHookCommandsBakeTheInstallingBinary(t *testing.T) {
	const review = `"` + fakeInstalledBinary + `" review stop-hook --agent codebuddy`
	const telemetry = `"` + fakeInstalledBinary + `" telemetry runtime codebuddy --json`
	const skillRegistry = `sh -c '"` + fakeInstalledBinary + `" skill-registry refresh --quiet --no-gitignore --cwd "${CODEBUDDY_PROJECT_DIR:-$PWD}" || true'`

	t.Run("posix retained hooks", func(t *testing.T) {
		home := t.TempDir()
		result, err := installRetainedClaudeHooks(home, codeBuddyAdapter(t), fakeInstalledBinary)
		if err != nil || !result.Changed {
			t.Fatalf("install: %+v %v", result, err)
		}
		commands := hookCommands(t, codeBuddyAdapter(t).SettingsPath(home))
		if got := countCommand(commands["Stop"], review); got != 1 {
			t.Fatalf("Stop review hook count = %d, want 1: %v", got, commands["Stop"])
		}
		if got := countCommand(commands["SessionStart"], review); got != 1 {
			t.Fatalf("SessionStart review hook count = %d, want 1: %v", got, commands["SessionStart"])
		}
		if got := countCommand(commands["SubagentStop"], telemetry); got != 1 {
			t.Fatalf("SubagentStop telemetry hook count = %d, want 1: %v", got, commands["SubagentStop"])
		}
		if got := countCommand(commands["Stop"], telemetry); got != 1 {
			t.Fatalf("Stop telemetry hook count = %d, want 1: %v", got, commands["Stop"])
		}
	})

	t.Run("posix skill registry", func(t *testing.T) {
		home := t.TempDir()
		result, err := installSkillRegistry(home, codeBuddyAdapter(t), "linux", fakeInstalledBinary)
		if err != nil || !result.Changed {
			t.Fatalf("install: %+v %v", result, err)
		}
		commands := hookCommands(t, codeBuddyAdapter(t).SettingsPath(home))
		if got := countCommand(commands["UserPromptSubmit"], skillRegistry); got != 1 {
			t.Fatalf("UserPromptSubmit hook count = %d, want 1: %v", got, commands["UserPromptSubmit"])
		}
	})

	t.Run("claude keeps plain PATH resolution", func(t *testing.T) {
		home := t.TempDir()
		if _, err := installRetainedClaudeHooks(home, claudeAdapter(t), fakeInstalledBinary); err != nil {
			t.Fatal(err)
		}
		commands := hookCommands(t, claudeAdapter(t).SettingsPath(home))
		if got := countCommand(commands["Stop"], "gentle-ai review stop-hook --agent claude-code"); got != 1 {
			t.Fatalf("claude Stop review hook count = %d, want the plain-name form: %v", got, commands["Stop"])
		}
		if strings.Contains(strings.Join(commands["Stop"], "\n"), fakeInstalledBinary) {
			t.Fatalf("claude hooks must not bake the binary path: %v", commands["Stop"])
		}
	})

	t.Run("windows keeps plain PATH resolution", func(t *testing.T) {
		home := t.TempDir()
		if _, err := installRetainedClaudeHooks(home, codeBuddyAdapter(t), "gentle-ai"); err != nil {
			t.Fatal(err)
		}
		if _, err := installSkillRegistry(home, codeBuddyAdapter(t), "windows", fakeInstalledBinary); err != nil {
			t.Fatal(err)
		}
		commands := hookCommands(t, codeBuddyAdapter(t).SettingsPath(home))
		joined := strings.Join(append(commands["Stop"], commands["UserPromptSubmit"]...), "\n")
		if strings.Contains(joined, fakeInstalledBinary) {
			t.Fatalf("windows hooks must keep the plain name: %s", joined)
		}
		if !strings.Contains(joined, `gentle-ai review stop-hook --agent codebuddy`) {
			t.Fatalf("windows review hook missing: %s", joined)
		}
	})
}

// TestCodeBuddyRetainedHooksRetireBareCommands upgrades a settings file that a
// path-unaware version installed: the plain-name review and telemetry hooks
// are replaced by the baked-path forms, not accumulated beside them.
func TestCodeBuddyRetainedHooksRetireBareCommands(t *testing.T) {
	adapter := codeBuddyAdapter(t)
	home := t.TempDir()
	path := adapter.SettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"hooks":{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"gentle-ai review stop-hook --agent codebuddy","timeout":60},{"type":"command","command":"gentle-ai telemetry runtime codebuddy --json","async":true,"asyncTimeout":5}]}]}}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := installRetainedClaudeHooks(home, adapter, fakeInstalledBinary)
	if err != nil || !result.Changed {
		t.Fatalf("upgrade install: %+v %v", result, err)
	}
	commands := hookCommands(t, path)
	for _, bare := range []string{
		"gentle-ai review stop-hook --agent codebuddy",
		"gentle-ai telemetry runtime codebuddy --json",
	} {
		if got := countCommand(commands["Stop"], bare); got != 0 {
			t.Fatalf("bare command %q survived the upgrade: %v", bare, commands["Stop"])
		}
	}
	if got := countCommand(commands["Stop"], `"`+fakeInstalledBinary+"\" review stop-hook --agent codebuddy"); got != 1 {
		t.Fatalf("baked review hook count = %d, want 1: %v", got, commands["Stop"])
	}
	second, err := installRetainedClaudeHooks(home, adapter, fakeInstalledBinary)
	if err != nil || second.Changed {
		t.Fatalf("second install not idempotent: %+v %v", second, err)
	}
}

// TestCodeBuddySkillRegistryRetiresBareShWrappedCommand upgrades the
// UserPromptSubmit hook from the plain-name sh -c form to the baked-path form,
// and keeps byte stability when the resolved token is the plain name.
func TestCodeBuddySkillRegistryRetiresBareShWrappedCommand(t *testing.T) {
	adapter := codeBuddyAdapter(t)
	home := t.TempDir()
	path := adapter.SettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	bare := `sh -c 'gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "${CODEBUDDY_PROJECT_DIR:-$PWD}" || true'`
	seed := `{"hooks":{"UserPromptSubmit":[{"matcher":"","hooks":[{"type":"command","command":` + mustJSON(t, bare) + `}]}]}}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := installSkillRegistry(home, adapter, "linux", fakeInstalledBinary)
	if err != nil || !result.Changed {
		t.Fatalf("upgrade install: %+v %v", result, err)
	}
	commands := hookCommands(t, path)
	if countCommand(commands["UserPromptSubmit"], bare) != 0 {
		t.Fatalf("bare sh -c command survived: %v", commands["UserPromptSubmit"])
	}
	want := `sh -c '"` + fakeInstalledBinary + `" skill-registry refresh --quiet --no-gitignore --cwd "${CODEBUDDY_PROJECT_DIR:-$PWD}" || true'`
	if countCommand(commands["UserPromptSubmit"], want) != 1 {
		t.Fatalf("baked command missing: %v", commands["UserPromptSubmit"])
	}
	second, err := installSkillRegistry(home, adapter, "linux", fakeInstalledBinary)
	if err != nil || second.Changed {
		t.Fatalf("second install not idempotent: %+v %v", second, err)
	}

	// With the plain-name token the legacy form stays canonical, the baked
	// form is retired rather than accumulated, and the file is byte-stable
	// across installs.
	if _, err := installSkillRegistry(home, adapter, "linux", "gentle-ai"); err != nil {
		t.Fatal(err)
	}
	commands = hookCommands(t, path)
	if countCommand(commands["UserPromptSubmit"], want) != 0 {
		t.Fatalf("baked command survived the plain-token install: %v", commands["UserPromptSubmit"])
	}
	if countCommand(commands["UserPromptSubmit"], bare) != 1 {
		t.Fatalf("plain-token install should restore the bare canonical form: %v", commands["UserPromptSubmit"])
	}
	third, err := installSkillRegistry(home, adapter, "linux", "gentle-ai")
	if err != nil || third.Changed {
		t.Fatalf("plain-token install not idempotent: %+v %v", third, err)
	}
}
