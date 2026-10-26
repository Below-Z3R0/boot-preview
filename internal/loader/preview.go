package loader

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

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
		cli.SleepMillis(200)
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
