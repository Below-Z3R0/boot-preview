package animation

import "fmt"

// usage muestra la ayuda del comando animation.
func usage() {
	fmt.Print(`boot-preview animation — ver y convertir splashes de Plymouth sin reiniciar

Usa el Plymouth OFICIAL del sistema (plymouthd + sus plugins) con el
renderer X11: dibuja en una ventana GTK sobre tu escritorio. No es un
reproductor de imágenes: es el mismo daemon, los mismos plugins y el mismo
tema que usa la máquina al arrancar. Sin VM, sin initramfs.

USO
  boot-preview animation [comando] [opciones]

VER
  (ninguno)              Ventana con el splash. El daemon queda en
                         background: la ventana sobrevive al cierre de
                         la terminal
  x11 [segundos]         Idem, con límite de tiempo
  x11 --foreground       Idem, pero el comando espera (Ctrl+C para salir)

CAPTURAR
  shot [archivo]         Captura el splash a un PNG + contact sheet.
                         Sin nombre va a <repo>/previews/animation/

ACCESO
  access                 Da acceso de root al servidor X (una vez por
                         sesión). Sin esto el splash no aparece y no hay
                         error: plymouthd exige root y root necesita
                         permiso de X.

EL DAEMON
  status                 Si el daemon corre y qué tema muestra
  ping                   Si el daemon responde
  show                   Mostrar el splash
  hide                   Ocultarlo
  reload                 Recargar el tema (después de editar un PNG)
  update <texto>         Cambiar el mensaje de estado ("Iniciando...")
  pause                  Pausar la barra de progreso
  unpause                Reanudarla
  error                  Reportar un error de arranque
  stop                   Cerrar el daemon (igual que quit)

TEMAS
  sources                Los temas de Plymouth instalados
  preview <tema>         Cambia el tema SOLO del preview
  use <tema>             Aplica el tema al SISTEMA (-R, regenera el
                         initramfs)

STEAM
  steam-source           Los videos de startup (cuál está activo)
  steam-convert <vid>   Convierte un video en tema de Plymouth
  steam-apply <vid>     Convierte y lo aplica al sistema

OTROS
  help                   Esta ayuda

EJEMPLOS
  boot-preview animation
  boot-preview animation access
  boot-preview animation shot
  boot-preview animation status
  boot-preview animation reload
  boot-preview animation sources

AJUSTES DE LA CONVERSIÓN
  STEAM_FPS=15 STEAM_RES=1280x720 boot-preview animation steam-convert
                         15 fps a 720p: 105 frames, 45 MB. A 30 fps y
                         1080p son 210 frames y 133 MB.
`)
}
