package install

import (
	"fmt"
	"os"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── Las dependencias ─────────────────────────────────────────────────────────

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
