package loader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── shot ────────────────────────────────────────────────────────────────────

// cmdShot captura el menú a un PNG.
func cmdShot(opts options, paths cli.Paths) error {
	if err := checkRequirements(opts); err != nil {
		return err
	}

	conf := opts.conf
	if opts.real {
		out, err := cli.RunCombined("sudo", "cat", "/boot/limine.conf")
		if err != nil {
			return fmt.Errorf("no se pudo leer /boot/limine.conf (hace falta sudo)")
		}
		tmp, err := os.CreateTemp("", "boot-preview-loader-real.*.conf")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		tmp.Close()
		if err := os.WriteFile(tmp.Name(), []byte(out), 0o644); err != nil {
			return err
		}
		conf = tmp.Name()
	}

	iso, err := buildISO(conf, opts)
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

	cmd := exec.Command("qemu-system-x86_64", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo arrancar QEMU: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Espera a que el monitor responda.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(monitorSock); err == nil {
			break
		}
		sleepMillis(200)
	}

	// En vez de esperar un tiempo fijo, se le pregunta al monitor si
	// Limine ya dibujó el menú. Ahorra ~10 s de los 15 que costaba antes.
	//
	// La señal NO es "hay algo pintado": el logo de TianoCore (OVMF) ya
	// cuenta como pintado, y se terminaba capturando el logo en vez del
	// menú. La señal buena es el CAMBIO DE RESOLUCIÓN: OVMF arranca en
	// 1280x800 y Limine la cambia a la interface_resolution al tomar el
	// control.
	fmt.Printf("Esperando a que arranque el menu (max %ds)...\n", opts.timeout)

	raw := filepath.Join(workDir, "shot.ppm")
	_ = os.Remove(raw)

	deadline := nowSeconds() + int64(opts.timeout)
	settled := false

	for {
		_ = screendump(monitorSock, raw)

		if size, ok := imageSize(raw); ok && size == opts.resolution {
			// Ya es la resolución del menú: falta que termine de pintar
			// (Limine dibuja el fondo y después el texto).
			if settled {
				break
			}
			settled = true
			sleepMillis(500)
			continue
		}

		if nowSeconds() >= deadline {
			fmt.Println("  se agoto el tiempo de espera, capturando lo que haya")
			break
		}
		sleepMillis(300)
	}

	if _, err := os.Stat(raw); err != nil {
		return fmt.Errorf("no se pudo capturar el framebuffer")
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

// screendump le pide un volcado del framebuffer al monitor de QEMU.
func screendump(monitorSock, dest string) error {
	cmd := exec.Command("socat", "-", "UNIX-CONNECT:"+monitorSock)
	cmd.Stdin = strings.NewReader("screendump " + dest + "\n")
	return cmd.Run()
}

// convertToPNG convierte el PPM a PNG.
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
	return cli.CopyFile(src, dest)
}

// imageSize devuelve el tamaño de una imagen como "WxH".
func imageSize(path string) (string, bool) {
	if cli.HaveCommand("vipsheader") {
		w, errW := cli.Run("vipsheader", "-f", "width", path)
		h, errH := cli.Run("vipsheader", "-f", "height", path)
		if errW == nil && errH == nil {
			return w + "x" + h, true
		}
	}
	if cli.HaveCommand("magick") {
		out, err := cli.Run("magick", "identify", "-format", "%wx%h", path)
		if err == nil {
			return out, true
		}
	}
	return "", false
}
