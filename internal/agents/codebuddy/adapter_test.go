package codebuddy

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/agents/capabilitymanifest"
	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
)

// TestAdapter_Agent pins the identity returned by Agent().
func TestAdapter_Agent(t *testing.T) {
	a := NewAdapter()
	if got := a.Agent(); got != model.AgentCodeBuddy {
		t.Errorf("Agent() = %v, want %v", got, model.AgentCodeBuddy)
	}
}

// TestAdapter_Tier pins the support tier returned by Tier().
func TestAdapter_Tier(t *testing.T) {
	a := NewAdapter()
	if got := a.Tier(); got != model.TierFull {
		t.Errorf("Tier() = %v, want %v", got, model.TierFull)
	}
}

// TestAdapter_ConfigPaths pins the CodeBuddy config paths under ~/.codebuddy.
func TestAdapter_ConfigPaths(t *testing.T) {
	a := NewAdapter()
	homeDir := "/home/test"

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"GlobalConfigDir", a.GlobalConfigDir(homeDir), filepath.Join(homeDir, ".codebuddy")},
		{"SystemPromptDir", a.SystemPromptDir(homeDir), filepath.Join(homeDir, ".codebuddy")},
		{"SystemPromptFile", a.SystemPromptFile(homeDir), filepath.Join(homeDir, ".codebuddy", "CODEBUDDY.md")},
		{"SettingsPath", a.SettingsPath(homeDir), filepath.Join(homeDir, ".codebuddy", "settings.json")},
		{"MCPConfigPath", a.MCPConfigPath(homeDir, ""), filepath.Join(homeDir, ".codebuddy", ".mcp.json")},
		{"SubAgentsDir", a.SubAgentsDir(homeDir), filepath.Join(homeDir, ".codebuddy", "agents")},
		{"CommandsDir", a.CommandsDir(homeDir), filepath.Join(homeDir, ".codebuddy", "commands")},
		{"SkillsDir", a.SkillsDir(homeDir), filepath.Join(homeDir, ".codebuddy", "skills")},
		{"OutputStyleDir", a.OutputStyleDir(homeDir), filepath.Join(homeDir, ".codebuddy", "output-styles")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestAdapter_ConfigPathsSuffixes pins the loosely-specified suffixes the
// audit requires (file basenames) regardless of the host separator.
func TestAdapter_ConfigPathsSuffixes(t *testing.T) {
	a := NewAdapter()
	homeDir := "/home/test"

	if got := a.SystemPromptFile(homeDir); !strings.HasSuffix(got, "CODEBUDDY.md") {
		t.Errorf("SystemPromptFile() = %q, want suffix CODEBUDDY.md", got)
	}
	if got := a.SettingsPath(homeDir); !strings.HasSuffix(got, "settings.json") {
		t.Errorf("SettingsPath() = %q, want suffix settings.json", got)
	}
	if got := a.MCPConfigPath(homeDir, ""); !strings.HasSuffix(got, ".mcp.json") {
		t.Errorf("MCPConfigPath() = %q, want suffix .mcp.json", got)
	}
}

// TestAdapter_Capabilities pins that CodeBuddy, like Claude Code, supports the
// full set of injection mechanisms gentle-ai manages.
func TestAdapter_Capabilities(t *testing.T) {
	a := NewAdapter()

	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"SupportsOutputStyles", a.SupportsOutputStyles(), true},
		{"SupportsSlashCommands", a.SupportsSlashCommands(), true},
		{"SupportsSubAgents", a.SupportsSubAgents(), true},
		{"SupportsSkills", a.SupportsSkills(), true},
		{"SupportsSystemPrompt", a.SupportsSystemPrompt(), true},
		{"SupportsMCP", a.SupportsMCP(), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
			}
		})
	}
}

// TestAdapter_CapabilityManifestMatchesPredicates pins that the canonical
// capability manifest's feature claims agree with the adapter's predicates.
func TestAdapter_CapabilityManifestMatchesPredicates(t *testing.T) {
	a := NewAdapter()
	manifest := a.CapabilityManifest()

	if manifest.AgentID != model.AgentCodeBuddy {
		t.Errorf("CapabilityManifest().AgentID = %v, want %v", manifest.AgentID, model.AgentCodeBuddy)
	}

	features := manifest.Features
	if features.OutputStyles != a.SupportsOutputStyles() {
		t.Errorf("Features.OutputStyles = %v, want %v", features.OutputStyles, a.SupportsOutputStyles())
	}
	if features.SlashCommands != a.SupportsSlashCommands() {
		t.Errorf("Features.SlashCommands = %v, want %v", features.SlashCommands, a.SupportsSlashCommands())
	}
	if features.FileSubAgents != a.SupportsSubAgents() {
		t.Errorf("Features.FileSubAgents = %v, want %v", features.FileSubAgents, a.SupportsSubAgents())
	}
	if features.Skills != a.SupportsSkills() {
		t.Errorf("Features.Skills = %v, want %v", features.Skills, a.SupportsSkills())
	}
	if features.SystemPrompt != a.SupportsSystemPrompt() {
		t.Errorf("Features.SystemPrompt = %v, want %v", features.SystemPrompt, a.SupportsSystemPrompt())
	}
	if features.MCP != a.SupportsMCP() {
		t.Errorf("Features.MCP = %v, want %v", features.MCP, a.SupportsMCP())
	}
}

// TestAdapter_ManifestResolves pins that MustForAgent succeeds for CodeBuddy.
func TestAdapter_ManifestResolves(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MustForAgent() panicked for CodeBuddy: %v", r)
		}
	}()
	_ = capabilitymanifest.MustForAgent(model.AgentCodeBuddy)
}
