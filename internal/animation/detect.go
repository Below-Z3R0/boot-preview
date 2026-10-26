package animation

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

func detectVideoProps(video string) (fps, width, height int) {
	rawFPS, _ := cli.Run("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=r_frame_rate", "-of", "csv=p=0", video)
	rawW, _ := cli.Run("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width", "-of", "csv=p=0", video)
	rawH, _ := cli.Run("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=height", "-of", "csv=p=0", video)

	width, _ = strconv.Atoi(strings.TrimSpace(rawW))
	height, _ = strconv.Atoi(strings.TrimSpace(rawH))

	// r_frame_rate viene como "30/1" (num/den).
	if num, den, ok := strings.Cut(strings.TrimSpace(rawFPS), "/"); ok {
		n, errN := strconv.Atoi(num)
		d, errD := strconv.Atoi(den)
		if errN == nil && errD == nil && d != 0 {
			fps = int(float64(n)/float64(d) + 0.5)
		}
	}
	if fps <= 0 {
		fps = 30
	}
	if width <= 0 {
		width = 1920
	}
	if height <= 0 {
		height = 1080
	}
	return fps, width, height
}

// screenSize devuelve el tamaño de la pantalla.
//
// Orden: las variables de entorno (lo que pide el preview), después la
// resolución real del X11 (xrandr) y al final 1920x1080. Antes era solo
// 1920x1080 fijo: en un monitor distinto el splash se veía con borde.
func screenSize() (int, int) {
	w, h := 0, 0

	if env := os.Getenv("PLYMOUTH_SCREEN_W"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			w = v
		}
	}
	if env := os.Getenv("PLYMOUTH_SCREEN_H"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			h = v
		}
	}

	if w <= 0 || h <= 0 {
		if out, err := cli.Run("xrandr", "--current"); err == nil {
			if xw, xh := parseXrandr(out); xw > 0 && xh > 0 {
				w, h = xw, xh
			}
		}
	}

	if w <= 0 {
		w = 1920
	}
	if h <= 0 {
		h = 1080
	}
	return w, h
}

// parseXrandr saca la resolución actual de la salida de `xrandr --current`.
//
// La línea del monitor sale así:
//
//	eDP-1 connected primary 1920x1080+0+0 (normal left inverted right x axis y axis) ...
func parseXrandr(out string) (int, int) {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, " connected") {
			continue
		}
		for _, field := range strings.Fields(line) {
			// La geometría viene como 1920x1080+0+0.
			plus := strings.IndexByte(field, '+')
			if plus < 0 || !strings.ContainsRune(field[:plus], 'x') {
				continue
			}
			if w, h, err := cli.ParseRes(field[:plus]); err == nil {
				return w, h
			}
		}
	}
	return 0, 0
}

// listFrames lista los frames de un directorio, ordenados por número.
func listFrames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "animation-") && strings.HasSuffix(entry.Name(), ".png") {
			names = append(names, entry.Name())
		}
	}
	// Orden numérico, no lexicográfico: con más de 999 frames,
	// animation-1000.png debe ir después de animation-999.png.
	sortByNumber(names)
	frames := make([]string, 0, len(names))
	for _, name := range names {
		frames = append(frames, filepath.Join(dir, name))
	}
	return frames
}

// sortByNumber ordena por el número del final del nombre.
func sortByNumber(names []string) {
	number := func(name string) int {
		base := strings.TrimSuffix(name, ".png")
		idx := strings.LastIndex(base, "-")
		if idx < 0 {
			return 0
		}
		n, _ := strconv.Atoi(base[idx+1:])
		return n
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && number(names[j]) < number(names[j-1]); j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
}

// scaleImage escala una imagen al tamaño pedido.
//
// El tamaño tiene que ser EXACTO: el sprite se dibuja a su tamaño nativo
// y si no llena la pantalla queda un borde.
//
// vips thumbnail con --size both NO fuerza el aspecto (encaja la imagen
// proporcionalmente), así que la vía principal es magick con el "!" final,
// que sí fuerza el aspecto exacto. vips queda como atajo solo cuando la
// imagen ya tiene la proporción correcta.
func scaleImage(src, dest string, w, h int) error {
	if srcW, srcH := cli.ImageSize(src); srcW > 0 && srcH > 0 && srcW*h == srcH*w {
		if cli.HaveCommand("vips") {
			_, err := cli.RunCombined("vips", "thumbnail", src, dest,
				strconv.Itoa(w), "--height", strconv.Itoa(h), "--size", "both")
			if err == nil {
				return nil
			}
		}
	}

	if cli.HaveCommand("magick") {
		// El "!" fuerza el aspecto exacto: sin él, magick encaja la
		// imagen dentro del recuadro y deja bandas.
		_, err := cli.RunCombined("magick", src, "-resize",
			fmt.Sprintf("%dx%d!", w, h), dest)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("no se pudo escalar %s (hace falta imagemagick)", filepath.Base(src))
}

// imageSize devuelve el tamaño de una imagen, en píxeles.
