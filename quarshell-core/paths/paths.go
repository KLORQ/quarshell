// Package paths resolves OpenShell's XDG configuration, state, and data roots.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// XDGStateDir returns XDG_STATE_HOME, LOCALAPPDATA on Windows, or
// $HOME/.local/state, following OpenShell's core path rules.
func XDGStateDir() (string, error) {
	if value := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); value != "" {
		return value, nil
	}
	if runtime.GOOS == "windows" {
		if value := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); value != "" {
			return value, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("resolve XDG state directory: HOME is not set")
	}
	return filepath.Join(home, ".local", "state"), nil
}

// OpenShellStateDir returns the shared OpenShell state root.
func OpenShellStateDir() (string, error) {
	base, err := XDGStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "openshell"), nil
}

// XDGDataDir returns XDG_DATA_HOME, LOCALAPPDATA on Windows, or
// $HOME/.local/share, following OpenShell's core path rules.
func XDGDataDir() (string, error) {
	if value := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); value != "" {
		return value, nil
	}
	if runtime.GOOS == "windows" {
		if value := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); value != "" {
			return value, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("resolve XDG data directory: HOME is not set")
	}
	return filepath.Join(home, ".local", "share"), nil
}

// OpenShellDataDir returns the shared OpenShell data root.
func OpenShellDataDir() (string, error) {
	base, err := XDGDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "openshell"), nil
}
