package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewPathsRespetaXDG verifica que los paths siguen las variables de
// entorno XDG.
//
// El bug: el verify del HOME real找不到 la config del HOME aislado.
func TestNewPathsRespetaXDG(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, "config")
	dataHome := filepath.Join(home, "data")

	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("SUDO_USER", "")

	p := NewPaths()

	if want := filepath.Join(configHome, "boot-preview"); p.Config != want {
		t.Errorf("Config = %q, quiero %q", p.Config, want)
	}
	if want := filepath.Join(home, ".local", "bin"); p.Bin != want {
		t.Errorf("Bin = %q, quiero %q", p.Bin, want)
	}
	if want := filepath.Join(home, ".local", "lib", "boot-preview"); p.Lib != want {
		t.Errorf("Lib = %q, quiero %q", p.Lib, want)
	}
}

// TestPreviewsRootSinRepo verifica que, sin repo, las capturas van a
// ~/.local/share/boot-preview (el caso de la instalacion desde el
// bundle).
func TestPreviewsRootSinRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("SUDO_USER", "")

	p := NewPaths()
	p.Repo = ""

	want := filepath.Join(home, ".local", "share", "boot-preview")
	if got := p.PreviewsRoot(); got != want {
		t.Errorf("PreviewsRoot() = %q, quiero %q", got, want)
	}
}

// TestIsRepoDetectaElRepo verifica la deteccion del repo.
//
// El bug: buscaba loader.sh (el script de bash) y con el repo en Go no
// lo encontraba.
func TestIsRepoDetectaElRepo(t *testing.T) {
	dir := t.TempDir()
	if isRepo(dir) {
		t.Error("isRepo() dio true para una carpeta vacia")
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isRepo(dir) {
		t.Error("isRepo() dio false con un go.mod adentro")
	}
}

// TestCopyFileCreaLosDirectorios verifica que la copia crea la carpeta
// del destino.
//
// El bug: copyDir no creaba el destino y el install fallaba.
func TestCopyFileCreaLosDirectorios(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "origen.txt")
	dest := filepath.Join(dir, "sub", "carpeta", "destino.txt")

	if err := os.WriteFile(src, []byte("contenido"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(src, dest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "contenido" {
		t.Errorf("copio %q", data)
	}
}

// TestCopyDirCopiaRecursivamente verifica la copia de carpetas.
func TestCopyDirCopiaRecursivamente(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "origen")
	dest := filepath.Join(dir, "destino")

	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(src, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "sub", "b.txt")); err != nil {
		t.Errorf("no se copio el archivo anidado: %v", err)
	}
}

// TestParseRes verifica el parseo de la resolucion.
func TestParseRes(t *testing.T) {
	w, h, err := ParseRes("1920x1080")
	if err != nil || w != 1920 || h != 1080 {
		t.Errorf("ParseRes(\"1920x1080\") = %d x %d, err=%v", w, h, err)
	}

	if _, _, err := ParseRes("no-es-una-resolucion"); err == nil {
		t.Error("ParseRes() dio nil error para una basura")
	}
	if _, _, err := ParseRes("1920"); err == nil {
		t.Error("ParseRes() dio nil error para una resolucion sin alto")
	}
}
