package loader

import (
	"regexp"
	"strings"
)

// ── El parseo del limine.conf ────────────────────────────────────────────────
//
// Las opciones son "clave: valor" a nivel global (antes de la primera
// entrada) o dentro de una entrada. Acá vive lo que lee y escribe el
// bloque de theming; el comando (apply) está en apply.go.

// optionRe matchea una opción del conf ("clave: valor").
var optionRe = regexp.MustCompile(`^\s*([a-z_0-9]+)\s*:`)

// extractThemeBlock saca las opciones de theming de un conf.
func extractThemeBlock(content string) []string {
	var block []string
	for _, line := range strings.Split(content, "\n") {
		match := optionRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for _, option := range themeOptions {
			if match[1] == option {
				block = append(block, line)
				break
			}
		}
	}
	return block
}

// isThemeOption dice si una línea es una opción de theming.
func isThemeOption(line string) bool {
	match := optionRe.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	for _, option := range themeOptions {
		if match[1] == option {
			return true
		}
	}
	return false
}

// isOwnComment dice si la línea es un comentario que escribió boot-preview.
func isOwnComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, comment := range ownComments {
		if strings.HasPrefix(trimmed, comment) {
			return true
		}
	}
	return false
}

// rewriteWallpaperForReal cambia el wallpaper del preview (el del ISO)
// por el de la ESP.
//
// Se matchea con la misma regex que usa la extracción: la línea puede
// venir indentada o con espacios antes de los dos puntos
// ("  wallpaper : boot():/wallpaper.png").
func rewriteWallpaperForReal(block []string) {
	for i, line := range block {
		match := optionRe.FindStringSubmatch(line)
		if match != nil && match[1] == "wallpaper" {
			block[i] = "wallpaper: boot():/splash.png"
		}
	}
}
