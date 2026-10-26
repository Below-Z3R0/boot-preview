// Package wallpaper prepara las imágenes para Limine.
//
// Limine NO puede difuminar una imagen: dibuja el wallpaper tal cual, con
// el escalado que toque. El blur hay que hacerlo ANTES, al preparar el
// fichero. Este paquete lo hace, y de paso deja la imagen en la resolución
// exacta de la interfaz, para que no haya reescalado y se vea nítida.
package wallpaper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// Options son las opciones de la preparación.
type Options struct {
	Src     string
	Out     string
	Blur    string // "0xRADIO"; "0x0" = sin desenfoque
	Res     string // "WxH"; vacío = la de la imagen
	Format  string // "png" o "qoi"
	Dim     float64
	Overlay string
}

// Prepare genera el wallpaper.
//
// Se prefiere vips (libvips): es ~6x más rápido que ImageMagick para
// resize+blur. Pero vips no escribe QOI, y para el velo de color (no
// negro) hay que mezclar, así que esos casos pasan por ImageMagick.
func Prepare(opts Options) error {
	if opts.Src == "" {
		return fmt.Errorf("falta la imagen de origen")
	}
	if _, err := os.Stat(opts.Src); err != nil {
		return fmt.Errorf("no existe: %s", opts.Src)
	}
	if opts.Format == "" {
		opts.Format = "png"
	}
	if opts.Blur == "" {
		opts.Blur = "0x0"
	}
	if opts.Overlay == "" {
		opts.Overlay = "#000000"
	}

	haveVips := haveCommand("vips")
	if !haveVips && !haveCommand("magick") {
		return fmt.Errorf("falta: imagemagick (o vips)")
	}
	if opts.Format == "qoi" && !haveCommand("magick") {
		return fmt.Errorf("falta: imagemagick (para el formato QOI)")
	}

	out := opts.Out
	if out == "" {
		base := strings.TrimSuffix(opts.Src, filepath.Ext(opts.Src))
		out = base + "-limine." + opts.Format
	}

	tmpDir, err := os.MkdirTemp("", "limine-wp.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpOut := filepath.Join(tmpDir, "out.png")

	if haveVips {
		if err := prepareWithVips(opts, tmpOut); err != nil {
			return err
		}
	} else {
		if err := prepareWithMagick(opts, tmpOut); err != nil {
			return err
		}
	}

	// ── La salida ──
	switch opts.Format {
	case "qoi":
		// QOI necesita la extensión .qoi para que ImageMagick elija el
		// encoder. (vips no sabe escribir QOI, por eso esta rama siempre
		// pasa por magick.)
		out = strings.TrimSuffix(out, filepath.Ext(out)) + ".qoi"
		if err := run("magick", tmpOut, "-define", "qoi:codec=1", out); err != nil {
			return fmt.Errorf("no se pudo escribir el QOI: %s", err)
		}
	case "png":
		if haveVips {
			// tmpOut ya es un PNG escrito por vips: no hace falta
			// re-codificar.
			if err := cli.CopyFile(tmpOut, out); err != nil {
				return err
			}
		} else {
			if err := run("magick", tmpOut, "PNG32:"+out); err != nil {
				return fmt.Errorf("no se pudo escribir el PNG: %s", err)
			}
		}
	default:
		return fmt.Errorf("formato desconocido: %s", opts.Format)
	}

	w, h := cli.ImageSize(out)
	size := cli.FileSize(out)
	fmt.Printf("%dx%d  %s  %s\n", w, h, size, out)
	return nil
}

// prepareWithVips hace el pipeline con libvips.
//
// libvips NO puede leer y escribir el mismo archivo: en operaciones con
// acceso aleatorio (crop) revienta con "Bus error (core dumped)". Por eso
// se alterna entre dos temporales, uno lee y el otro escribe.
func prepareWithVips(opts Options, out string) error {
	tmpA := filepath.Join(filepath.Dir(out), "a.v")
	tmpB := filepath.Join(filepath.Dir(out), "b.v")

	cur, next := tmpA, tmpB
	swap := func() { cur, next = next, cur }

	// runVips ejecuta un paso de vips y detecta el fallo. Sin esto, un
	// fallo silencioso deja un PNG vacío sin decir nada.
	runVips := func(desc string, args ...string) error {
		if err := run("vips", args...); err != nil {
			return fmt.Errorf("vips falló en %q: %s", desc, err)
		}
		swap()
		return nil
	}

	if opts.Res != "" {
		tw, th, err := cli.ParseRes(opts.Res)
		if err != nil {
			return err
		}
		sw, sh := cli.ImageSize(opts.Src)
		if sw == 0 || sh == 0 {
			return fmt.Errorf("no se pudo leer el tamaño de %s", opts.Src)
		}

		// El factor es el mayor de los dos: así la imagen cubre todo el
		// destino y sobra por un lado (se recorta al centro).
		scale := max(float64(tw)/float64(sw), float64(th)/float64(sh))
		nw := int(float64(sw)*scale + 0.5)
		nh := int(float64(sh)*scale + 0.5)

		if err := runVips("resize", "resize", opts.Src, next, strconv.FormatFloat(scale, 'f', 6, 64)); err != nil {
			return err
		}
		// Simétrico o no, el recorte al centro solo hace falta si sobra
		// imagen.
		if nw > tw || nh > th {
			offX := (nw - tw) / 2
			offY := (nh - th) / 2
			if err := runVips("crop", "crop", cur, next,
				strconv.Itoa(offX), strconv.Itoa(offY),
				strconv.Itoa(tw), strconv.Itoa(th)); err != nil {
				return err
			}
		}
	} else {
		if err := runVips("copy", "copy", opts.Src, next); err != nil {
			return err
		}
	}

	// Sharpen: DESPUÉS del resize (devuelve el detalle que el escalado se
	// come) y ANTES del blur (un blur afterward lo desharía).
	if opts.Res != "" && opts.Blur == "0x0" {
		if err := runVips("sharpen", "sharpen", cur, next, "--sigma", "0.9"); err != nil {
			return err
		}
	}

	// Blur gaussiano. En ImageMagick `0xSIGMA` es radio 0 + sigma SIGMA,
	// que es exactamente el sigma de vips.
	if opts.Blur != "0x0" {
		sigma := strings.TrimPrefix(opts.Blur, "0x")
		if err := runVips("gaussblur", "gaussblur", cur, next, sigma); err != nil {
			return err
		}
	}

	// Velo uniforme. Con negro alcanza un `linear` (out = in * (1-dim));
	// con otro color hace falta mezclar, y ahí vuelve magick.
	if opts.Dim > 0 {
		if opts.Overlay == "#000000" || opts.Overlay == "black" {
			factor := strconv.FormatFloat(1-opts.Dim, 'f', 6, 64)
			if err := runVips("linear", "linear", cur, next, factor, "0"); err != nil {
				return err
			}
		} else {
			percent := strconv.Itoa(int(opts.Dim * 100))
			if err := run("magick", cur, "-fill", opts.Overlay, "-colorize", percent, next); err != nil {
				return fmt.Errorf("no se pudo aplicar el velo: %s", err)
			}
			swap()
		}
	}

	return run("vips", "copy", cur, out)
}

// prepareWithMagick hace el pipeline con ImageMagick.
func prepareWithMagick(opts Options, out string) error {
	args := []string{opts.Src}
	if opts.Res != "" {
		// "^" fuerza el recorte a la relación de aspecto (como
		// PreserveAspectCrop).
		args = append(args, "-resize", opts.Res+"^", "-gravity", "center", "-extent", opts.Res)
	}
	if opts.Blur != "0x0" {
		args = append(args, "-blur", opts.Blur)
	}
	if opts.Dim > 0 {
		percent := strconv.Itoa(int(opts.Dim * 100))
		args = append(args, "-fill", opts.Overlay, "-colorize", percent)
	}
	args = append(args, "-depth", "8", "-strip", out)

	if err := run("magick", args...); err != nil {
		return fmt.Errorf("magick falló: %s", err)
	}

	// El sharpen va DESPUÉS del resize (devuelve el detalle que el
	// escalado se come), pero antes del blur (un blur afterward lo
	// desharía).
	if opts.Res != "" && opts.Blur == "0x0" {
		if err := run("magick", out, "-unsharp", "0x0.75+0.75+0.01", "-depth", "8", "-strip", out); err != nil {
			return fmt.Errorf("magick falló en el sharpen: %s", err)
		}
	}

	return nil
}

// ── Utilidades ──────────────────────────────────────────────────────────────

// run ejecuta un comando y devuelve el error con su salida.
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	// El módulo openslide de Arch viene sin su lib y vips tira un warning
	// en cada invocación. VIPS_WARNING=0 no lo silencia (sale del loader
	// de módulos, antes de leer la variable), así que se descarta stderr.
	if name == "vips" {
		cmd.Stderr = nil
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func haveCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// parseRes separa "WxH".
// parseRes parsea la resolucion pedida.
//
// Si no se puede, devuelve 0,0: el wallpaper no se genera y el comando
// avisa, en vez de fallar con un error de sintaxis.

// imageSize devuelve el tamaño de una imagen, en píxeles.

// fileSize devuelve el tamaño de un archivo, en formato legible.

// copyFile copia un archivo.
