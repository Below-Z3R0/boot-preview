package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsRepo(t *testing.T) {
	// El go.mod DEL módulo: sí.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module github.com/Below-Z3R0/boot-preview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isRepo(dir) {
		t.Error("isRepo() dio false con el go.mod del módulo")
	}

	// Una carpeta vacía no.
	if isRepo(t.TempDir()) {
		t.Error("isRepo() dio true con una carpeta vacia")
	}

	// Un go.mod CUALQUIERA no alcanza: con eso, el home del usuario (que
	// puede tener cualquier cosa) pasaba por repo.
	ajeno := t.TempDir()
	if err := os.WriteFile(filepath.Join(ajeno, "go.mod"), []byte("module otra-cosa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isRepo(ajeno) {
		t.Error("isRepo() dio true con un go.mod de otro módulo")
	}

	// Un scripts/ suelto tampoco: lo tiene casi cualquier proyecto.
	conScripts := t.TempDir()
	if err := os.MkdirAll(filepath.Join(conScripts, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if isRepo(conScripts) {
		t.Error("isRepo() dio true solo por tener scripts/")
	}

	// El árbol de fuentes sí.
	fuentes := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fuentes, "cmd", "boot-preview"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fuentes, "cmd", "boot-preview", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isRepo(fuentes) {
		t.Error("isRepo() dio false con el árbol de fuentes")
	}
}

func TestPathsPreviews(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

	p := NewPaths()

	if p.Config != filepath.Join(home, ".config", "boot-preview") {
		t.Errorf("Config = %q", p.Config)
	}
	if p.Bin != filepath.Join(home, ".local", "bin") {
		t.Errorf("Bin = %q", p.Bin)
	}

	// Sin repo, las capturas van a ~/.local/share/boot-preview.
	want := filepath.Join(home, ".local", "share", "boot-preview", "previews", "loader")
	if got := p.LoaderPreviews(); got != want {
		t.Errorf("LoaderPreviews() = %q, quiero %q", got, want)
	}
}

func TestPathsPreviewsConRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPaths()
	p.Repo = repo

	want := filepath.Join(repo, "previews", "animation")
	if got := p.AnimationPreviews(); got != want {
		t.Errorf("AnimationPreviews() = %q, quiero %q", got, want)
	}
}

func TestHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := Home(); got != home {
		t.Errorf("Home() = %q, quiero %q", got, home)
	}
}

func TestHaveCommand(t *testing.T) {
	if !HaveCommand("sh") {
		t.Error("HaveCommand(\"sh\") dio false")
	}
	if HaveCommand("este-comando-no-existe-nada") {
		t.Error("HaveCommand() dio true para un comando inexistente")
	}
}

func TestCommandPath(t *testing.T) {
	if CommandPath("este-comando-no-existe-nada") != "" {
		t.Error("CommandPath() devolvio una ruta para un comando inexistente")
	}
	if !strings.HasSuffix(CommandPath("sh"), "sh") {
		t.Error("CommandPath(\"sh\") no termina en sh")
	}
}
