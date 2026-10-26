package loader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── El ISO del preview ──────────────────────────────────────────────────────
//
// Limine no se puede previsualizar sin arrancarlo: el preview arma un ISO
// híbrido con la config y el wallpaper, y lo bootea en QEMU. Acá está el
// armado del ISO; la ventana y los chequeos están en preview.go.

// buildISO arma el árbol del ISO y lo construye.
func buildISO(conf string, opts options) (string, error) {
	if err := os.RemoveAll(workDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(workDir, "EFI", "BOOT"), 0o755); err != nil {
		return "", err
	}

	for _, file := range []string{"limine-uefi-cd.bin", "limine-bios-cd.bin", "limine-bios.sys"} {
		if err := cli.CopyFile(filepath.Join(limineDir, file), filepath.Join(workDir, file)); err != nil {
			return "", fmt.Errorf("falta %s en %s", file, limineDir)
		}
	}
	if err := cli.CopyFile(filepath.Join(limineDir, "BOOTX64.EFI"), filepath.Join(workDir, "EFI", "BOOT", "BOOTX64.EFI")); err != nil {
		return "", fmt.Errorf("falta BOOTX64.EFI en %s", limineDir)
	}

	// La config: se copia tal cual y se le inyecta el wallpaper y la
	// resolución.
	target := filepath.Join(workDir, "limine.conf")
	if err := cli.CopyFile(conf, target); err != nil {
		return "", err
	}
	if err := injectPreviewOptions(target, opts.resolution); err != nil {
		return "", err
	}

	if err := cli.CopyFile(opts.wallpaper, filepath.Join(workDir, "wallpaper.png")); err != nil {
		return "", err
	}

	// El ISO híbrido (UEFI + BIOS).
	iso := filepath.Join(workDir, "boot-preview-loader.iso")
	xorriso := exec.Command("xorriso", "-as", "mkisofs", "-R", "-r", "-J",
		"-b", "limine-bios-cd.bin",
		"-no-emul-boot", "-boot-load-size", "4", "-boot-info-table", "-hfsplus",
		"-apm-block-size", "2048", "--efi-boot", "limine-uefi-cd.bin",
		"-efi-boot-part", "--efi-boot-image", "--protective-msdos-label",
		".", "-o", iso)
	xorriso.Dir = workDir
	if out, err := xorriso.CombinedOutput(); err != nil {
		return "", fmt.Errorf("xorriso falló: %s", strings.TrimSpace(string(out)))
	}

	// La receta oficial de Limine termina con `limine bios-install`: sin
	// eso, la mitad BIOS del ISO híbrido queda sin instalar (el camino
	// UEFI, que es el que usa el preview, funciona igual).
	if bin := cli.CommandPath("limine"); bin != "" {
		bios := exec.Command(bin, "bios-install", iso)
		if out, err := bios.CombinedOutput(); err != nil {
			// No es fatal: el preview arranca por UEFI.
			cli.Warn("limine bios-install falló: %s", qemuDiagnostic(string(out)))
		}
	} else {
		cli.Warn("no encontré limine: el ISO no sirve para arrancar por BIOS (por UEFI sí)")
	}

	return iso, nil
}
