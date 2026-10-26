// El preview en X11.
//
// Plymouth trae un renderer X11 (renderers/x11.so) que dibuja en una
// ventana GTK sobre el escritorio. El daemon oficial levanta esa
// ventana: no es un reproductor de imágenes.
//
//	x11.go      el comando
//	daemon.go   el daemon: arrancarlo, detachment y pararlo
//	splash.go   hablar con el daemon (show, update)
package animation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── El preview en X11 ────────────────────────────────────────────────────────

// ── El preview en X11 ───────────────────────────────────────────────────────

// cmdX11 abre la ventana con el splash.
func cmdX11(args []string, paths cli.ToolPaths) error {
	// El daemon queda en background: el comando devuelve el control y la
	// ventana sigue viva. Se cierra con 'boot-preview animation stop'.
	//
	// Un límite de tiempo es opcional: con él, el preview se cierra solo.
	seconds := 0
	detach := true

	for _, arg := range args {
		switch {
		case arg == "--foreground", arg == "-f":
			detach = false
		default:
			if v, err := strconv.Atoi(arg); err == nil {
				seconds = v
			}
		}
	}

	theme := previewTheme(paths)
	if _, err := os.Stat(filepath.Join(themesDir, theme)); err != nil {
		return fmt.Errorf("no existe el tema: %s", theme)
	}

	if os.Getenv("DISPLAY") == "" {
		cli.Warn("No hay DISPLAY. X11 necesita un servidor gráfico.")
		cli.Say("En Wayland: export DISPLAY=:1 (Xwayland)")
		return fmt.Errorf("falta DISPLAY: no hay servidor gráfico al que dibujar")
	}

	cli.Step("Plymouth en X11 con el tema '%s'", theme)

	if ok, motivo := x11RootAccess(); !ok {
		cli.Warn("%s (DISPLAY=%s).", motivo, os.Getenv("DISPLAY"))
		fmt.Println()
		cli.Say("Si es el control de acceso de X, la solución (una vez por")
		cli.Say("sesión) es:")
		cli.Say("  boot-preview animation access")
		fmt.Println()
		cli.Say("Eso le da a root acceso al servidor X. Plymouth dibuja en una")
		cli.Say("ventana GTK sobre tu escritorio, con el tema real.")
		return fmt.Errorf("root no tiene acceso a %s: corré 'boot-preview animation access'", os.Getenv("DISPLAY"))
	}

	proc, err := startPlymouthX11(theme, paths)
	if err != nil {
		return err
	}

	// En background: el daemon se queda vivo después de que el comando
	// termine. En foreground, el comando espera a que termine.
	if !detach {
		defer proc.cleanup()
		if err := proc.start(false); err != nil {
			return err
		}

		cli.Say("")
		cli.Say("Ventana abierta con el tema '%s'.", theme)
		cli.Say("Ctrl+C para cerrar.")

		if seconds == 0 {
			proc.wait()
			return nil
		}
		for i := 0; i < seconds; i++ {
			if !proc.alive() {
				break
			}
			time.Sleep(time.Second)
			updateSplash("Iniciando " + strconv.Itoa(i+1))
		}
		return nil
	}

	// El daemon se desacopla: se cierra de su sesión para que no
	// dependa de la terminal que lo lanzó, y se anota el PID para poder
	// controlarlo después.
	if err := proc.start(true); err != nil {
		return err
	}

	fmt.Println()
	cli.Ok("Ventana abierta con el tema '%s'.", theme)
	cli.Say("")
	cli.Say("El daemon quedó en background. La ventana sigue viva aunque")
	cli.Say("cierres la terminal.")
	fmt.Println()
	cli.Say("Para controlarlo:")
	cli.Say("  boot-preview animation status      # ver el estado")
	cli.Say("  boot-preview animation reload      # recargar tras editar un PNG")
	cli.Say("  boot-preview animation update '...' # cambiar el mensaje")
	cli.Say("  boot-preview animation stop        # cerrar el daemon")
	return nil
}

// plymouthProcess es el plymouthd corriendo en segundo plano.
type plymouthProcess struct {
	cmd      *exec.Cmd
	etcDir   string
	runTheme string
	devNull  *os.File
	pidFile  string
}

// detach saca el daemon de la sesión de la terminal.
//
// Un proceso que muere con su terminal (SIGHUP) no sirve: el preview
// tiene que sobrevivir al cierre de la shell que lo lanzó.
//
// Se hace con setsid: el proceso se pone en su propia sesión, y con
// eso deja de recibir el SIGHUP de la terminal. El PID se anota para
// poder controlarlo después con 'boot-preview animation'.
