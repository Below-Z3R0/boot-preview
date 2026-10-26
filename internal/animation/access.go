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
		cli.Say("En Wayland usá el Xwayland: export DISPLAY=:1")
		return fmt.Errorf("falta DISPLAY: no hay servidor gráfico")
	}

	if ok, _ := x11RootAccess(); ok {
		cli.Ok("root ya puede abrir ventanas en %s", display)
		return nil
	}

	if !cli.HaveCommand("xhost") {
		cli.Warn("Falta xhost (paquete xorg-xhost).")
		cli.Say("Instalalo con: sudo pacman -S xorg-xhost")
		return fmt.Errorf("falta xhost")
	}

	cli.Say("Ejecutando: xhost +SI:localuser:root")
	if out, err := cli.RunCombined("xhost", "+SI:localuser:root"); err != nil {
		cli.Warn("No se pudo dar el acceso: %s", out)
		cli.Say("Probá manualmente:  DISPLAY=%s xhost +SI:localuser:root", display)
		return fmt.Errorf("no se pudo dar acceso a root: %s", out)
	}

	cli.Ok("root puede abrir ventanas en %s", display)
	cli.Say("El permiso dura hasta que cierres la sesión gráfica.")
	return nil
}

// x11RootAccess dice si root puede conectarse al servidor X.
//
// Devuelve también el motivo cuando no, para no mandar al usuario a
// correr `access` cuando en realidad le falta xdotool: el mensaje
// anterior decía "root no puede abrir una ventana" en los dos casos.
func x11RootAccess() (bool, string) {
	if !cli.HaveCommand("xdotool") {
		return false, "falta xdotool (necesario para probar el acceso)"
	}

	cmd := exec.Command("sudo", "xdotool", "getdisplaygeometry")
	cmd.Env = append(os.Environ(),
		"DISPLAY="+os.Getenv("DISPLAY"),
		"XAUTHORITY="+os.Getenv("XAUTHORITY"))
	if err := cmd.Run(); err != nil {
		return false, "root no tiene acceso al servidor X"
	}
	return true, ""
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
	if daemonRunning() {
		cli.Warn("el daemon sigue respondiendo; probá 'sudo pkill -f plymouthd'")
		return fmt.Errorf("no se pudo cerrar el daemon")
	}
	cli.Ok("daemon cerrado")
	return nil
}
