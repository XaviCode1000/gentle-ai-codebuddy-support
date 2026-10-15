// Package agenthooks owns runtime hook installation independent of optional components.
package agenthooks

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gentleman-programming/gentle-ai/v4/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v4/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
)

type Result struct {
	Changed bool
	Files   []string
}

// posixSkillRegistryCommand wraps a POSIX body in `sh -c` so it runs under a
// guaranteed POSIX shell. Claude Code and CodeBuddy execute hook commands with
// the user's login shell, so a fish/zsh/csh user would otherwise hit a
// "parameter expansion not supported" error on `"${VAR:-$PWD}"`. The single
// quotes keep the body opaque to whatever shell the host uses to launch `sh`.
func posixSkillRegistryCommand(projectDirVar string) string {
	return `sh -c 'gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "${` + projectDirVar + `:-$PWD}" || true'`
}

func claudeSkillRegistryCommand(platform string) string {
	if platform == "windows" {
		return `powershell -NoProfile -Command 'if (Test-Path env:CLAUDE_PROJECT_DIR) { $dir = $env:CLAUDE_PROJECT_DIR } else { $dir = $PWD }; gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "$dir"; exit 0'`
	}
	return posixSkillRegistryCommand("CLAUDE_PROJECT_DIR")
}

// codebuddySkillRegistryCommand mirrors the Claude hook, resolving the project
// directory from CodeBuddy's own CODEBUDDY_PROJECT_DIR hook variable.
func codebuddySkillRegistryCommand(platform string) string {
	if platform == "windows" {
		return `powershell -NoProfile -Command 'if (Test-Path env:CODEBUDDY_PROJECT_DIR) { $dir = $env:CODEBUDDY_PROJECT_DIR } else { $dir = $PWD }; gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "$dir"; exit 0'`
	}
	return posixSkillRegistryCommand("CODEBUDDY_PROJECT_DIR")
}

// claudeRetiredSkillRegistryCommands are the exact retired POSIX forms that a
// prior version installed for Claude. The bare `${VAR:-$PWD}` expansion fails
// under a non-POSIX login shell (fish/zsh/csh), so the current install wraps
// the body in `sh -c` (posixSkillRegistryCommand). These literals are kept
// verbatim so an upgrade recognizes and retires the exact old hook.
var claudeRetiredSkillRegistryCommands = []string{
	`gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "${CLAUDE_PROJECT_DIR:-$PWD}" || true`,
}

// codebuddyRetiredSkillRegistryCommands are the exact retired POSIX forms a
// prior version installed for CodeBuddy.
var codebuddyRetiredSkillRegistryCommands = []string{
	`gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "${CODEBUDDY_PROJECT_DIR:-$PWD}" || true`,
}

// pruneSkillRegistryHooks removes the given retired commands from
// UserPromptSubmit entries, preserving unrelated hooks and their outer entries.
func pruneSkillRegistryHooks(entries []any, retired []string) ([]any, bool) {
	matchesRetired := func(cmd string) bool {
		for _, r := range retired {
			if cmd == r {
				return true
			}
		}
		return false
	}
	var keptEntries []any
	changed := false
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			keptEntries = append(keptEntries, entry)
			continue
		}
		hooks, ok := item["hooks"].([]any)
		if !ok {
			keptEntries = append(keptEntries, entry)
			continue
		}
		var keptHooks []any
		for _, hook := range hooks {
			h, ok := hook.(map[string]any)
			if ok && matchesRetired(fmt.Sprint(h["command"])) {
				changed = true
				continue
			}
			keptHooks = append(keptHooks, hook)
		}
		if len(keptHooks) == len(hooks) {
			keptEntries = append(keptEntries, entry)
			continue
		}
		if len(keptHooks) == 0 {
			continue
		}
		copyItem := make(map[string]any, len(item))
		for k, v := range item {
			copyItem[k] = v
		}
		copyItem["hooks"] = keptHooks
		keptEntries = append(keptEntries, copyItem)
	}
	return keptEntries, changed
}

// InstallSkillRegistry installs startup refresh hooks without selecting SDD.
func InstallSkillRegistry(homeDir string, adapter agents.Adapter) (Result, error) {
	return installSkillRegistry(homeDir, adapter, runtime.GOOS)
}

func installSkillRegistry(homeDir string, adapter agents.Adapter, platform string) (Result, error) {
	var path, event, command string
	switch adapter.Agent() {
	case model.AgentCodex:
		path = filepath.Join(adapter.GlobalConfigDir(homeDir), "hooks.json")
		event = "SessionStart"
		command = `gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "$PWD" || true`
	case model.AgentClaudeCode:
		path = adapter.SettingsPath(homeDir)
		event = "UserPromptSubmit"
		command = claudeSkillRegistryCommand(platform)
	case model.AgentCodeBuddy:
		path = adapter.SettingsPath(homeDir)
		event = "UserPromptSubmit"
		command = codebuddySkillRegistryCommand(platform)
	default:
		return Result{}, nil
	}
	if path == "" {
		return Result{}, nil
	}
	var data []byte
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	if err == nil {
		if !info.Mode().IsRegular() {
			return Result{}, fmt.Errorf("hook settings %q is not a regular file", path)
		}
		if data, err = os.ReadFile(path); err != nil {
			return Result{}, err
		}
	}
	root, err := decodeHookSettings(data)
	if err != nil {
		return Result{}, fmt.Errorf("parse hook settings %q: %w", path, err)
	}
	hooks, ok := root["hooks"].(map[string]any)
	if _, exists := root["hooks"]; exists && !ok {
		return Result{}, fmt.Errorf("hook settings %q has unsupported hooks shape: want object", path)
	}
	if !ok {
		hooks = map[string]any{}
	}
	entries, ok := hooks[event].([]any)
	if _, exists := hooks[event]; exists && !ok {
		return Result{}, fmt.Errorf("hook settings %q has unsupported hooks.%s shape: want array", path, event)
	}
	// Retire any prior bare-POSIX command so an upgrade never leaves the old
	// (non-POSIX-shell-breaking) hook beside the new `sh -c` one. Applies to
	// both hosts on every platform: the retired literal is POSIX-only, so it is
	// stale wherever it appears.
	var retired []string
	switch adapter.Agent() {
	case model.AgentClaudeCode:
		retired = claudeRetiredSkillRegistryCommands
	case model.AgentCodeBuddy:
		retired = codebuddyRetiredSkillRegistryCommands
	}
	pruned := false
	if len(retired) > 0 {
		entries, pruned = pruneSkillRegistryHooks(entries, retired)
		if pruned {
			if len(entries) == 0 {
				delete(hooks, event)
			} else {
				hooks[event] = entries
			}
		}
	}
	if claudeHookListContains(entries, command) {
		if !pruned {
			return Result{Files: []string{path}}, nil
		}
	} else {
		hook := map[string]any{"type": "command", "command": command}
		entry := map[string]any{"matcher": "", "hooks": []any{hook}}
		if adapter.Agent() == model.AgentCodex {
			entry["matcher"] = "startup|resume|clear|compact"
			hook["timeout"] = 30
			hook["statusMessage"] = "Refreshing skill registry"
		}
		hooks[event] = append(entries, entry)
	}
	out, err := encodeHookSettings(data, root, hooks)
	if err != nil {
		return Result{}, fmt.Errorf("rewrite hook settings %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, err
	}
	wr, err := filemerge.WriteFileAtomic(path, out, 0o644)
	if err != nil {
		return Result{}, err
	}
	return Result{Changed: wr.Changed, Files: []string{path}}, nil
}
