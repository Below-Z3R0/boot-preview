package loader

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ── La config que se le pasa al ISO ────────────────────────────────────────
//
// Al conf del preview se le inyectan (o reemplazan) las opciones de
// theming: el wallpaper del ISO, la resolución y el timeout. El bloque es
// idempotente: aplicarlo dos veces deja el mismo archivo.

// injectComment es el comentario que marca el bloque que inyecta
// boot-preview. Se borra antes de reinyectar, para que el apply sea
// idempotente.
const injectComment = "# ── inyectado por boot-preview loader ──"

// injectPreviewOptions mete las opciones del preview en el conf.
//
// OJO: en Limine las opciones GLOBALES tienen que ir antes de la primera
// entrada de menú (una línea que empieza con "/"). Si se inyectan al
// final, Limine las trata como opciones de esa entrada y las ignora.
func injectPreviewOptions(path, resolution string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Se borra el bloque inyectado en una corrida anterior (si lo hay),
	// para que la inyección sea idempotente: aplicarla dos veces tiene
	// que dejar el mismo archivo.
	lines := strings.Split(string(data), "\n")
	var kept []string
	skipping := false
	for _, line := range lines {
		if strings.TrimSpace(line) == injectComment {
			skipping = true
			continue
		}
		if skipping {
			// Se saltean las líneas en blanco que quedaron del bloque
			// anterior; la primera con contenido NO se descarta (puede ser
			// una entrada del menú).
			if strings.TrimSpace(line) == "" {
				continue
			}
			skipping = false
		}
		kept = append(kept, line)
	}
	text := strings.Join(kept, "\n")

	// Se borran las opciones de theming previas para que la config
	// inyectada gane.
	text = optionLineRe("wallpaper|wallpaper_style|interface_resolution").ReplaceAllString(text, "")
	// El timeout se fuerza a "no": si no, Limine autobootea la entrada
	// default (que en la config real apunta a un kernel con hash que no
	// está en el ISO) y se va de la pantalla antes de que puedas mirarla.
	text = optionLineRe("timeout").ReplaceAllString(text, "")

	inject := fmt.Sprintf(`%s
timeout: no
wallpaper: boot():/wallpaper.png
wallpaper_style: centered
interface_resolution: %s
`, injectComment, resolution)

	lines = strings.Split(text, "\n")
	insertAt := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "/") {
			insertAt = i
			break
		}
	}

	var out []string
	out = append(out, lines[:insertAt]...)
	out = append(out, strings.TrimRight(inject, "\n"))
	out = append(out, lines[insertAt:]...)

	result := strings.Join(out, "\n")
	// Las opciones borradas dejan lineas vacias: se colapsan para que el
	// archivo no crezca con cada inyeccion.
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return os.WriteFile(path, []byte(result+"\n"), 0o644)
}

// optionLineRe matchea una línea de opción completa.
func optionLineRe(names string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*(` + names + `)\s*:.*$\n?`)
}
