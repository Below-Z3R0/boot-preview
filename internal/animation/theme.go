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
	return iniValue(string(data), "Theme")
}

// iniValue devuelve el valor de una clave de un .plymouth / .conf.
//
// El formato es INI sin secciones: "Clave=valor", con espacios
// opcionales alrededor. Se matchea la clave exacta: "Theme" no debe
// tomar "ThemeName".
func iniValue(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, key) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, key))
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(rest, "="))
		if value != "" {
			return value
		}
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

	name := strings.TrimSpace(string(data))
	// Si el tema elegido ya no está (se desinstaló, o la config se copió
	// de otra máquina), el preview quedaba roto con "no existe el tema: ".
	if _, err := os.Stat(filepath.Join(themesDir, name)); err != nil {
		return systemTheme()
	}
	return name
}
