// Package animation previsualiza el splash de Plymouth.
//
// Usa el plymouthd oficial con el renderer X11: dibuja en una ventana
// GTK sobre el escritorio. No es un reproductor de imágenes: es el mismo
// daemon, los mismos plugins y el mismo tema que usa la máquina al
// arrancar. Sin VM, sin initramfs.
//
// El paquete se divide por comando:
//
//	animation.go       el dispatcher: parsea los argumentos y elige comando
//	x11.go             el preview: la ventana con el splash
//	daemon.go          el ciclo de vida del daemon (arrancar, parar)
//	daemon_commands.go controlarlo: status, show, reload, update...
//	splash.go          hablar con el daemon
//	env.go             el entorno que se le pasa al daemon
//	log.go             el log del daemon
//	shot.go            la captura de la ventana
//	access.go          darle a root acceso al servidor X
//	theme.go           los temas: cuál está activo, cuál se previsualiza
//	sources.go         listar los temas instalados
//	use.go             aplicar un tema al sistema
//	steam_source.go    los videos de startup de Steam
//	steam_convert.go   convertir un video en tema
//	convert_video.go   la conversión en sí
//	detect.go          las propiedades del video (fps, resolución)
//	script.go          el script de Plymouth que reproduce los frames
//	name.go            el nombre del tema a partir del video
package animation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

const (
	themesDir = "/usr/share/plymouth/themes"
	sysConf   = "/etc/plymouth/plymouthd.conf"
	x11Log    = "/tmp/plymouth-x11.log"

	// refreshRate es el FRAMES_PER_SECOND del plugin "script" de Plymouth.
	// El script se refresca a esa frecuencia, y de ahí sale el timing.
	refreshRate = 50
)

func Run(args []string, paths cli.ToolPaths) error {
	cmd := "x11"
	rest := args

	if len(args) > 0 {
		switch args[0] {
		case "x11", "shot", "access", "sources", "preview", "use", "stop",
			"status", "ping", "show", "hide", "reload", "update", "pause",
			"unpause", "error", "quit",
			"steam-source", "steam-convert", "steam-apply", "help", "-h", "--help":
			cmd = args[0]
			rest = args[1:]
		default:
			// Un número (los segundos) o una opción (-f) son del preview:
			// `animation 30` y `animation --foreground` tienen que
			// funcionar. Antes cualquier cosa que no fuera subcomando daba
			// "opción desconocida", así que el límite de tiempo del usage
			// no se podía usar.
			if _, err := strconv.Atoi(args[0]); err == nil || strings.HasPrefix(args[0], "-") {
				break
			}
			// Un subcomando que no existe tiene que avisar, no abrir la
			// ventana como si fuera el preview.
			return fmt.Errorf("opción desconocida: %s", args[0])
		}
	}

	switch cmd {
	case "x11":
		return cmdX11(rest, paths)
	case "shot":
		return cmdShot(rest, paths)
	case "access":
		return cmdAccess()
	case "stop", "quit":
		return cmdStop()
	case "status", "ping", "show", "hide", "reload", "update", "pause", "unpause", "error":
		// El wrapper recibe los args completos: el propio subcomando es
		// lo que tiene que interpretar.
		return runDaemonCommands(args)
	case "sources":
		return cmdSources(paths)
	case "preview":
		return cmdPreview(rest, paths)
	case "use":
		return cmdUse(rest, paths)
	case "steam-source":
		return cmdSteamSource()
	case "steam-convert":
		return cmdSteamConvert(rest, paths, false)
	case "steam-apply":
		return cmdSteamConvert(rest, paths, true)
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		// Sin subcomando: la ventana con el splash.
		return cmdX11(rest, paths)
	}
}
