package loader

import "fmt"

func usage() {
	fmt.Print(`boot-preview loader — ver el menú de Limine sin reiniciar

USO
  boot-preview loader [comando] [opciones]

VER
  (ninguno)        Preview del menú, con la config de prueba
  real             Preview con tu config real de /boot/limine.conf
  wall <imagen>    Preview con otro wallpaper (sin instalarlo)

CAPTURAR
  shot [archivo]   Captura el menú a un PNG. Sin nombre va a
                   <repo>/previews/loader/ con timestamp

EDITAR
  add <imagen>     Pone una imagen como wallpaper del preview
                   (con --blur/--dim/--overlay la prepara antes)
  edit             Abre la config del preview en tu editor

APLICAR AL LIMINE REAL
  apply             Wallpaper + config visual
  apply config      Solo la config visual
  apply wallpaper   Solo el wallpaper

WALLPAPERS
  wallpapers       Genera las dos versiones (real y difuminada)

OTROS
  help             Esta ayuda

INSTALACIÓN (de todo el tool)
  boot-preview setup

EJEMPLOS
  boot-preview loader
  boot-preview loader shot
  boot-preview loader add ~/Pictures/foto.png
  boot-preview loader edit
  boot-preview loader apply

OPCIONES AVANZADAS
  -c, --config <ruta>     Config de Limine a usar
  -w, --wallpaper <ruta>  Wallpaper a incluir en el ISO
      --resolution <WxH>  Resolución de la interfaz (default: 1920x1080)
      --timeout <seg>     Espera antes de capturar (default: 15)

PREPARAR EL WALLPAPER (para add)
  Limine no puede difuminar: el blur se aplica ANTES, a la imagen.
      --blur 0x30          Desenfoque gaussiano (0xRADIO). 0x0 = sin desenfoque
      --dim 0.4            Velo negro uniforme encima (0 = nada, 1 = negro)
      --overlay "#191724"  Velo de ese color en vez de negro
      --format qoi         QOI: lossless y se decodifica más rápido que PNG
`)
}
