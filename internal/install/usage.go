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

// printUsage muestra los comandos de lo recién instalado.
func printUsage(what string) {
	switch what {
	case "loader":
		fmt.Println(`  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader real         # con tu config real de /boot/limine.conf
      boot-preview loader shot         # captura a PNG
      boot-preview loader add ~/Pictures/foto.png
      boot-preview loader edit         # abrir la config en tu editor
      boot-preview loader apply        # aplicar al Limine real
      boot-preview loader wallpapers   # las dos versiones del wallpaper`)
	case "animation":
		fmt.Println(`  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation sources   # los temas instalados
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)`)
	default:
		fmt.Println(`  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader shot         # captura a PNG
      boot-preview loader apply        # aplicar al Limine real

  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)`)
	}
}
