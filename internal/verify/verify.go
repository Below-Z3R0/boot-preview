// Comando verify: comprueba que la instalación esté en su sitio.
//
// Además compara las versiones de las dependencias con el snapshot que
// dejó install: si algo del sistema se actualizó, lo avisa. El preview
// usa el software del sistema, así que ya está usando la versión nueva:
// el aviso es informativo.
package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// state es el snapshot que escribe install.
type state struct {
	Version    string            `json:"version"`
	UpdatedAt  string            `json:"updated_at"`
	Components []string          `json:"components"`
	Deps       map[string]string `json:"deps"`
	Repo       string            `json:"repo"`
}

// Run ejecuta el comando verify.
func Run(paths cli.Paths) error {
	failed := 0
	warnings := 0

	cli.Step("Verificando la instalación")

	// La versión instalada.
	stateFile := filepath.Join(paths.Config, "state.json")
	st, hasState := readState(stateFile)
	if hasState {
		fmt.Printf("  Versión instalada: %s\n\n", st.Version)
	}

	// ── Los módulos ──
	//
	// El tool es UN solo binario: no hay módulos sueltos que instalar.
	fmt.Println("  Binario")
	if _, err := os.Stat(filepath.Join(paths.Bin, "boot-preview")); err == nil {
		row(true, "boot-preview", filepath.Join(paths.Bin, "boot-preview"))
	} else {
		row(false, "boot-preview", "no instalado")
		failed++
	}

	// ── Las dependencias ──
	fmt.Println()
	fmt.Println("  Dependencias de Limine")
	for _, bin := range []string{"qemu-system-x86_64", "xorriso", "socat", "magick"} {
		if path := cli.CommandPath(bin); path != "" {
			row(true, bin, path)
		} else {
			row(false, bin, "falta (boot-preview install loader)")
			failed++
		}
	}
	if _, err := os.Stat("/usr/share/edk2/x64/OVMF_CODE.4m.fd"); err == nil {
		row(true, "edk2-ovmf", "presente")
	} else {
		row(false, "edk2-ovmf", "falta (boot-preview install loader)")
		failed++
	}
	if _, err := os.Stat("/dev/kvm"); err == nil {
		row(true, "/dev/kvm", "presente")
	} else {
		rowWarn("/dev/kvm", "sin KVM: la VM va por emulación (lenta)")
		warnings++
	}

	fmt.Println()
	fmt.Println("  Dependencias de Plymouth")
	for _, bin := range []string{"ffmpeg", "xdotool", "xhost"} {
		if path := cli.CommandPath(bin); path != "" {
			row(true, bin, path)
		} else {
			row(false, bin, "no instalado")
		}
	}
	if _, err := os.Stat("/usr/lib/plymouth/renderers/x11.so"); err == nil {
		row(true, "renderers/x11.so", "presente")
	} else {
		row(false, "renderers/x11.so", "falta (reinstalá plymouth)")
		failed++
	}

	// ── La config ──
	fmt.Println()
	fmt.Println("  Config")
	if _, err := os.Stat(filepath.Join(paths.LoaderConfig(), "limine.conf")); err == nil {
		row(true, "loader/limine.conf", "ok")
	} else {
		row(false, "loader/limine.conf", "no instalado")
	}
	if _, err := os.Stat(filepath.Join(paths.LoaderConfig(), "wallpaper", "wallpaper.png")); err == nil {
		row(true, "loader/wallpaper", "ok")
	} else {
		row(false, "loader/wallpaper", "no instalado")
	}
	themeFile := filepath.Join(paths.AnimationConfig(), "theme")
	if data, err := os.ReadFile(themeFile); err == nil {
		row(true, "animation/theme", strings.TrimSpace(string(data)))
	} else {
		rowOptional("animation/theme", "sin tema propio (usa el del sistema)")
	}

	// ── Cambios desde la instalación ──
	if hasState && len(st.Deps) > 0 {
		fmt.Println()
		fmt.Println("  Cambios desde la instalación")

		changes := changedPackages(st.Deps)
		if len(changes) == 0 {
			row(true, "ninguno", "")
		} else {
			for _, c := range changes {
				rowWarn(c.pkg, fmt.Sprintf("%s → %s", c.old, c.new))
			}
			fmt.Println()
			cli.Say("El preview usa el software del sistema, así que ya está")
			cli.Say("usando la versión nueva.")
			warnings++
		}
	}

	fmt.Println()
	if failed == 0 {
		fmt.Printf("\033[1;38;2;125;207;216m✓ Listo.\033[0m")
		if warnings > 0 {
			fmt.Printf(" (%d avisos)\n", warnings)
		} else {
			fmt.Println()
		}
		return nil
	}

	fmt.Printf("\033[1;38;2;246;153;150m✗ Faltan cosas.\033[0m\n")
	cli.Say("Corré: boot-preview install [loader|animation]")
	return fmt.Errorf("")
}

// change es un paquete que cambió de versión.
type change struct {
	pkg string
	old string
	new string
}

// changedPackages compara el snapshot con el estado actual.
func changedPackages(deps map[string]string) []change {
	var changes []change
	for pkg, oldVersion := range deps {
		newVersion := cli.PackageVersion(pkg)
		if newVersion == "" || newVersion == oldVersion {
			continue
		}
		changes = append(changes, change{pkg: pkg, old: oldVersion, new: newVersion})
	}
	return changes
}

// readState lee el snapshot de la instalación.
func readState(path string) (state, bool) {
	var st state
	data, err := os.ReadFile(path)
	if err != nil {
		return st, false
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, false
	}
	return st, true
}

// ── Salida ──────────────────────────────────────────────────────────────────

const (
	colorOK   = "\033[38;2;125;207;216m"
	colorWarn = "\033[38;2;246;153;150m"
	colorDim  = "\033[38;2;146;136;158m"
	colorOff  = "\033[0m"
)

// row imprime una línea de la tabla de verificación.
func row(ok bool, name, value string) {
	mark := colorOK + "✓" + colorOff
	if !ok {
		mark = colorWarn + "✗" + colorOff
	}
	fmt.Printf("    %s %-22s %s\n", mark, name, value)
}

// rowOptional es una fila que no es ni un ok ni un fallo: algo que se
// puede dejar sin configurar.
//
// El tema del preview es el caso: si no hay ninguno, el preview usa el
// tema que Plymouth tiene configurado en el sistema, que funciona.
func rowOptional(name, value string) {
	fmt.Printf("    %s %-22s %s\n", colorDim+"·"+colorOff, name, value)
}

// rowWarn imprime una línea de aviso (no es un fallo).
func rowWarn(name, value string) {
	fmt.Printf("    %s!%s %-22s %s\n", colorWarn, colorOff, name, value)
}
