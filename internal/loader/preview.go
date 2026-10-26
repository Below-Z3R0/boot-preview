package loader

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// injectComment es el comentario que marca el bloque que inyecta
// boot-preview. Se borra antes de reinyectar, para que el apply sea
// idempotente.
const injectComment = "# ── inyectado por boot-preview loader ──"

// ── El preview ──────────────────────────────────────────────────────────────

// cmdPreview abre el menú en una ventana.
func cmdPreview(opts options, paths cli.ToolPaths) error {
	if err := checkRequirements(opts); err != nil {
		return err
	}

	conf := opts.conf
	cleanup := func() {}
	if opts.real {
		// OJO: `sudo cat ... > archivo` y no `sudo cp ... archivo`. Con
		// cp el archivo queda con el modo de /boot/limine.conf
		// (-rwx------ root) y el usuario no lo puede leer en el paso
		// siguiente.
		tmp, err := os.CreateTemp("", "boot-preview-loader-real.*.conf")
		if err != nil {
			return err
		}
		tmp.Close()
		out, err := cli.RunCombined("sudo", "cat", "/boot/limine.conf")
		if err != nil {
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("no se pudo leer /boot/limine.conf (hace falta sudo): %s", out)
		}
		if err := os.WriteFile(tmp.Name(), []byte(out), 0o644); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
		conf = tmp.Name()
		cleanup = func() { _ = os.Remove(tmp.Name()) }
	}

	iso, err := buildISO(conf, opts)
	cleanup()
	if err != nil {
		return err
	}
	fmt.Printf("ISO listo: %s\n", iso)

	monitorSock := filepath.Join(workDir, "monitor.sock")
	qmpSock := filepath.Join(workDir, "qmp.sock")
	_ = os.Remove(monitorSock)
	_ = os.Remove(qmpSock)

	args := []string{
		"-enable-kvm", "-m", "512",
		"-cdrom", iso,
		"-monitor", "unix:" + monitorSock + ",server,nowait",
		"-qmp", "unix:" + qmpSock + ",server,nowait",
		"-boot", "d",
	}

	// UEFI (OVMF) — es lo que usa la máquina real, y el menú gráfico con
	// wallpaper solo funciona bien bajo UEFI. Sin esto, QEMU arranca con
	// SeaBIOS y Limine cae a modo texto.
	if _, err := os.Stat(ovmfCode); err == nil {
		if err := cli.CopyFile(ovmfVars, filepath.Join(workDir, "OVMF_VARS.fd")); err != nil {
			return err
		}
		args = []string{
			"-enable-kvm", "-m", "512",
			"-drive", "if=pflash,format=raw,readonly=on,file=" + ovmfCode,
			"-drive", "if=pflash,format=raw,file=" + filepath.Join(workDir, "OVMF_VARS.fd"),
			"-cdrom", iso,
			"-monitor", "unix:" + monitorSock + ",server,nowait",
			"-qmp", "unix:" + qmpSock + ",server,nowait",
			"-boot", "d",
		}
	} else {
		cli.Warn("sin OVMF, arranca con SeaBIOS y el menú puede salir en modo texto")
	}

	// El display es SDL y no GTK: la ventana es NATIVA del sistema, no un
	// GTK embebido. En Wayland funciona sin Xwayland.
	//
	// El backend SDL no trae Ctrl+Q (era del backend GTK), así que se mapea
	// con QMP: send-key inyecta ctrl+q en el QEMU y el quit de QMP lo
	// cierra.
	args = append(args, "-display", "sdl")

	var qemuErr bytes.Buffer

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = nil
	cmd.Stderr = &qemuErr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo arrancar QEMU: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Espera a que el monitor responda.
	ready := false
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(monitorSock); err == nil {
			ready = true
			break
		}
		sleepMillis(200)
	}
	if !ready {
		return fmt.Errorf("QEMU no abrió el monitor: %s", qemuDiagnostic(qemuErr.String()))
	}

	go mapCtrlQ(qmpSock)

	fmt.Println()
	fmt.Println("QEMU corriendo. Para salir: Ctrl+Q (o cerrá la ventana).")
	fmt.Printf("Monitor QEMU en: %s\n", monitorSock)

	if err := cmd.Wait(); err != nil {
		// La ventana se cerró con Ctrl+Q o con la X: no es un fallo. Pero
		// si QEMU murió solo (pflash inválido, SDL ausente), el mensaje
		// tiene que decirlo.
		if msg := qemuDiagnostic(qemuErr.String()); msg != "no dijo nada" {
			return fmt.Errorf("QEMU terminó: %s", msg)
		}
	}
	return nil
}

// checkRequirements verifica que estén las dependencias y los archivos.
func checkRequirements(opts options) error {
	// socat es el que habla con el monitor de QEMU: sin él, el shot
	// quema todo el timeout y falla con "no se pudo capturar el
	// framebuffer", que no dice nada.
	for _, bin := range []string{"xorriso", "qemu-system-x86_64", "socat"} {
		if !cli.HaveCommand(bin) {
			return fmt.Errorf("falta: %s", bin)
		}
	}
	if _, err := os.Stat(opts.conf); err != nil {
		return fmt.Errorf("no existe la config: %s", opts.conf)
	}
	if _, err := os.Stat(opts.wallpaper); err != nil {
		return fmt.Errorf("no existe el wallpaper: %s", opts.wallpaper)
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		cli.Warn("sin /dev/kvm, la VM va a ir por emulación (lenta)")
	}
	return nil
}

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

// injectPreviewOptions mete las opciones del preview en el conf.
//
// OJO: en Limine las opciones GLOBALES tienen que ir antes de la primera
// entrada de menú (una línea que empieza con "/"). Si se inyectan al
// final, Limine las trata como opciones de esa entrada y las ignora.
func injectPreviewOptions(path, resolution string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Se borra el bloque inyectado en una corrida anterior (si lo hay),
	// para que la inyección sea idempotente: aplicarla dos veces tiene
	// que dejar el mismo archivo.
	lines := strings.Split(string(data), "\n")
	var kept []string
	skipping := false
	for _, line := range lines {
		if strings.TrimSpace(line) == injectComment {
			skipping = true
			continue
		}
		if skipping {
			// Se saltean las líneas en blanco que quedaron del bloque
			// anterior; la primera con contenido NO se descarta (puede ser
			// una entrada del menú).
			if strings.TrimSpace(line) == "" {
				continue
			}
			skipping = false
		}
		kept = append(kept, line)
	}
	text := strings.Join(kept, "\n")

	// Se borran las opciones de theming previas para que la config
	// inyectada gane.
	text = optionLineRe("wallpaper|wallpaper_style|interface_resolution").ReplaceAllString(text, "")
	// El timeout se fuerza a "no": si no, Limine autobootea la entrada
	// default (que en la config real apunta a un kernel con hash que no
	// está en el ISO) y se va de la pantalla antes de que puedas mirarla.
	text = optionLineRe("timeout").ReplaceAllString(text, "")

	inject := fmt.Sprintf(`%s
timeout: no
wallpaper: boot():/wallpaper.png
wallpaper_style: centered
interface_resolution: %s
`, injectComment, resolution)

	lines = strings.Split(text, "\n")
	insertAt := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "/") {
			insertAt = i
			break
		}
	}

	var out []string
	out = append(out, lines[:insertAt]...)
	out = append(out, strings.TrimRight(inject, "\n"))
	out = append(out, lines[insertAt:]...)

	result := strings.Join(out, "\n")
	// Las opciones borradas dejan lineas vacias: se colapsan para que el
	// archivo no crezca con cada inyeccion.
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return os.WriteFile(path, []byte(result+"\n"), 0o644)
}

// optionLineRe matchea una línea de opción completa.
func optionLineRe(names string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*(` + names + `)\s*:.*$\n?`)
}
