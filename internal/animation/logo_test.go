package animation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidarPNG(t *testing.T) {
	dir := t.TempDir()

	png := filepath.Join(dir, "logo.png")
	if err := os.WriteFile(png, append(firmaPNG, []byte("resto")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validarPNG(png); err != nil {
		t.Fatalf("PNG válido rechazado: %v", err)
	}

	txt := filepath.Join(dir, "logo.txt")
	os.WriteFile(txt, []byte("hola"), 0o644)
	if err := validarPNG(txt); err == nil {
		t.Fatal("esperaba error con un archivo que no es PNG")
	}

	if err := validarPNG(filepath.Join(dir, "no-existe.png")); err == nil {
		t.Fatal("esperaba error con un archivo inexistente")
	}
}
