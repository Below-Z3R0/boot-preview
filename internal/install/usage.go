package install

import "fmt"

// usage muestra la ayuda del comando install.
func usage() {
	fmt.Println(`install — instala boot-preview

  install                  Instala todo (loader + animation)
  install loader           Solo el módulo de Limine
  install animation        Solo el módulo de Plymouth
  install --no-deps        No instala paquetes del sistema`)
}

// Los comandos de cada módulo, en un solo lugar: el bloque "todo" es la
// suma de los dos. Antes estaba escrito tres veces y se desincronizaba.
const (
	loaderHelp = `  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader real         # con tu config real de /boot/limine.conf
      boot-preview loader shot         # captura a PNG
      boot-preview loader add ~/Pictures/foto.png
      boot-preview loader edit         # abrir la config en tu editor
      boot-preview loader apply        # aplicar al Limine real
      boot-preview loader wallpapers   # las dos versiones del wallpaper`

	animationHelp = `  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation sources   # los temas instalados
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)`
)

// printUsage muestra los comandos de lo recién instalado.
func printUsage(what string) {
	switch what {
	case "loader":
		fmt.Println(loaderHelp)
	case "animation":
		fmt.Println(animationHelp)
	default:
		fmt.Println(loaderHelp)
		fmt.Println()
		fmt.Println(animationHelp)
	}
}
