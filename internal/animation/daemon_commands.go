// Controlar el daemon de Plymouth.
//
// Plymouth trae un binario aparte ('plymouth') que habla con el daemon
// por un socket. Los comandos son crípticos y hay que acordarse del
// 'sudo'.
//
// Acá se exponen con nombres claros, dentro del comando 'animation':
//
//	boot-preview animation status       # ¿está vivo? ¿qué tema muestra?
//	boot-preview animation show         # mostrar el splash
//	boot-preview animation hide         # ocultarlo
//	boot-preview animation reload       # recargar (tras editar un PNG)
//	boot-preview animation update "..." # cambiar el mensaje de estado
//	boot-preview animation stop         # cerrar el daemon
package animation

import (
	"fmt"
	"os"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// Run ejecuta el comando plymouth.
func runDaemonCommands(args []string) error {
	if len(args) == 0 {
		daemonUsage()
		return nil
	}

	switch args[0] {
	case "status":
		return daemonStatus()
	case "ping":
		return daemonPing()
	case "show", "show-splash":
		return daemonCall("--show-splash")
	case "hide", "hide-splash":
		return daemonCall("--hide-splash")
	case "reload":
		return daemonCall("reload")
	case "quit":
		return daemonCall("--quit")
	case "update":
		if len(args) < 2 {
			return fmt.Errorf("falta el mensaje: boot-preview animation update \"Iniciando...\"")
		}
		return daemonCall("--update=" + strings.Join(args[1:], " "))
	case "pause":
		return daemonCall("pause-progress")
	case "unpause":
		return daemonCall("unpause-progress")
	case "error":
		return daemonCall("report-error")
	case "help", "-h", "--help":
		daemonUsage()
		return nil
	default:
		return fmt.Errorf("opción desconocida: %s (probá: boot-preview animation --help)", args[0])
	}
}

// call corre un comando de Plymouth.
//
// Con sudo siempre: el daemon corre como root y el socket es de root.
// El --preserve-env es para el DISPLAY, que Plymouth necesita para el
// renderer X11.
//
// El comando va con timeout: hay algunos que esperan una tecla
// (display-message, ask-for-password) y sin timeout la terminal queda
// colgada.
func daemonCall(args ...string) error {
	// Si el daemon no está, el error no es del comando: es que no hay a
	// quién mandarle el mensaje.
	if !daemonRunning() {
		return fmt.Errorf("el daemon de Plymouth no está corriendo (arrancá uno con: boot-preview animation)")
	}

	full := append([]string{"timeout", "5", "plymouth"}, args...)
	out, err := cli.RunCombined("sudo", append([]string{"--preserve-env=DISPLAY,XAUTHORITY"}, full...)...)
	if err != nil {
		// El timeout sale con 124: no es un error del comando, es que
		// Plymouth no respondió a tiempo.
		if strings.Contains(out, "timeout") || strings.TrimSpace(out) == "" {
			return fmt.Errorf("plymouth %s no respondió a tiempo", strings.Join(args, " "))
		}
		return fmt.Errorf("plymouth %s falló: %s", strings.Join(args, " "), strings.TrimSpace(out))
	}
	return nil
}

// ping dice si el daemon está vivo.
func daemonPing() error {
	out, err := cli.RunCombined("sudo", "--preserve-env=DISPLAY,XAUTHORITY", "plymouth", "--ping")
	if err != nil {
		cli.Warn("El daemon de Plymouth no está corriendo")
		return fmt.Errorf("")
	}
	_ = out
	cli.Ok("El daemon de Plymouth está corriendo")
	return nil
}

// status muestra el estado del daemon: si corre, y qué tema muestra.
func daemonStatus() error {
	cli.Step("Estado de Plymouth")

	// El daemon responde al ping con 0 si está vivo.
	if out, err := cli.RunCombined("sudo", "--preserve-env=DISPLAY,XAUTHORITY", "plymouth", "--ping"); err != nil {
		cli.Warn("El daemon de Plymouth no está corriendo")
		cli.Say("Para levantarlo con el tema del preview:")
		cli.Say("    boot-preview animation")
		return fmt.Errorf("")
	} else {
		_ = out
		cli.Ok("daemon corriendo")
	}

	// Qué tema está usando.
	if data, err := os.ReadFile("/etc/plymouth/plymouthd.conf"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "Theme=") {
				cli.Say("    tema del sistema: %s", strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Theme=")))
			}
		}
	}

	// El modo: boot, shutdown o reboot.
	if out, err := cli.RunCombined("plymouth", "change-mode", "--help"); err == nil {
		_ = out
	}

	return nil
}

func daemonUsage() {
	fmt.Print(`boot-preview animation — controlar el daemon de Plymouth

USO
  boot-preview animation <comando> [texto]

ESTADO
  status            Si el daemon corre y qué tema muestra
  ping              Si el daemon responde

EL SPLASH
  show              Mostrar el splash
  hide              Ocultarlo
  reload            Recargar el tema (después de editar un PNG)

EL MENSAJE
  update <texto>    Cambiar el mensaje de estado ("Iniciando...")

PROGRESO
  pause             Pausar la barra de progreso
  unpause           Reanudarla
  error             Reportar un error de arranque

VIDA
  quit              Cerrar el daemon

NOTA: los comandos que esperan una tecla (display-message, ask-for-
password) no están: bloquearían la terminal. Para el prompt de
contraseña hay que usar el comando de Plymouth directo.

EJEMPLOS
  boot-preview animation status
  boot-preview animation update "Cargando el kernel"
  boot-preview animation reload
  boot-preview animation quit
`)
}
