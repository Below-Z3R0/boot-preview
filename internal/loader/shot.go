package loader

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── shot ────────────────────────────────────────────────────────────────────

// minMenuWait es lo mínimo que se espera antes de dar el menú por bueno.
//
// OVMF dibuja su logo en el primer segundo. Limine tarda unos pocos más
// en tomar el control y pintar el menú. Sin esta espera mínima, un menú
// cuya resolución coincide con la inicial del firmware se confunde con
// el logo.
const minMenuWait = 3

// cmdShot captura el menú a un PNG.
func cmdShot(opts options, paths cli.ToolPaths) error {
	if err := checkRequirements(opts); err != nil {
		return err
	}

	conf := opts.conf
	cleanup := func() {}
	if opts.real {
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

	monitorSock := filepath.Join(workDir, "monitor.sock")
	_ = os.Remove(monitorSock)

	// Sin GUI no hace falta ningún display: el screendump va por el monitor.
	args := []string{
		"-enable-kvm", "-m", "512",
		"-cdrom", iso,
		"-monitor", "unix:" + monitorSock + ",server,nowait",
		"-boot", "d",
		"-display", "none",
	}
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
			"-boot", "d",
			"-display", "none",
		}
	}

	// El stderr se guarda: si QEMU arranca y muere (pflash inválido, OVMF
	// con tamaño raro), el error final decía "no se pudo capturar el
	// framebuffer" y no la causa.
	var qemuErr bytes.Buffer

	cmd := exec.Command("qemu-system-x86_64", args...)
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

	// En vez de esperar un tiempo fijo, se le pregunta al monitor si
	// Limine ya dibujó el menú.
	//
	// La señal es la resolución: OVMF arranca en una (1280x800) y Limine
	// la cambia a la interface_resolution al tomar el control. Se exige
	// además la espera mínima, porque si la resolución pedida coincide
	// con la del firmware, el primer frame (el logo de TianoCore) ya la
	// cumpliría.
	wantW, wantH, err := cli.ParseRes(opts.resolution)
	if err != nil {
		return err
	}

	fmt.Printf("Esperando a que arranque el menu (max %ds)...\n", opts.timeout)

	raw := filepath.Join(workDir, "shot.ppm")
	_ = os.Remove(raw)

	startedAt := nowSeconds()
	deadline := startedAt + int64(opts.timeout)
	stable := 0
	confirmed := false

	for {
		_ = screendump(monitorSock, raw)

		w, h := cli.ImageSize(raw)
		matched := w == wantW && h == wantH
		waited := nowSeconds()-startedAt >= minMenuWait

		if matched && waited {
			// Falta que termine de pintar (Limine dibuja el fondo y
			// después el texto): se pide que se mantenga dos frames.
			stable++
			if stable >= 2 {
				confirmed = true
				break
			}
			sleepMillis(400)
			continue
		}
		stable = 0

		if nowSeconds() >= deadline {
			break
		}
		sleepMillis(300)
	}

	if _, err := os.Stat(raw); err != nil {
		return fmt.Errorf("no se pudo capturar el framebuffer: %s", qemuDiagnostic(qemuErr.String()))
	}
	if !confirmed {
		cli.Warn("no se pudo confirmar que sea el menú (%dx%d no apareció); se capturó lo que había", wantW, wantH)
	}

	// Sin nombre: va a <repo>/previews/loader/ con timestamp.
	out := opts.shotPath
	if out == "" {
		dir := paths.LoaderPreviews()
		if dir == "" {
			dir = workDir
		}
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, "loader-"+timestamp()+".png")
	} else if !filepath.IsAbs(out) && paths.Repo != "" {
		dir := paths.LoaderPreviews()
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, filepath.Base(out))
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}

	// vips es ~6x más rápido que ImageMagick para esto.
	if err := convertToPNG(raw, out); err != nil {
		return err
	}
	fmt.Printf("Captura: %s\n", out)
	return nil
}

// qemuDiagnostic arma un mensaje con lo que dijo QEMU.
func qemuDiagnostic(stderr string) string {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		return "no dijo nada"
	}
	// Solo la primera línea: alcanza para saber qué pasó y no llena la
	// terminal con el trace completo.
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	return msg
}

// screendump le pide un volcado del framebuffer al monitor de QEMU.
func screendump(monitorSock, dest string) error {
	cmd := exec.Command("socat", "-", "UNIX-CONNECT:"+monitorSock)
	cmd.Stdin = strings.NewReader("screendump " + dest + "\n")
	return cmd.Run()
}

// convertToPNG convierte el PPM a PNG.
//
// Si no hay con qué convertir, falla: copiar el PPM a un archivo con
// nombre .png produce un archivo que ningún lector acepta, y el error
// aparecía mucho después, en quien lo abría.
func convertToPNG(src, dest string) error {
	if cli.HaveCommand("vips") {
		if _, err := cli.RunCombined("vips", "copy", src, dest); err == nil {
			return nil
		}
	}
	if cli.HaveCommand("magick") {
		if _, err := cli.RunCombined("magick", src, dest); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no se pudo convertir el PPM a PNG: instalá vips o imagemagick")
}
