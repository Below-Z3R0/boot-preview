package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsThemeOption(t *testing.T) {
	// Las opciones que el apply se lleva al Limine real.
	theme := []string{
		"wallpaper:            boot/wallpaper.png",
		"wallpaper_style:      blurred",
		"backdrop:             000000",
		"term_margin:          true",
		"term_foreground:      ffffff",
		"term_font_scale:      1.0",
		"interface_resolution: 1920x1080",
		"interface_branding:   true",
	}

	for _, line := range theme {
		if !isThemeOption(line) {
			t.Errorf("isThemeOption(%q) dio false, quiero true", line)
		}
	}
}

func TestIsThemeOptionNo(t *testing.T) {
	// Estas no son de theming: no se tocan al aplicar.
	notTheme := []string{
		"timeout:              3",
		"default_entry:        1",
		"icons_dir:            boot/icons",
		"verbose:              false",
		"",
		"# un comentario",
		"  ",
	}

	for _, line := range notTheme {
		if isThemeOption(line) {
			t.Errorf("isThemeOption(%q) dio true, quiero false", line)
		}
	}
}

func TestExtractThemeBlock(t *testing.T) {
	const content = `timeout:              3
default_entry:        1
wallpaper:            boot/wallpaper.png
backdrop:             000000
term_foreground:      ffffff
icons_dir:            boot/icons
interface_resolution: 1920x1080
`

	block := extractThemeBlock(content)

	// Solo las opciones de theming, en el orden del archivo.
	want := []string{
		"wallpaper:            boot/wallpaper.png",
		"backdrop:             000000",
		"term_foreground:      ffffff",
		"interface_resolution: 1920x1080",
	}

	if len(block) != len(want) {
		t.Fatalf("extractThemeBlock() devolvio %d lineas, quiero %d: %v", len(block), len(want), block)
	}
	for i := range want {
		if block[i] != want[i] {
			t.Errorf("linea %d = %q, quiero %q", i, block[i], want[i])
		}
	}
}

func TestExtractThemeBlockVacio(t *testing.T) {
	if block := extractThemeBlock("timeout: 3\n"); len(block) != 0 {
		t.Errorf("extractThemeBlock() devolvio %v, quiero nada", block)
	}
	if block := extractThemeBlock(""); len(block) != 0 {
		t.Errorf("extractThemeBlock(\"\") devolvio %v", block)
	}
}

func TestIsOwnComment(t *testing.T) {
	// Los comentarios que escribe boot-preview se reconocen para poder
	// borrarlos en una reinstalacion: sin esto, el apply duplicaria el
	// bloque en cada corrida.
	own := []string{
		"# ── Limine theming (b3l0wz3r0-os) ──",
		"# Aplicado por boot-preview loader apply",
		"# y volvé a aplicar para cambiarlo",
		"  # ── Limine theming (b3l0wz3r0-os) ──",
	}

	for _, line := range own {
		if !isOwnComment(line) {
			t.Errorf("isOwnComment(%q) dio false, quiero true", line)
		}
	}

	other := []string{
		"# un comentario del usuario",
		"# Limine",
		"#",
		"",
	}

	for _, line := range other {
		if isOwnComment(line) {
			t.Errorf("isOwnComment(%q) dio true, quiero false", line)
		}
	}
}

func TestOptionLineRe(t *testing.T) {
	re := optionLineRe("wallpaper|banner")

	if !re.MatchString("wallpaper:  x.png") {
		t.Error("el regex no agarra wallpaper")
	}
	if !re.MatchString("banner:  x.png") {
		t.Error("el regex no agarra banner")
	}
	if re.MatchString("timeout: 3") {
		t.Error("el regex agarra timeout, que no esta en la lista")
	}
}

func TestExtractThemeBlockNoDuplica(t *testing.T) {
	// Si la misma opcion aparece dos veces, el bloque tiene que traer las
	// dos: el apply respeta el archivo tal cual.
	const content = `wallpaper: a.png
wallpaper: b.png
`
	block := extractThemeBlock(content)
	if len(block) != 2 {
		t.Errorf("extractThemeBlock() devolvio %d lineas, quiero 2: %v", len(block), block)
	}
	if !strings.Contains(block[0], "a.png") || !strings.Contains(block[1], "b.png") {
		t.Errorf("el bloque no respeta el orden: %v", block)
	}
}

// TestApplyEsIdempotente verifica la propiedad clave del apply: aplicarlo
// dos veces deja el mismo archivo.
//
// Es la garantia de que se puede volver a aplicar sin miedo a duplicar
// el bloque de theming.
func TestApplyEsIdempotente(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limine.conf")

	const original = `timeout:              3
wallpaper:            boot/wallpaper.png
interface_resolution: 1920x1080
`

	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := injectPreviewOptions(path, "1920x1080"); err != nil {
		t.Fatal(err)
	}
	once, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := injectPreviewOptions(path, "1920x1080"); err != nil {
		t.Fatal(err)
	}
	twice, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(once) != string(twice) {
		t.Errorf("el apply no es idempotente:\n--- una vez ---\n%s\n--- dos veces ---\n%s", once, twice)
	}
}

// TestInyectaForzaTimeoutNo verifica que el timeout quede en "no": con un
// timeout real, Limine autobootea la entrada default y el menu se va
// antes de que se pueda mirar.
func TestInyectaForzaTimeoutNo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limine.conf")

	const original = "timeout:              3\ndefault_entry:        1\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := injectPreviewOptions(path, "1280x720"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	if !strings.Contains(text, "timeout: no") {
		t.Errorf("el timeout no quedo en no:\n%s", text)
	}
	if strings.Contains(text, "timeout:              3") {
		t.Errorf("el timeout viejo quedo:\n%s", text)
	}
	if !strings.Contains(text, "interface_resolution: 1280x720") {
		t.Errorf("no se inyecto la resolucion:\n%s", text)
	}
	// Lo que no es de theming no se toca.
	if !strings.Contains(text, "default_entry:        1") {
		t.Errorf("se toco una opcion que no es de theming:\n%s", text)
	}
}
