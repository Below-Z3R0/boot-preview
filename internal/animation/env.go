package animation

import (
	"os"
	"strings"
)

func envPrefix() string {
	var parts []string
	// Se pasan SOLO estas variables. WAYLAND_DISPLAY no va a propósito:
	// el renderer x11.so usa GTK, y si GTK encuentra un servidor Wayland
	// elige ese backend en vez del X11. La ventana se abriría en la
	// sesión Wayland (donde no la ve el 'import' de X) y el preview
	// parecería no hacer nada.
	//
	// El daemon corre como root: su entorno viene del shell que lo
	// lanzó, así que hay que pasarle el DISPLAY a mano.
	for _, name := range []string{"DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR"} {
		if value := os.Getenv(name); value != "" {
			parts = append(parts, name+"="+shellQuote(value))
		}
	}
	// TERM siempre: sin él Plymouth/GTK puede fallar al inicializar.
	term := os.Getenv("TERM")
	if term == "" {
		term = "unknown"
	}
	parts = append(parts, "TERM="+shellQuote(term))
	return "env " + strings.Join(parts, " ")
}

// shellQuote entrecomilla un valor para el shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// replaceLine cambia el valor de una clave del .plymouth.
func replaceLine(content, key, value string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// La clave exacta, con los espacios que quiera alrededor de los
		// dos puntos: "clave: valor" y "clave : valor".
		//
		// Solo la clave exacta: "ImageDir" no debe pisar "ImageDirBackup"
		// ni "ImageDirExtra".
		trimmed := strings.TrimSpace(line)
		name, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			name, _, ok = strings.Cut(trimmed, "=")
		}
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		lines[i] = key + "=" + value
		break
	}
	return strings.Join(lines, "\n")
}

// printLogTail muestra las últimas líneas del log de plymouthd.
