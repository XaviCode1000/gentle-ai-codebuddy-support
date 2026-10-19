package agentguidance

import (
	"fmt"
	"os"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v4/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v4/internal/components/filemerge"
)

const (
	codebuddyBackgroundStart = "<!-- gentle-ai:codebuddy-background-subagents -->"
	codebuddyBackgroundEnd   = "<!-- /gentle-ai:codebuddy-background-subagents -->"
)

// ApplyCodeBuddyBackgroundPolicy updates only the managed orchestrator agent
// file. CodeBuddy decides background execution per delegation call, so the
// managed block is guidance for the orchestrator's own Agent tool calls; it
// never forces background on the review or Judgment Day agents, whose reports
// the orchestrator needs before it can continue.
func ApplyCodeBuddyBackgroundPolicy(agentPath string, enabled bool) (Result, error) {
	raw, err := os.ReadFile(agentPath)
	if os.IsNotExist(err) {
		return Result{}, nil
	}
	if err != nil {
		return Result{}, err
	}

	content := string(raw)
	start, end := strings.Index(content, codebuddyBackgroundStart), strings.Index(content, codebuddyBackgroundEnd)
	if (start < 0) != (end < 0) || strings.Count(content, codebuddyBackgroundStart) > 1 || strings.Count(content, codebuddyBackgroundEnd) > 1 || (start >= 0 && end <= start) {
		return Result{}, fmt.Errorf("invalid managed CodeBuddy background policy markers")
	}
	if start < 0 && !enabled {
		return Result{}, nil
	}

	if start >= 0 {
		head := strings.TrimRight(content[:start], "\n")
		tail := strings.TrimLeft(content[end+len(codebuddyBackgroundEnd):], "\n")
		content = head
		if tail != "" {
			content += "\n\n" + tail
		}
		content = strings.TrimRight(content, "\n") + "\n"
	}

	if enabled {
		policy := strings.TrimSpace(assets.MustRead("codebuddy/background-subagents.md"))
		if !strings.HasPrefix(policy, codebuddyBackgroundStart) || !strings.HasSuffix(policy, codebuddyBackgroundEnd) {
			return Result{}, fmt.Errorf("invalid embedded CodeBuddy background policy")
		}
		content = strings.TrimRight(content, "\n") + "\n\n" + policy + "\n"
	}

	if content == string(raw) {
		return Result{}, nil
	}
	written, err := filemerge.WriteFileAtomic(agentPath, []byte(content), filemerge.ExistingFileMode(agentPath, 0o644))
	if err != nil {
		return Result{}, err
	}
	return Result{Changed: written.Changed, Files: []string{agentPath}}, nil
}
