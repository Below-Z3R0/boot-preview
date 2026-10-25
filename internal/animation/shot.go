package animation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── shot ─────────────────────────────────────────────────────────────────────

// ── shot ────────────────────────────────────────────────────────────────────

// cmdShot captura el splash a un PNG.
func cmdShot(args []string, paths cli.ToolPaths) error {
	out := ""
	if len(args) > 0 {
		out = args[0]
	}

	theme := previewTheme(paths)
	if _, err := os.Stat(filepath.Join(themesDir, theme)); err != nil {
		return fmt.Errorf("no existe el tema: %s", theme)
	}

	if os.Getenv("DISPLAY") == "" {
		cli.Warn("No hay DISPLAY. X11 necesita un servidor gráfico.")
		return fmt.Errorf("")
	}
	if !cli.HaveCommand("import") {
		cli.Warn("Falta ImageMagick (import).")
		return fmt.Errorf("")
	}
	if !x11RootAccess() {
		cli.Warn("root no puede abrir una ventana en %s.", os.Getenv("DISPLAY"))
		cli.Say("Solución: boot-preview animation access")
		return fmt.Errorf("")
	}

	// Si ya hay un daemon corriendo (el usuario dejó la ventana
	// abierta), se reutiliza: levantar otro mataría al primero y la
	// ventana desaparecería.
	reusing := daemonRunning()

	var proc *plymouthProcess
	if reusing {
		cli.Step("Reutilizando el daemon que ya está corriendo")
	} else {
		var err error
		proc, err = startPlymouthX11(theme, paths)
		if err != nil {
			return err
		}
		// startPlymouthX11 solo PREPARA el comando: el arranque (y el
		// show-splash) los hace start().
		if err := proc.start(false); err != nil {
			return err
		}
		defer proc.cleanup()
	}

	// La ventana de Plymouth se llama "plymouthd".
	//
	// El daemon abre la ventana GTK un poco después de arrancar: hay que
	// esperarla, no buscar una sola vez.
	wid := waitForPlymouthWindow()
	if wid == "" {
		cli.Warn("No encontré la ventana de Plymouth.")
		printLogTail(x11Log)
		return fmt.Errorf("")
	}

	cli.Step("Capturando (ventana %s)", wid)

	tmpDir, err := os.MkdirTemp("", "plymouth-shot.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	bestSize := 0
	bestFrame := ""
	var candidates []string

	// 8 muestras cada 500 ms: 4 s de animación.
	for i := 1; i <= 8; i++ {
		sleep(500 * time.Millisecond)
		png := filepath.Join(tmpDir, fmt.Sprintf("shot-%d.png", i))
		if err := captureWindow(wid, png); err != nil {
			continue
		}
		info, err := os.Stat(png)
		if err != nil || info.Size() == 0 {
			continue
		}
		// Un frame negro pesa ~2 KB; uno con contenido, 200 KB o más.
		size := int(info.Size())
		if size > 20000 {
			candidates = append(candidates, png)
		}
		if size > bestSize {
			bestSize = size
			bestFrame = png
		}
	}

	if bestFrame == "" || bestSize < 20000 {
		return fmt.Errorf("no se pudo capturar el splash (todas las frames salieron vacías)")
	}

	if out == "" {
		dir := paths.AnimationPreviews()
		if dir == "" {
			dir = paths.AnimationConfig()
		}
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, "animation-"+timeStamp()+".png")
	} else if !filepath.IsAbs(out) && paths.Repo != "" {
		dir := paths.AnimationPreviews()
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, filepath.Base(out))
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := cli.CopyFile(bestFrame, out); err != nil {
		return err
	}

	fmt.Printf("Captura: %s  (%s)\n", out, cli.FileSize(out))

	// Contact sheet con 4 frames DISTRIBUIDOS en el tiempo: así se ve cómo
	// evoluciona la animación, no solo el frame más pesado.
	if len(candidates) >= 4 && cli.HaveCommand("magick") {
		sheet := strings.TrimSuffix(out, ".png") + "-sheet.png"
		pick := spread(candidates, 4)
		args := append(append([]string{}, pick...), "-resize", "640x360", "+append", "-append", sheet)
		if _, err := cli.RunCombined("magick", args...); err == nil {
			fmt.Printf("Sheet:   %s  (%s)\n", sheet, cli.FileSize(sheet))
		}
	}

	return nil
}

// findPlymouthWindow busca la ventana de Plymouth por su nombre.

// waitForPlymouthWindow espera a que aparezca la ventana de Plymouth.
//
// El daemon arranca y tarda un poco en abrir la ventana GTK: si se busca
// una sola vez, justo después de arrancar, puede que no esté todavía.
func waitForPlymouthWindow() string {
	for i := 0; i < 20; i++ {
		if wid := findPlymouthWindow(); wid != "" {
			return wid
		}
		sleep(250 * time.Millisecond)
	}
	return ""
}

func findPlymouthWindow() string {
	// DISPLAY se pasa explícitamente: xdotool no lo busca por su cuenta y
	// si el proceso que lo invoca no lo tiene en su entorno, busca en :0.
	cmd := exec.Command("xdotool", "search", "--name", "^plymouthd$")
	cmd.Env = append(os.Environ(), "DISPLAY="+os.Getenv("DISPLAY"))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if id := strings.TrimSpace(line); id != "" {
			return id
		}
	}
	return ""
}

// captureWindow captura una ventana X11 a un PNG.
func captureWindow(wid, dest string) error {
	cmd := exec.Command("import", "-window", wid, dest)
	cmd.Env = append(os.Environ(), "DISPLAY="+os.Getenv("DISPLAY"))
	return cmd.Run()
}

// spread toma n elementos distribuidos a lo largo de la lista.
func spread(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	var picked []string
	for i := 0; i < n; i++ {
		pos := i * (len(list) - 1) / (n - 1)
		picked = append(picked, list[pos])
	}
	return picked
}
