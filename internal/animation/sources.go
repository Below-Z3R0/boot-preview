package animation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── sources ──────────────────────────────────────────────────────────────────

// ── sources ────────────────────────────────────────────────────────────────

// cmdSources lista los temas de Plymouth instalados.
func cmdSources(paths cli.Paths) error {
	actual := systemTheme()
	preview := previewTheme(paths)

	cli.Step("Temas de Plymouth instalados")
	cli.Say("directorio: %s/", themesDir)
	fmt.Println()

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("no se pudo leer %s: %w", themesDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		module := themeModule(filepath.Join(themesDir, name))

		var marca string
		if name == actual {
			marca += " " + cli.Green("[sistema]")
		}
		if name == preview {
			marca += " " + cli.Green("[preview]")
		}
		if module == "" {
			module = "?"
		}
		fmt.Printf("    %-30s %-12s%s\n", name, module, marca)
	}

	fmt.Println()
	cli.Say("sistema: %s", actual)
	cli.Say("preview: %s", preview)
	return nil
}

// themeModule lee el ModuleName del .plymouth de un tema.
// themeModule devuelve el módulo de Plymouth de un tema.
//
// Si no lo dice el .plymouth, se asume "script": es el único que sabe
// interpretar una secuencia de PNGs, y es el que usa Plymouth por
// defecto cuando el .plymouth no especifica nada.
func themeModule(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "script"
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".plymouth") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line != "ModuleName" && !strings.HasPrefix(line, "ModuleName=") {
				continue
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, "ModuleName"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
			if rest != "" {
				return rest
			}
		}
	}
	return "script"
}
