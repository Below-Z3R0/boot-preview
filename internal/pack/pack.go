// Package pack arma el paquete descargable y mantiene el hook de pacman.
//
// Son tres cosas que comparten el theme de "distribuir el tool":
//
//	update.go  traer la última versión
//	hooks.go   el hook de pacman que avisa tras actualizar
//	bundle.go  armar el tarball descargable
package pack

import (
	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// Update trae la última versión del tool.
func Update(paths cli.ToolPaths) error { return update(paths) }

// Hooks instala o quita el hook de pacman.
func Hooks(args []string, paths cli.ToolPaths) error { return hooks(args, paths) }

// Bundle arma el paquete descargable.
func Bundle(paths cli.ToolPaths) error { return bundle(paths) }
