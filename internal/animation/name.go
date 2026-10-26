package animation

import (
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/steam"
)

func themeNameFor(video, home string) string {
	base := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))

	if id, _, ok := strings.Cut(base, "_"); ok && isDigits(id) {
		for _, m := range steam.Movies(home) {
			if m.ID == id && m.Name != "" {
				base = m.Name
				break
			}
		}
		if base == id {
			base = "item-" + id
		}
	}

	// A slug: minúsculas, espacios y símbolos a guiones.
	slug := strings.ToLower(base)
	var b strings.Builder
	lastDash := false
	for _, r := range slug {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug = strings.Trim(b.String(), "-")
	if slug == "" {
		// Un nombre sin letras ni números (todo símbolos) daba "steam-",
		// que no es un nombre de tema válido.
		slug = "video"
	}

	if strings.HasPrefix(slug, "steam") {
		return slug
	}
	return "steam-" + slug
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// convertVideoToTheme extrae los frames y arma el tema.
