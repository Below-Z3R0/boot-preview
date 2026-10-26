package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInjectPreviewOptionsEsIdempotente verifica que inyectar dos veces
// deja el mismo archivo.
//
// El bug: el bloque inyectado no se borraba, asi que cada corrida
// duplicaba las opciones.
func TestInjectPreviewOptionsEsIdempotente(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limine.conf")

	const original = `timeout: 3
default_entry: 1
wallpaper: boot():/wallpaper.png
interface_resolution: 1280x720
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
		t.Errorf("inyectar dos veces no es idempotente:\n--- una ---\n%s\n--- dos ---\n%s", once, twice)
	}
}

// TestInjectPreviewOptionsNoDuplicaLasOpciones cuenta las lineas de cada
// opcion: tiene que haber una sola de cada una.
func TestInjectPreviewOptionsNoDuplicaLasOpciones(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limine.conf")

	const original = `timeout: 3
default_entry: 1
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	for range 3 {
		if err := injectPreviewOptions(path, "1920x1080"); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for _, opcion := range []string{"timeout:", "wallpaper:", "wallpaper_style:", "interface_resolution:"} {
		if n := strings.Count(text, opcion); n != 1 {
			t.Errorf("hay %d lineas con %q, quiero 1:\n%s", n, opcion, text)
		}
	}
}

// TestInjectPreviewOptionsFuerzaElTimeoutNo verifica que el timeout
// quede en "no".
//
// Con un timeout real, Limine autobootea la entrada default (que en el
// config real apunta a un kernel con hash que no esta en el ISO) y se va
// de la pantalla antes de que puedas mirarla.
func TestInjectPreviewOptionsFuerzaElTimeoutNo(t *testing.T) {
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
}

// TestInjectPreviewOptionsNoTocaLoQueNoEsDeTheming verifica que las
// entradas del menu siguen intactas.
func TestInjectPreviewOptionsNoTocaLoQueNoEsDeTheming(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "limine.conf")

	const original = `timeout: 3
default_entry: 2
/+CachyOS
  //linux-cachyos
  protocol: linux
  path: boot():/vmlinuz
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := injectPreviewOptions(path, "1920x1080"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for _, linea := range []string{"default_entry: 2", "/+CachyOS", "//linux-cachyos", "path: boot():/vmlinuz"} {
		if !strings.Contains(text, linea) {
			t.Errorf("se perdio %q:\n%s", linea, text)
		}
	}
}

// TestApplyThemeBlockConservaLasEntradas verifica que el apply real no
// pierde las entradas del menu.
//
// El apply real trabaja sobre /boot/limine.conf, asi que aqui se prueba
// la parte de la limpieza: que solo saque las opciones de theming.
func TestApplyThemeBlockConservaLasEntradas(t *testing.T) {
	const real = `timeout: 3
default_entry: 2
wallpaper: boot():/splash.png
term_margin: 0
/+CachyOS
  //linux-cachyos
  protocol: linux
`

	lines := strings.Split(real, "\n")
	var kept []string
	skipping := false
	for _, line := range lines {
		if isThemeOption(line) {
			continue
		}
		if isOwnComment(line) {
			skipping = true
			continue
		}
		if skipping {
			if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
				continue
			}
			skipping = false
		}
		kept = append(kept, line)
	}

	resultado := strings.Join(kept, "\n")
	for _, linea := range []string{"timeout: 3", "default_entry: 2", "/+CachyOS", "//linux-cachyos", "protocol: linux"} {
		if !strings.Contains(resultado, linea) {
			t.Errorf("se perdio %q:\n%s", linea, resultado)
		}
	}
	// Y las de theming, no.
	for _, opcion := range []string{"wallpaper:", "term_margin:"} {
		if strings.Contains(resultado, opcion) {
			t.Errorf("quedo la opcion de theming %q:\n%s", opcion, resultado)
		}
	}
}

// TestIsOwnCommentReconoceElViejo verifica que el apply borre el bloque
// que dejaron las versiones viejas del tool.
//
// El tool se llamaba limine-preview antes, asi que los configs que hay
// instalados en /boot tienen el comentario viejo.
func TestIsOwnCommentReconoceElViejo(t *testing.T) {
	propios := []string{
		"# ── Limine theming (b3l0wz3r0-os) ──",
		"# Aplicado por boot-preview loader apply",
		"# Aplicado por limine-preview apply",
	}
	for _, linea := range propios {
		if !isOwnComment(linea) {
			t.Errorf("isOwnComment(%q) dio false, quiero true", linea)
		}
	}

	ajenos := []string{"# un comentario del usuario", "# Limine", ""}
	for _, linea := range ajenos {
		if isOwnComment(linea) {
			t.Errorf("isOwnComment(%q) dio true, quiero false", linea)
		}
	}
}

// TestExtractThemeBlockSacaLoQueNoEs verifica que el bloque solo tiene
// las opciones de theming.
func TestExtractThemeBlockSacaLoQueNoEs(t *testing.T) {
	const config = `timeout: 3
default_entry: 1
wallpaper: boot():/wallpaper.png
term_margin: 0
interface_branding: b3low-z3r0
/+CachyOS
  //linux-cachyos
`

	block := extractThemeBlock(config)
	if len(block) != 3 {
		t.Fatalf("extractThemeBlock() devolvio %d lineas, quiero 3: %v", len(block), block)
	}
	for _, linea := range block {
		if strings.HasPrefix(strings.TrimSpace(linea), "timeout") ||
			strings.HasPrefix(strings.TrimSpace(linea), "default_entry") ||
			strings.HasPrefix(strings.TrimSpace(linea), "/") {
			t.Errorf("el bloque trae algo que no es de theming: %q", linea)
		}
	}
}

// TestRewriteWallpaperForRealMatcheaIndentado verifica que el wallpaper
// se reescribe aunque la linea venga indentada o con espacios antes de
// los dos puntos.
//
// El bug: se buscaba strings.HasPrefix(line, "wallpaper:"), asi que una
// linea "  wallpaper : boot():/wallpaper.png" no matcheaba y el conf
// real quedaba apuntando a un archivo que no existe en la ESP.
func TestRewriteWallpaperForRealMatcheaIndentado(t *testing.T) {
	block := []string{
		"  wallpaper : boot():/wallpaper.png",
		"wallpaper: boot():/wallpaper.png",
		"  wallpaper_style: centered",
		"interface_resolution: 1920x1080",
	}

	rewriteWallpaperForReal(block)

	const want = "wallpaper: boot():/splash.png"
	if block[0] != want {
		t.Errorf("block[0] = %q, quiero %q", block[0], want)
	}
	if block[1] != want {
		t.Errorf("block[1] = %q, quiero %q", block[1], want)
	}
	// wallpaper_style no se toca.
	if block[2] != "  wallpaper_style: centered" {
		t.Errorf("se toco wallpaper_style: %q", block[2])
	}
	if block[3] != "interface_resolution: 1920x1080" {
		t.Errorf("se toco interface_resolution: %q", block[3])
	}
}

// TestIsThemeOptionConIndentacion verifica que la deteccion de opciones
// de theming no depende de la columna.
func TestIsThemeOptionConIndentacion(t *testing.T) {
	for _, line := range []string{
		"wallpaper: boot():/x.png",
		"  wallpaper: boot():/x.png",
		"\twallpaper_style: centered",
		"interface_resolution : 1280x720",
	} {
		if !isThemeOption(line) {
			t.Errorf("isThemeOption(%q) dio false", line)
		}
	}

	for _, line := range []string{
		"timeout: 3",
		"/+CachyOS",
		"path: boot():/vmlinuz",
		"# wallpaper: comentado",
	} {
		if isThemeOption(line) {
			t.Errorf("isThemeOption(%q) dio true", line)
		}
	}
}
