package agenthooks

import (
	"os"
	"path/filepath"
	"strings"
)

// managedHookInvocationBodies are the gentle-ai argument strings installed hook
// commands are built from, independent of the binary token in front of them.
// A hook command is recognized by its invocation, so upgrades can retire hooks
// written by another binary location and uninstall can remove hooks without
// knowing which binary wrote them.
var managedHookInvocationBodies = []string{
	`skill-registry refresh --quiet --no-gitignore --cwd "$PWD" || true`,
	`skill-registry refresh --quiet --no-gitignore --cwd "${CLAUDE_PROJECT_DIR:-$PWD}" || true`,
	`skill-registry refresh --quiet --no-gitignore --cwd "${CODEBUDDY_PROJECT_DIR:-$PWD}" || true`,
	"review stop-hook --agent claude-code",
	"review stop-hook --agent codebuddy",
	"telemetry runtime claude --json",
	"telemetry runtime codebuddy --json",
	"telemetry runtime codex --json",
}

// IsManagedHookCommand reports whether a settings hook command was installed by
// gentle-ai, whether the binary was recorded as the plain name or as the
// absolute path of the installing executable. Commands that merely wrap a
// managed invocation behind a different binary token are still managed;
// anything else stays unmanaged user text.
func IsManagedHookCommand(command string) bool {
	body := hookInvocationBody(command)
	for _, invocation := range managedHookInvocationBodies {
		if body == invocation {
			return true
		}
	}
	return false
}

// IsManagedHookFamily reports whether a settings hook command is a managed
// invocation of the given subcommand family ("skill-registry", "review" or
// "telemetry"), regardless of the binary token.
func IsManagedHookFamily(command, family string) bool {
	body := hookInvocationBody(command)
	if !IsManagedHookCommand(command) {
		return false
	}
	head, _, _ := strings.Cut(body, " ")
	return head == family
}

// hookInvocationBody strips the wrappers installed commands are built with:
// the POSIX `sh -c '...'` wrapper, a leading double-quoted absolute path and
// the plain binary name.
func hookInvocationBody(command string) string {
	if inner, ok := strings.CutPrefix(command, `sh -c '`); ok && strings.HasSuffix(inner, "'") {
		command = strings.TrimSuffix(inner, "'")
	}
	if rest, ok := strings.CutPrefix(command, `"`); ok {
		if end := strings.Index(rest, `"`); end >= 0 {
			command = strings.TrimSpace(rest[end+1:])
		}
	}
	return strings.TrimPrefix(command, "gentle-ai ")
}

// HookBinaryFor resolves the binary token hook commands are written with on
// the given platform. POSIX hosts bake the absolute path of the installing
// executable, so a later PATH lookup cannot silently reach a global install
// that predates the agent support the hooks depend on. Windows keeps the plain
// name: hook commands there run under shells whose invocation of a quoted path
// differs (PowerShell needs the call operator, cmd does not), so a baked path
// is not portable across them. Executables under the temp directory come from
// `go run`/`go test` and disappear, so they fall back to the plain name too.
func HookBinaryFor(goos string) string {
	if goos == "windows" {
		return "gentle-ai"
	}
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return "gentle-ai"
	}
	if prefix := filepath.Clean(os.TempDir()); prefix != "" && strings.HasPrefix(exe, prefix+string(os.PathSeparator)) {
		return "gentle-ai"
	}
	return exe
}

// hookCommandToken renders the binary token for a hook command string: the
// plain name stays bare so exact legacy matching keeps working, and an
// absolute path is double-quoted so spaces survive the host shell.
func hookCommandToken(binary string) string {
	if binary == "" || binary == "gentle-ai" {
		return "gentle-ai"
	}
	return `"` + binary + `"`
}

// pruneHookCommands removes hook commands matched by keep from the entries of
// one event, preserving unrelated hooks and their outer entries. It reports
// the kept entries and whether anything was removed.
func pruneHookCommands(entries []any, remove func(command string) bool) ([]any, bool) {
	keptEntries := make([]any, 0, len(entries))
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
		keptHooks := make([]any, 0, len(hooks))
		for _, hook := range hooks {
			h, ok := hook.(map[string]any)
			command, _ := h["command"].(string)
			if ok && remove(command) {
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
