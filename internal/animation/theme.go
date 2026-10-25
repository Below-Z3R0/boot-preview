package animation

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── El tema ───────────────────────────────────────────────────────────────────

// ── El tema ─────────────────────────────────────────────────────────────────

// systemTheme es el tema que Plymouth usa ahora.
func systemTheme() string {
	data, err := os.ReadFile(sysConf)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Theme") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "Theme"))
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(rest, "="))
	}
	return ""
}

// previewTheme es el tema que usa el preview (o el del sistema, si el
// usuario no eligió otro).
func previewTheme(paths cli.ToolPaths) string {
	data, err := os.ReadFile(filepath.Join(paths.AnimationConfig(), "theme"))
	if err != nil {
		return systemTheme()
	}
	return strings.TrimSpace(string(data))
}
