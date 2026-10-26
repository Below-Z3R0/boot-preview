package wallpaper

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// fixture genera una imagen sólida del tamaño pedido.
//
// Se usa magick porque ya es dependencia del paquete; si no está, se
// saltea el test en vez de fallar (CI sin ImageMagick).
func fixture(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	if !cli.HaveCommand("magick") && !cli.HaveCommand("vips") {
		t.Skip("sin vips ni imagemagick")
	}

	path := filepath.Join(dir, name)
	var cmd *exec.Cmd
	if cli.HaveCommand("magick") {
		cmd = exec.Command("magick", "-size", strconv.Itoa(w)+"x"+strconv.Itoa(h), "xc:#335577", path)
	} else {
		cmd = exec.Command("vips", "black", path, strconv.Itoa(w), strconv.Itoa(h))
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("no se pudo generar el fixture: %s", out)
	}
	return path
}

// TestPrepareGeneraElTamanoPedido verifica el resize exacto.
//
// Limine dibuja el wallpaper tal cual: si no queda en la resolución de la
// interfaz, se ve reescalado.
func TestPrepareGeneraElTamanoPedido(t *testing.T) {
	dir := t.TempDir()
	src := fixture(t, dir, "in.png", 100, 100)
	out := filepath.Join(dir, "out.png")

	if err := Prepare(Options{Src: src, Out: out, Res: "64x36", Format: "png"}); err != nil {
		t.Fatalf("Prepare fallo: %v", err)
	}

	w, h := cli.ImageSize(out)
	if w != 64 || h != 36 {
		t.Errorf("salida %dx%d, quiero 64x36", w, h)
	}
}

// TestPrepareResInvalidaFalla verifica que una resolución basura no pase.
//
// Antes se convertía en 0x0 y el fallo aparecía recién en el comando de
// magick, con un mensaje que no explicaba nada.
func TestPrepareResInvalidaFalla(t *testing.T) {
	dir := t.TempDir()
	src := fixture(t, dir, "in.png", 50, 50)

	for _, res := range []string{"basura", "1920", "0x0", "abcxdef"} {
		err := Prepare(Options{Src: src, Out: filepath.Join(dir, "out.png"), Res: res})
		if err == nil {
			t.Errorf("Prepare aceptó la resolución %q", res)
		}
	}
}

// TestPrepareSinOrigenFalla verifica el error cuando falta la imagen.
func TestPrepareSinOrigenFalla(t *testing.T) {
	if err := Prepare(Options{Src: "/no/existe.png", Out: t.TempDir() + "/o.png"}); err == nil {
		t.Error("Prepare aceptó un origen inexistente")
	}
	if err := Prepare(Options{Src: ""}); err == nil {
		t.Error("Prepare aceptó un origen vacío")
	}
}

// TestPrepareConservaElAspectoDondeCorresponde verifica que sin --res la
// imagen no se deforme.
func TestPrepareConservaElAspectoDondeCorresponde(t *testing.T) {
	dir := t.TempDir()
	src := fixture(t, dir, "in.png", 80, 40)
	out := filepath.Join(dir, "out.png")

	if err := Prepare(Options{Src: src, Out: out, Format: "png"}); err != nil {
		t.Fatalf("Prepare fallo: %v", err)
	}
	w, h := cli.ImageSize(out)
	if w != 80 || h != 40 {
		t.Errorf("sin --res cambió el tamaño: %dx%d, quiero 80x40", w, h)
	}
}

// TestPrepareFormatoDesconocidoFalla verifica el enum del formato.
func TestPrepareFormatoDesconocidoFalla(t *testing.T) {
	dir := t.TempDir()
	src := fixture(t, dir, "in.png", 20, 20)

	err := Prepare(Options{Src: src, Out: filepath.Join(dir, "out.webp"), Format: "webp"})
	if err == nil {
		t.Error("Prepare aceptó el formato webp")
	}
}

// TestPrepareNoDejaBasura verifica que el temporal se limpie.
func TestPrepareNoDejaBasura(t *testing.T) {
	dir := t.TempDir()
	src := fixture(t, dir, "in.png", 30, 30)

	if err := Prepare(Options{Src: src, Out: filepath.Join(dir, "out.png"), Res: "20x20"}); err != nil {
		t.Fatalf("Prepare fallo: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// in.png + out.png, y nada más.
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("quedaron archivos de más: %v", names)
	}
}
