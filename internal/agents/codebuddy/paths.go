package codebuddy

import "path/filepath"

func ConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".codebuddy")
}
