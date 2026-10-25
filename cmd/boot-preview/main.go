// boot-preview — previsualizar el arranque de Linux sin reiniciar.
//
//	boot-preview loader      el menú de Limine (el bootloader)
//	boot-preview animation   el splash de Plymouth (la animación)
//
// Además, el tool se instala y se mantiene desde acá:
//
//	boot-preview install [loader|animation]
//	boot-preview verify
//	boot-preview update
//	boot-preview hooks
package main

import (
	"fmt"
	"os"

	"github.com/Below-Z3R0/boot-preview/internal/animation"
	"github.com/Below-Z3R0/boot-preview/internal/cli"
	"github.com/Below-Z3R0/boot-preview/internal/install"
	"github.com/Below-Z3R0/boot-preview/internal/loader"
	"github.com/Below-Z3R0/boot-preview/internal/pack"
	"github.com/Below-Z3R0/boot-preview/internal/verify"
)

// version se inyecta en el build con -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

// run despacha el subcomando.
func run(args []string) error {
	paths := cli.NewPaths()

	if len(args) == 0 {
		usage()
		return nil
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "loader":
		return loader.Run(rest, paths)
	case "animation":
		return animation.Run(rest, paths)
	case "install", "setup":
		return install.Run(rest, paths)
	case "verify", "doctor":
		return verify.Run(paths)
	case "update":
		return pack.Update(paths)
	case "hooks":
		return pack.Hooks(rest, paths)
	case "bundle":
		return pack.Bundle(paths)
	case "version", "--version":
		fmt.Printf("boot-preview %s\n", version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Herramienta desconocida: %s\n\n", cmd)
		usage()
		return fmt.Errorf("")
	}
}

// usage muestra la ayuda general.
func usage() {
	fmt.Print(`boot-preview — previsualizar el arranque de Linux sin reiniciar

USO
  boot-preview <herramienta> [comando] [opciones]

HERRAMIENTAS
  loader      El menú de Limine (el bootloader)
              Construye un ISO híbrido con tu config y lo abre en QEMU
              con KVM. El render es exacto: mismo bootloader, mismo
              wallpaper, misma paleta. No toca el disco real.

  animation   El splash de Plymouth (la animación)
              Usa el plymouthd oficial con el renderer X11: dibuja en
              una ventana sobre tu escritorio. Sin VM, sin initramfs.

INSTALACIÓN Y MANTENIMIENTO
  install [loader|animation]  Instala el tool (o solo una parte)
  verify                       Verifica que todo esté en su sitio
  update                       Trae la última versión
  hooks [install|uninstall]   El hook de pacman que avisa tras actualizar
  bundle                      Arma el paquete descargable

AYUDA DE CADA UNO
  boot-preview loader --help
  boot-preview animation --help

EJEMPLOS
  boot-preview loader              # preview del menú de Limine
  boot-preview loader shot         # captura del menú a PNG
  boot-preview animation           # ventana con el splash de Plymouth
  boot-preview animation shot      # captura del splash a PNG
  boot-preview install animation   # instalar solo el módulo de Plymouth
`)
}
