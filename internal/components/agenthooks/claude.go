package agenthooks

import (
	"fmt"
	"os"
	"runtime"

	"github.com/gentleman-programming/gentle-ai/v4/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v4/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
)

// InstallRetainedClaudeHooks installs review and telemetry hooks independently
// of SDD. It serves Claude Code and CodeBuddy Code: both read the same hooks
// shape from settings.json, with one schema difference — CodeBuddy's async
// hooks take an `asyncTimeout` key where Claude takes `timeout`.
//
// CodeBuddy commands bake the binary token resolved by HookBinaryFor, so the
// hooks reach the gentle-ai build that carried the installed agent support
// even when a PATH lookup would resolve to an older global install. Claude
// keeps plain PATH resolution.
func InstallRetainedClaudeHooks(homeDir string, adapter agents.Adapter) (Result, error) {
	return installRetainedClaudeHooks(homeDir, adapter, HookBinaryFor(runtime.GOOS))
}

func installRetainedClaudeHooks(homeDir string, adapter agents.Adapter, binary string) (Result, error) {
	agent := adapter.Agent()
	if agent != model.AgentClaudeCode && agent != model.AgentCodeBuddy {
		return Result{}, nil
	}
	asyncTimeoutKey := "timeout"
	if agent == model.AgentCodeBuddy {
		asyncTimeoutKey = "asyncTimeout"
	}
	path := adapter.SettingsPath(homeDir)
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
		return Result{}, fmt.Errorf("parse Claude settings %q: %w", path, err)
	}
	raw, exists := root["hooks"]
	hooks, ok := raw.(map[string]any)
	if exists && !ok {
		return Result{}, fmt.Errorf("Claude settings %q has unsupported hooks shape: want object", path)
	}
	if !ok {
		hooks = map[string]any{}
	}
	telemetryRuntime := "claude"
	binaryToken := "gentle-ai"
	if agent == model.AgentCodeBuddy {
		telemetryRuntime = "codebuddy"
		binaryToken = hookCommandToken(binary)
	}
	command := binaryToken + " review stop-hook --agent " + string(agent)
	telemetryCommand := binaryToken + " telemetry runtime " + telemetryRuntime + " --json"
	entries := []struct {
		key, matcher, command string
		timeout               int
		async                 bool
	}{
		{"Stop", "", command, 60, false},
		{"SessionStart", "startup|resume|clear|compact", command, 30, false},
		{"SubagentStop", "", telemetryCommand, 5, true},
		{"Stop", "", telemetryCommand, 5, true},
	}
	changed := false
	if binaryToken != "gentle-ai" {
		// Retire managed review and telemetry hooks written with another
		// binary token so switching install locations never accumulates
		// stale gentle-ai hooks beside the canonical ones.
		canonical := map[string]bool{command: true, telemetryCommand: true}
		for _, key := range []string{"Stop", "SessionStart", "SubagentStop"} {
			list, _ := hooks[key].([]any)
			kept, pruned := pruneHookCommands(list, func(cmd string) bool {
				return cmd != "" && !canonical[cmd] &&
					(IsManagedHookFamily(cmd, "review") || IsManagedHookFamily(cmd, "telemetry"))
			})
			if pruned {
				if len(kept) == 0 {
					delete(hooks, key)
				} else {
					hooks[key] = kept
				}
				changed = true
			}
		}
	}
	for _, e := range entries {
		raw, exists := hooks[e.key]
		list, ok := raw.([]any)
		if exists && !ok {
			return Result{}, fmt.Errorf("Claude settings %q has unsupported hooks.%s shape: want array", path, e.key)
		}
		if claudeHookListContains(list, e.command) {
			continue
		}
		hook := map[string]any{"type": "command", "command": e.command, "timeout": e.timeout}
		if e.async {
			hook["async"] = true
			// CodeBuddy's async protocol names the bound `asyncTimeout`; the
			// generic `timeout` above is retained for Claude's schema.
			hook[asyncTimeoutKey] = e.timeout
		}
		hooks[e.key] = append(list, map[string]any{"matcher": e.matcher, "hooks": []any{hook}})
		changed = true
	}
	if !changed {
		return Result{Files: []string{path}}, nil
	}
	out, err := encodeHookSettings(data, root, hooks)
	if err != nil {
		return Result{}, fmt.Errorf("rewrite Claude settings %q: %w", path, err)
	}
	wr, err := filemerge.WriteFileAtomic(path, out, 0644)
	if err != nil {
		return Result{}, err
	}
	return Result{Changed: wr.Changed, Files: []string{path}}, nil
}

func claudeHookListContains(entries []any, command string) bool {
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := item["hooks"].([]any)
		if !ok {
			continue
		}
		for _, hook := range hooks {
			h, ok := hook.(map[string]any)
			if ok && h["command"] == command {
				return true
			}
		}
	}
	return false
}
