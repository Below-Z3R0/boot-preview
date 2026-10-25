package animation

import (
	"os"
	"os/exec"
)

func showSplash() {
	runPlymouth("--show-splash")
}

// updateSplash le manda un mensaje al splash en curso.
//
// Plymouth redibuja el splash cuando recibe un update. Sin esto el
// primer frame puede no llegar a pintarse.
func updateSplash(message string) {
	runPlymouth("--update=" + message)
}

// runPlymouth ejecuta un subcomando de plymouth como root.
//
// La salida va a /dev/null y NO por un pipe: Plymouth se comporta distinto
// si su stdout no es una terminal (deja de dibujar). Además, un pipe sin
// lector se llena y bloquea el daemon.
func runPlymouth(args ...string) {
	cmd := exec.Command("sudo", append([]string{"--preserve-env=DISPLAY,XAUTHORITY", "plymouth"}, args...)...)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
		defer devNull.Close()
	}
	_ = cmd.Run()
}

// envPrefix arma el prefijo de entorno para el daemon.
//
// sudo NO preserva DISPLAY, y sin él el renderer x11.so no puede abrir la
// ventana GTK.
//
// DISPLAY se pasa SIEMPRE, aunque el proceso no lo tenga: el
// daemon corre como root y su entorno viene del shell que lo
// lanzó.
