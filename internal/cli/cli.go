// Package cli tiene la base compartida por todos los comandos: los colores
// de la salida, las rutas de la configuración y los helpers de sistema.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// ── Rutas ───────────────────────────────────────────────────────────────────

// Paths son las rutas que usa el tool.
type Paths struct {
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
func NewPaths() Paths {
	home := Home()
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	p := Paths{
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
		for _, candidate := range []string{
			dir,
			filepath.Dir(filepath.Join(dir, "..")),
			filepath.Dir(filepath.Join(dir, "..", "..")),
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
// Se reconoce por el go.mod (repo de desarrollo) o por los examples (repo
// clonado sin compilar).
func isRepo(dir string) bool {
	for _, marker := range []string{"go.mod", "examples", "scripts"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// LoaderConfig es la carpeta de configuración del módulo de Limine.
func (p Paths) LoaderConfig() string { return filepath.Join(p.Config, "loader") }

// AnimationConfig es la carpeta de configuración del módulo de Plymouth.
func (p Paths) AnimationConfig() string { return filepath.Join(p.Config, "animation") }

// PreviewsRoot es la carpeta base donde van las capturas.
//
// Es el repo si hay uno; si el tool se instaló desde el bundle (sin repo)
// es ~/.local/share/boot-preview.
func (p Paths) PreviewsRoot() string {
	if p.Repo != "" {
		return p.Repo
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(Home(), ".local", "share")
	}
	return filepath.Join(dataHome, "boot-preview")
}

// LoaderPreviews es la carpeta de capturas del menú de Limine.
func (p Paths) LoaderPreviews() string { return filepath.Join(p.PreviewsRoot(), "previews", "loader") }

// AnimationPreviews es la carpeta de capturas del splash de Plymouth.
func (p Paths) AnimationPreviews() string {
	return filepath.Join(p.PreviewsRoot(), "previews", "animation")
}

// ── Sistema ─────────────────────────────────────────────────────────────────

// HaveCommand dice si un binario está en el PATH.
func HaveCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// CommandPath devuelve la ruta de un binario, o "" si no está.
func CommandPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// Run ejecuta un comando y devuelve su stdout. Si falla, devuelve el error.
func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

// RunCombined ejecuta un comando con stderr mezclado.
func RunCombined(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// PackageVersion devuelve la versión instalada de un paquete de pacman, o ""
// si no está instalado.
func PackageVersion(name string) string {
	out, err := Run("pacman", "-Q", name)
	if err != nil {
		return ""
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return ""
	}
	return fields[len(fields)-1]
}

// IsRoot dice si el proceso corre como root.
func IsRoot() bool { return os.Geteuid() == 0 }

// SudoUser devuelve el usuario que lanzó sudo, o "" si no hay sudo.
func SudoUser() string {
	user := os.Getenv("SUDO_USER")
	if user == "root" {
		return ""
	}
	return user
}
