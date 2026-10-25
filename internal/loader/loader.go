// Package loader previsualiza el menú de Limine.
//
// Limine no tiene previewer: la única forma de ver el menú sin reiniciar
// es emularlo. Este paquete arma un ISO híbrido con la config y lo abre
// en QEMU con KVM. El render es exacto: mismo bootloader, mismo
// wallpaper, misma paleta. El disco real no se toca.
//
// El paquete se divide por comando:
//
//	loader.go    el dispatcher: parsea los argumentos y elige comando
//	add.go       poner una imagen como wallpaper del preview
//	apply.go     aplicar el tema al Limine real (/boot/limine.conf)
//	preview.go   el preview: el ISO y la VM
//	shot.go      la captura de la pantalla de la VM
//	wallpaper.go generar las dos versiones del wallpaper
//	commands.go  los helpers del paquete
//	usage.go     la ayuda
package loader

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── Constantes ────────────────────────────────────────────────────────────────

const (
	// limineDir es donde el paquete de Limine deja sus archivos.
	limineDir = "/usr/share/limine"

	// workDir es el árbol de trabajo del ISO.
	workDir = "/tmp/boot-preview-loader"

	// ovmfCode y ovmfVars son el firmware UEFI. Sin él, QEMU arranca con
	// SeaBIOS y el menú sale en modo texto (sin wallpaper).
	ovmfCode = "/usr/share/edk2/x64/OVMF_CODE.4m.fd"
	ovmfVars = "/usr/share/edk2/x64/OVMF_VARS.4m.fd"
)

// themeOptions son las opciones de theming que se llevan al Limine real.
var themeOptions = []string{
	"wallpaper", "wallpaper_style", "backdrop",
	"term_margin", "term_margin_gradient", "term_background", "term_foreground",
	"term_palette", "term_palette_bright", "term_font", "term_font_size",
	"term_font_scale", "term_font_spacing",
	"interface_resolution", "interface_rotation", "interface_branding",
	"interface_branding_colour", "interface_help_colour",
	"interface_help_colour_bright", "interface_help_hidden",
}

// ownComments son los comentarios que escribe boot-preview. Se borran
// antes de insertar el bloque nuevo, o se duplicarían.
var ownComments = []string{
	"# ── Limine theming (b3l0wz3r0-os) ──",
	"# Aplicado por boot-preview loader apply",
	"# y volvé a aplicar para cambiarlo",

	// El tool se llamaba limine-preview antes: los configs que hay
	// instalados en /boot tienen el comentario viejo, y si no se
	// recognizes el bloque anterior se queda pegado al reinyectar.
	"# Aplicado por limine-preview apply",
}

// options son las opciones del comando.
type options struct {
	command    string // preview, shot, add, edit, apply, wallpapers
	real       bool
	wallpaper  string
	conf       string
	resolution string
	timeout    int
	shotPath   string
	addImage   string
	applyWhat  string // all, config, wallpaper

	// Las opciones de preparation del wallpaper (ver `loader add --help`).
	blur    string  // "0xRADIO"; "0x0" = sin desenfoque
	dim     float64 // velo negro uniforme (0 = nada, 1 = negro)
	overlay string  // velo de ese color en vez de negro
	format  string  // png o qoi
}

// Run ejecuta el comando loader.
func Run(args []string, paths cli.Paths) error {
	opts := options{
		command:    "preview",
		wallpaper:  filepath.Join(paths.LoaderConfig(), "wallpaper", "wallpaper.png"),
		conf:       filepath.Join(paths.LoaderConfig(), "limine.conf"),
		resolution: "1920x1080",
		timeout:    15,
		applyWhat:  "all",
		blur:       "0x0",
		format:     "png",
	}

	rest := args
	if len(args) > 0 {
		switch args[0] {
		case "real", "shot", "wall", "add", "edit", "apply", "wallpapers", "help", "-h", "--help":
			opts.command = args[0]
			rest = args[1:]
		default:
			// Un subcomando que no existe tiene que avisar, no abrir la
			// VM como si fuera el preview.
			return fmt.Errorf("opción desconocida: %s", args[0])
		}
	}

	switch opts.command {
	case "help", "-h", "--help":
		usage()
		return nil
	case "real":
		opts.command = "preview"
		opts.real = true
	case "shot":
		opts.command = "shot"
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			opts.shotPath = rest[0]
			rest = rest[1:]
		}
	case "wall":
		opts.command = "preview"
		if len(rest) == 0 {
			return fmt.Errorf("falta la imagen")
		}
		opts.wallpaper = rest[0]
		rest = rest[1:]
	case "add":
		opts.command = "add"
		if len(rest) == 0 {
			return fmt.Errorf("falta la imagen")
		}
		opts.addImage = rest[0]
		rest = rest[1:]
	case "apply":
		opts.command = "apply"
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			switch rest[0] {
			case "config", "wallpaper":
				opts.applyWhat = rest[0]
				rest = rest[1:]
			default:
				return fmt.Errorf("opción de apply desconocida: %s (valores: config, wallpaper)", rest[0])
			}
		}
	}

	// Las opciones comunes.
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "-c", "--config":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.conf = rest[i+1]
			i++
		case "-w", "--wallpaper":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.wallpaper = rest[i+1]
			i++
		case "--resolution", "--res":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.resolution = rest[i+1]
			i++
		case "--timeout":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.timeout, _ = strconv.Atoi(rest[i+1])
			i++
		case "--real":
			opts.real = true
		case "--blur":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.blur = rest[i+1]
			i++
		case "--dim":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.dim, _ = strconv.ParseFloat(rest[i+1], 64)
			i++
		case "--overlay":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.overlay = rest[i+1]
			i++
		case "--format":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.format = rest[i+1]
			i++
		case "-h", "--help":
			usage()
			return nil
		default:
			return fmt.Errorf("opción desconocida: %s", rest[i])
		}
	}

	switch opts.command {
	case "add":
		return cmdAdd(opts, paths)
	case "edit":
		return cmdEdit(opts, paths)
	case "apply":
		return cmdApply(opts, paths)
	case "wallpapers":
		return cmdWallpapers(opts, paths)
	case "shot":
		return cmdShot(opts, paths)
	default:
		return cmdPreview(opts, paths)
	}
}
