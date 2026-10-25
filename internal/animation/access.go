package animation

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── access ───────────────────────────────────────────────────────────────────

// ── access ──────────────────────────────────────────────────────────────────

// cmdAccess da acceso de root al servidor X.
//
// plymouthd exige root, y root solo puede abrir ventanas si el servidor X
// le da acceso. Sin esto el splash no aparece y no hay error.
func cmdAccess() error {
	cli.Step("Dando acceso de root al servidor X")

	display := os.Getenv("DISPLAY")
	if display == "" {
		cli.Warn("No hay DISPLAY. X11 necesita un servidor gráfico.")
		cli.Say("En Wayland usually usás el Xwayland: export DISPLAY=:1")
		return fmt.Errorf("")
	}

	if x11RootAccess() {
		cli.Ok("root ya puede abrir ventanas en %s", display)
		return nil
	}

	if !cli.HaveCommand("xhost") {
		cli.Warn("Falta xhost (paquete xorg-xhost).")
		cli.Say("Instalalo con: sudo pacman -S xorg-xhost")
		return fmt.Errorf("")
	}

	cli.Say("Ejecutando: xhost +SI:localuser:root")
	if out, err := cli.RunCombined("xhost", "+SI:localuser:root"); err != nil {
		cli.Warn("No se pudo dar el acceso: %s", out)
		cli.Say("Probá manualmente:  DISPLAY=%s xhost +SI:localuser:root", display)
		return fmt.Errorf("")
	}

	cli.Ok("root puede abrir ventanas en %s", display)
	cli.Say("El permiso dura hasta que cierres la sesión gráfica.")
	return nil
}

// x11RootAccess dice si root puede conectarse al servidor X.
func x11RootAccess() bool {
	cmd := exec.Command("sudo", "xdotool", "getdisplaygeometry")
	cmd.Env = append(os.Environ(), "DISPLAY="+os.Getenv("DISPLAY"), "XAUTHORITY="+os.Getenv("XAUTHORITY"))
	return cmd.Run() == nil
}

// cmdStop cierra el daemon de Plymouth.
//
// Es lo mismo que `boot-preview animation stop`, pero desde el módulo de
// animación: el preview y su daemon son la misma cosa.
func cmdStop() error {
	cli.Step("Cerrando Plymouth")
	if err := StopPlymouth(); err != nil {
		return err
	}
	cli.Ok("daemon cerrado")
	return nil
}
