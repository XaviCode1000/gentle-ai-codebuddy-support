package cli

import (
	"fmt"
	"os"
	"strings"
)

// CodeBuddyBackgroundSubagentsEnv carries the managed CodeBuddy background
// policy intent when the flag is absent.
const CodeBuddyBackgroundSubagentsEnv = "GENTLE_AI_CODEBUDDY_BACKGROUND_SUBAGENTS"

// resolveCodeBuddyBackgroundCLI resolves the managed background policy intent
// for CodeBuddy. CLI outranks the environment. A nil result means no decision
// was expressed, so the managed block is left untouched instead of being
// removed by a sync that never asked about it.
func resolveCodeBuddyBackgroundCLI(set bool, raw string) (*bool, error) {
	value, source := strings.TrimSpace(raw), "flag"
	if !set {
		value, source = strings.TrimSpace(os.Getenv(CodeBuddyBackgroundSubagentsEnv)), "environment"
	}
	switch strings.ToLower(value) {
	case "":
		return nil, nil
	case "on":
		enabled := true
		return &enabled, nil
	case "off":
		enabled := false
		return &enabled, nil
	default:
		// refusal:by-design human-authority: invalid flag/env value is a usage error whose only honest continuation is the operator correcting their input; no gentle-ai command can auto-resolve a mistyped intent.
		return nil, fmt.Errorf("invalid %s value %q: use on or off", source, value)
	}
}
