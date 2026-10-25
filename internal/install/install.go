// Package install instala el tool y deja todo andando.
//
// Se puede instalar entero o por partes: `install loader` solo deja el
// preview de Limine, `install animation` solo el de Plymouth.
//
// El paquete se divide por paso:
//
//	install.go   el dispatcher
//	deps.go      las dependencias del sistema (pacman)
//	binaries.go  el binario, los assets y el shim de sudo
//	config.go    la config de prueba y el wallpaper
//	state.go     el snapshot de versiones (lo usa `verify`)
//	usage.go     la ayuda
package install

import (
	"fmt"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

func Run(args []string, paths cli.Paths) error {
	what := "all"
	withDeps := true

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "loader", "animation", "all":
			what = args[i]
		case "--no-deps":
			withDeps = false
		case "--deps":
			withDeps = true
		case "-h", "--help":
			usage()
			return nil
		default:
			return fmt.Errorf("opción desconocida: %s", args[i])
		}
	}

	wantLoader := what == "all" || what == "loader"
	wantAnimation := what == "all" || what == "animation"

	if withDeps {
		if err := installDeps(wantLoader, wantAnimation); err != nil {
			return err
		}
	} else {
		cli.Step("1/4  Dependencias")
		cli.Ok("omitidas (--no-deps)")
	}

	if err := installBinaries(paths, wantLoader, wantAnimation); err != nil {
		return err
	}
	if err := installConfig(paths, wantLoader, wantAnimation); err != nil {
		return err
	}
	if err := writeState(paths, what); err != nil {
		return err
	}

	fmt.Printf("\n\033[1;38;2;125;207;216m✓ Listo.\033[0m\n\n")
	printUsage(what)
	fmt.Println()
	cli.Say("Verificá la instalación con:  boot-preview verify")
	return nil
}

// installDeps instala los paquetes que falten.
