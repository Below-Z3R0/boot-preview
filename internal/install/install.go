// Comando install: deja el tool instalado y listo para usar.
//
// Se puede instalar todo o solo una de las partes:
//
//	boot-preview install            # todo
//	boot-preview install loader     # solo el módulo de Limine
//	boot-preview install animation  # solo el módulo de Plymouth
//	boot-preview install --no-deps  # sin tocar paquetes del sistema
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// deps son los paquetes del sistema que necesita cada módulo.
var (
	loaderDeps = []string{
		"qemu-desktop", // el emulador
		"libisoburn",   // xorriso: arma el ISO
		"imagemagick",  // convierte la captura
		"socat",        // habla con el monitor de QEMU
	}
	animationDeps = []string{
		"ffmpeg",     // convierte los videos de Steam en temas
		"xdotool",    // encuentra la ventana de Plymouth
		"xorg-xhost", // da acceso de root al servidor X
	}
)

// watched son los paquetes de los que se guarda la versión, para detectar
// que el sistema se actualizó.
var watched = []string{
	"qemu-desktop", "libisoburn", "edk2-ovmf", "imagemagick", "socat",
	"libvips", "ffmpeg", "xdotool", "xorg-xhost",
	"plymouth", "limine", "xorg-server-xvfb",
}

// state es el snapshot que se guarda al instalar.
type state struct {
	Version    string            `json:"version"`
	UpdatedAt  string            `json:"updated_at"`
	Components []string          `json:"components"`
	Deps       map[string]string `json:"deps"`
	Repo       string            `json:"repo"`
}

// Run ejecuta el comando install.
func Run(args []string, paths cli.Paths) error {
	what := "all"
	withDeps := true

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "loader", "animation", "all":
			what = args[i]
		case "--no-deps":
			withDeps = false
		case "--deps":
			withDeps = true
		case "-h", "--help":
			usage()
			return nil
		default:
			return fmt.Errorf("opción desconocida: %s", args[i])
		}
	}

	wantLoader := what == "all" || what == "loader"
	wantAnimation := what == "all" || what == "animation"

	if withDeps {
		if err := installDeps(wantLoader, wantAnimation); err != nil {
			return err
		}
	} else {
		cli.Step("1/4  Dependencias")
		cli.Ok("omitidas (--no-deps)")
	}

	if err := installBinaries(paths, wantLoader, wantAnimation); err != nil {
		return err
	}
	if err := installConfig(paths, wantLoader, wantAnimation); err != nil {
		return err
	}
	if err := writeState(paths, what); err != nil {
		return err
	}

	fmt.Printf("\n\033[1;38;2;125;207;216m✓ Listo.\033[0m\n\n")
	printUsage(what)
	fmt.Println()
	cli.Say("Verificá la instalación con:  boot-preview verify")
	return nil
}

// installDeps instala los paquetes que falten.
func installDeps(wantLoader, wantAnimation bool) error {
	cli.Step("1/4  Dependencias")

	var missing []string
	for _, pkg := range depsFor(wantLoader, wantAnimation) {
		if cli.PackageVersion(pkg) == "" {
			missing = append(missing, pkg)
		}
	}

	if len(missing) > 0 {
		cli.Say("Faltan: %s", strings.Join(missing, " "))
		cli.Say("Instalando con sudo (puede pedir contraseña)...")
		args := append([]string{"pacman", "-S", "--needed", "--noconfirm"}, missing...)
		if out, err := cli.RunCombined("sudo", args...); err != nil {
			return fmt.Errorf("no se pudieron instalar: %s", out)
		}
		cli.Ok("instalados: %s", strings.Join(missing, " "))
	} else {
		cli.Ok("todo instalado")
	}

	if wantLoader {
		// OVMF: sin el, el menú arranca en modo texto y no se ve el wallpaper.
		if _, err := os.Stat("/usr/share/edk2/x64/OVMF_CODE.4m.fd"); err != nil {
			cli.Say("Falta el firmware UEFI (edk2-ovmf)")
			if out, err := cli.RunCombined("sudo", "pacman", "-S", "--needed", "--noconfirm", "edk2-ovmf"); err != nil {
				return fmt.Errorf("no se pudo instalar edk2-ovmf: %s", out)
			}
			cli.Ok("edk2-ovmf instalado")
		} else {
			cli.Ok("OVMF (UEFI) presente")
		}

		if _, err := os.Stat("/dev/kvm"); err == nil {
			cli.Ok("/dev/kvm presente — la VM arranca en segundos")
		} else {
			cli.Warn("sin /dev/kvm: la VM va a ir por emulación (lenta)")
		}

		if cli.HaveCommand("vips") {
			cli.Ok("vips (libvips) presente — wallpapers ~6x mas rapidos")
		}
	}

	if wantAnimation {
		// El preview de Plymouth necesita el renderer X11: Plymouth ya lo
		// trae compilado (paquete plymouth), no hay nada que instalar.
		if _, err := os.Stat("/usr/lib/plymouth/renderers/x11.so"); err == nil {
			cli.Ok("renderer X11 de Plymouth presente (sin VM)")
		} else {
			cli.Warn("sin renderers/x11.so: el preview de Plymouth no va a funcionar")
		}
	}

	return nil
}

// depsFor devuelve los paquetes que necesita cada módulo.
func depsFor(wantLoader, wantAnimation bool) []string {
	var pkgs []string
	if wantLoader {
		pkgs = append(pkgs, loaderDeps...)
	}
	if wantAnimation {
		pkgs = append(pkgs, animationDeps...)
	}
	return pkgs
}

// installBinaries copia el binario y los recursos a sus lugares.
func installBinaries(paths cli.Paths, wantLoader, wantAnimation bool) error {
	cli.Step("2/4  Los scripts")

	if err := os.MkdirAll(paths.Bin, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.Lib, 0o755); err != nil {
		return err
	}

	// El dispatcher: un solo comando, `boot-preview`.
	if err := installBinary(paths, "boot-preview", filepath.Join(paths.Bin, "boot-preview")); err != nil {
		return err
	}
	cli.Ok("%s", filepath.Join(paths.Bin, "boot-preview"))

	// Los recursos (los ejemplos que el instalador usa) van junto al
	// binario, en lib/.
	//
	// Si no hay repo (el tool se instaló desde el bundle), los assets ya
	// están instalados y no hay nada que copiar.
	assets := filepath.Join(paths.Lib, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return err
	}
	if paths.Repo != "" {
		if err := cli.CopyDir(filepath.Join(paths.Repo, "examples"), assets); err != nil {
			return err
		}
		cli.Ok("%s", assets)
	} else {
		cli.Ok("%s (ya instalados)", assets)
	}

	// Un shim en /usr/local/bin para que `sudo boot-preview` funcione:
	// ~/.local/bin no está en el secure_path de sudo.
	return installSudoShim(paths)
}

// installBinary copia el binario compilado a su destino.
func installBinary(paths cli.Paths, name, dest string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	// Si ya estamos instalados, el ejecutable actual es el bueno.
	if err := cli.CopyFile(self, dest); err != nil {
		return err
	}
	return os.Chmod(dest, 0o755)
}

// installSudoShim deja un wrapper en /usr/local/bin.
func installSudoShim(paths cli.Paths) error {
	const shim = `#!/usr/bin/env bash
# Wrapper de boot-preview para poder correrlo con ` + "`sudo`" + `.
# ` + "`sudo boot-preview`" + ` falla con "command not found" porque ~/.local/bin
# no está en el secure_path de sudo. Si lo invoca root, baja al usuario que
# lo lanzó: QEMU no debe correr como root (no tiene acceso al socket gráfico).
set -euo pipefail
script="${HOME}/.local/bin/boot-preview"
if [[ ! -x "$script" ]]; then
  script="/home/b3low-z3r0/.local/bin/boot-preview"
fi
if [[ "$(id -u)" == "0" && -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
  exec sudo -u "$SUDO_USER" "$script" "$@"
fi
exec "$script" "$@"
`

	tmp, err := os.CreateTemp("", "boot-preview-shim.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(shim); err != nil {
		return err
	}
	tmp.Close()

	dest := "/usr/local/bin/boot-preview"
	if out, err := cli.RunCombined("sudo", "install", "-m", "755", tmp.Name(), dest); err != nil {
		cli.Warn("no se pudo instalar el shim en /usr/local/bin (sudo?)")
		cli.Say("%s", out)
		return nil
	}
	cli.Ok("%s (shim para sudo)", dest)
	return nil
}

// installConfig crea la configuración de lo instalado.
func installConfig(paths cli.Paths, wantLoader, wantAnimation bool) error {
	cli.Step("3/4  La config")

	if wantLoader {
		loaderDir := paths.LoaderConfig()
		if err := os.MkdirAll(filepath.Join(loaderDir, "wallpaper"), 0o755); err != nil {
			return err
		}

		// La config de prueba: se instala desde el asset, pero si el
		// usuario ya tenía una se guarda como .bak (no se pisa).
		conf := filepath.Join(loaderDir, "limine.conf")
		if _, err := os.Stat(conf); err == nil {
			_ = cli.CopyFile(conf, conf+".bak")
			cli.Say("hay una config previa: guardada como limine.conf.bak")
		}
		src := filepath.Join(paths.Lib, "assets", "loader.conf")
		if err := cli.CopyFile(src, conf); err != nil {
			return err
		}
		cli.Ok("%s", conf)

		// El wallpaper de ejemplo.
		rei := filepath.Join(loaderDir, "wallpaper", "rei.png")
		if _, err := os.Stat(rei); err != nil {
			if err := cli.CopyFile(filepath.Join(paths.Lib, "assets", "wallpaper", "rei.png"), rei); err != nil {
				return err
			}
			cli.Ok("%s", rei)
		} else {
			cli.Ok("%s (ya estaba)", rei)
		}

		// La ACTIVA: la que usan el preview y `apply`. Si no existe, se
		// copia la nítida; la difuminada la genera `loader wallpapers`.
		active := filepath.Join(loaderDir, "wallpaper", "wallpaper.png")
		if _, err := os.Stat(active); err != nil {
			if err := cli.CopyFile(rei, active); err != nil {
				return err
			}
			cli.Ok("%s (la activa, copiada de rei.png)", active)
		} else {
			cli.Ok("%s (la activa, ya estaba)", active)
		}
	}

	if wantAnimation {
		if err := os.MkdirAll(paths.AnimationConfig(), 0o755); err != nil {
			return err
		}
		cli.Ok("%s", paths.AnimationConfig())
	}

	// La ruta del repo: la usa el CLI para saber dónde van las capturas.
	//
	// Si no hay repo (instalación desde el bundle), las capturas van a
	// ~/.local/share/boot-preview/previews.
	repoFile := filepath.Join(paths.Config, "repo")
	repo := paths.Repo
	if repo == "" {
		repo = paths.PreviewsRoot()
	}
	if err := os.WriteFile(repoFile, []byte(repo+"\n"), 0o644); err != nil {
		return err
	}
	cli.Ok("%s -> %s", repoFile, repo)

	// Las carpetas de capturas.
	for _, dir := range []string{paths.LoaderPreviews(), paths.AnimationPreviews()} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	cli.Ok("%s", filepath.Join(repo, "previews", "{loader,animation}"))

	return nil
}

// writeState guarda el snapshot de versiones de las dependencias.
func writeState(paths cli.Paths, what string) error {
	cli.Step("4/4  Estado")

	deps := map[string]string{}
	for _, pkg := range watched {
		if v := cli.PackageVersion(pkg); v != "" {
			deps[pkg] = v
		}
	}

	st := state{
		Version:    Version(),
		UpdatedAt:  today(),
		Components: []string{what},
		Deps:       deps,
		Repo:       paths.Repo,
	}
	if what == "all" {
		st.Components = []string{"animation", "loader"}
	}

	// Se conserva lo que ya había: el snapshot es acumulativo.
	if data, err := os.ReadFile(filepath.Join(paths.Config, "state.json")); err == nil {
		var old state
		if json.Unmarshal(data, &old) == nil {
			for _, c := range old.Components {
				if !contains(st.Components, c) {
					st.Components = append(st.Components, c)
				}
			}
			if old.Repo != "" {
				st.Repo = old.Repo
			}
		}
	}
	sortStrings(st.Components)

	encoded, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	dest := filepath.Join(paths.Config, "state.json")
	if err := os.WriteFile(dest, encoded, 0o644); err != nil {
		return err
	}
	cli.Ok("%s (%d dependencias registradas)", dest, len(deps))
	return nil
}

// ── Utilidades ──────────────────────────────────────────────────────────────

// copyFile copia un archivo, preservando los permisos.

// copyDir copia un directorio entero.

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func sortStrings(list []string) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

func today() string {
	out, _ := cli.Run("date", "+%Y-%m-%d")
	return out
}

// Version devuelve la versión del tool. Se inyecta en el build con -ldflags.
var Version = func() string { return "dev" }

func usage() {
	fmt.Println(`install — instala boot-preview

  install                  Instala todo (loader + animation)
  install loader           Solo el módulo de Limine
  install animation        Solo el módulo de Plymouth
  install --no-deps        No instala paquetes del sistema`)
}

// printUsage muestra los comandos de lo recién instalado.
func printUsage(what string) {
	switch what {
	case "loader":
		fmt.Println(`  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader real         # con tu config real de /boot/limine.conf
      boot-preview loader shot         # captura a PNG
      boot-preview loader add ~/Pictures/foto.png
      boot-preview loader edit         # abrir la config en tu editor
      boot-preview loader apply        # aplicar al Limine real
      boot-preview loader wallpapers   # las dos versiones del wallpaper`)
	case "animation":
		fmt.Println(`  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation sources   # los temas instalados
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)`)
	default:
		fmt.Println(`  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader shot         # captura a PNG
      boot-preview loader apply        # aplicar al Limine real

  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)`)
	}
}
