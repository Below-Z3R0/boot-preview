package cli

import "fmt"

// ── Salida ──────────────────────────────────────────────────────────────────
//
// La paleta es Rosé Pine, la misma que usaba la versión en bash:
//
//	▸ step    Rosewater  235;194;186
//	✓ ok      Sapphire   125;207;216
//	! warn    Red        246;153;150
//	[algo]    Green      166;227;161  (solo en sources)
const (
	colorReset    = "\033[0m"
	colorBold     = "\033[1m"
	colorRose     = "\033[38;2;235;194;186m"
	colorSapphire = "\033[38;2;125;207;216m"
	colorRed      = "\033[38;2;246;153;150m"
	colorGreen    = "\033[38;2;166;227;161m"
)

// Say imprime una línea indentada.
func Say(format string, args ...any) {
	fmt.Printf("  %s\n", fmt.Sprintf(format, args...))
}

// Step imprime un encabezado de sección.
func Step(format string, args ...any) {
	fmt.Printf("\n%s▸%s %s%s%s\n",
		colorRose, colorReset,
		colorBold, fmt.Sprintf(format, args...), colorReset)
}

// Ok imprime un éxito.
func Ok(format string, args ...any) {
	fmt.Printf("  %s✓%s %s\n", colorSapphire, colorReset, fmt.Sprintf(format, args...))
}

// Warn imprime un aviso.
func Warn(format string, args ...any) {
	fmt.Printf("  %s! %s%s\n", colorRed, fmt.Sprintf(format, args...), colorReset)
}

// Green colorea un texto con el verde de la paleta (para las etiquetas de
// [sistema] y [preview]).
func Green(s string) string {
	return colorGreen + s + colorReset
}
