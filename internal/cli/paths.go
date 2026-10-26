package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── Rutas ───────────────────────────────────────────────────────────────────

// Paths son las rutas que usa el tool.
type ToolPaths struct {
	// Config es ~/.config/boot-preview
	Config string
	// Repo es la ruta del repo clonado (vacía si no está).
	Repo string
	// Bin es ~/.local/bin
	Bin string
	// Lib es ~/.local/lib/boot-preview
	Lib string
}

// Home devuelve el home del usuario real (no el de root cuando se corre
// con sudo).
func Home() string {
	if sudo := os.Getenv("SUDO_USER"); sudo != "" && sudo != "root" {
		if out, err := exec.Command("getent", "passwd", sudo).Output(); err == nil {
			fields := strings.Split(strings.TrimSpace(string(out)), ":")
			if len(fields) >= 6 && fields[5] != "" {
				return fields[5]
			}
		}
	}
	return os.Getenv("HOME")
}

// NewPaths arma las rutas a partir del entorno.
func NewPaths() ToolPaths {
	home := Home()
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	p := ToolPaths{
		Config: filepath.Join(configHome, "boot-preview"),
		Bin:    filepath.Join(home, ".local", "bin"),
		Lib:    filepath.Join(home, ".local", "lib", "boot-preview"),
	}

	// El repo: primero el que está al lado del binario (instalado), si no
	// el que guardó el instalador.
	//
	// Se busca un repo Go (go.mod) o el árbol de assets del bundle. El
	// repo no es obligatorio: si el tool se instaló desde el bundle, los
	// assets ya están en paths.Lib y no hay nada que clonar.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		// dir, su padre y su abuelo. filepath.Join ya limpia el "..":
		// envolverlo en otro filepath.Dir subía UN NIVEL DE MÁS, así que
		// desde dist/boot-preview nunca se encontraba el repo.
		for _, candidate := range []string{
			dir,
			filepath.Join(dir, ".."),
			filepath.Join(dir, "..", ".."),
		} {
			if isRepo(candidate) {
				p.Repo = candidate
				break
			}
		}
	}
	if p.Repo == "" {
		if data, err := os.ReadFile(filepath.Join(p.Config, "repo")); err == nil {
			candidate := strings.TrimSpace(string(data))
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				p.Repo = candidate
			}
		}
	}

	return p
}

// isRepo dice si una carpeta es el repo de boot-preview.
//
// Se reconoce por el go.mod DEL módulo o por el árbol de fuentes.
//
// NO alcanza con que exista un go.mod cualquiera, ni con "examples" o
// "scripts": casi cualquier proyecto tiene un scripts/, y el loop de
// NewPaths llega hasta dos niveles arriba del binario, o sea el home del
// usuario. Con esos marcadores, ~/ pasaba por repo y las capturas se
// guardaban en ~/previews/.
func isRepo(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err == nil && strings.Contains(string(data), "module github.com/Below-Z3R0/boot-preview") {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "cmd", "boot-preview", "main.go")); err == nil {
		return true
	}
	return false
}

// LoaderConfig es la carpeta de configuración del módulo de Limine.
func (p ToolPaths) LoaderConfig() string { return filepath.Join(p.Config, "loader") }

// AnimationConfig es la carpeta de configuración del módulo de Plymouth.
func (p ToolPaths) AnimationConfig() string { return filepath.Join(p.Config, "animation") }

// PreviewsRoot es la carpeta base donde van las capturas.
//
// Es el repo si hay uno; si el tool se instaló desde el bundle (sin repo)
// es ~/.local/share/boot-preview.
//
// OJO: se usa ~/.local/share y NO $XDG_DATA_HOME, para que quede en el
// mismo lugar que el binario y los assets (~/.local/bin y
// ~/.local/lib). Con XDG_DATA_HOME apuntando a otro lado, las capturas
// quedarian separadas del resto del tool.
func (p ToolPaths) PreviewsRoot() string {
	if p.Repo != "" {
		return p.Repo
	}
	return filepath.Join(Home(), ".local", "share", "boot-preview")
}

// LoaderPreviews es la carpeta de capturas del menú de Limine.
func (p ToolPaths) LoaderPreviews() string {
	return filepath.Join(p.PreviewsRoot(), "previews", "loader")
}

// AnimationPreviews es la carpeta de capturas del splash de Plymouth.
func (p ToolPaths) AnimationPreviews() string {
	return filepath.Join(p.PreviewsRoot(), "previews", "animation")
}
