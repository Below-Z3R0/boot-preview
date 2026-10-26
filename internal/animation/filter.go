package animation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── El filtro de ffmpeg ─────────────────────────────────────────────────────
//
// Traduce el video a frames del tamaño de la pantalla. Dos recortes: las
// bandas negras que detecta cropdetect, y lo mínimo para que el aspecto
// coincida con el destino (solo si no se pierde más del 12%).

// maxFillCrop es cuánto se acepta recortar para llenar la pantalla.
//
// Recortar un 10% para pasar de 16:10 a 16:9 no se nota; recortar el
// 44% de un video cuadrado sí (se come la imagen). Con este umbral, el
// 1920x1200 de Steam llena la pantalla y el 450x450 del Deck va con
// barras, sin perder nada.
const maxFillCrop = 0.12

// buildVideoFilter arma el filtro de ffmpeg para extraer los frames.
//
// El destino (outW×outH) tiene que ser el tamaño de la pantalla: el
// plugin script dibuja el sprite a su tamaño nativo y no puede
// escalarlo.
//
// Dos recortes, en este orden:
//
//  1. las bandas negras reales (lo que detectó cropdetect);
//  2. lo mínimo para que el aspecto coincida con el destino, SOLO si no
//     se pierde más de maxFillCrop. Si se perdería más, se deja el
//     aspecto del video y se rellena con barras.
func buildVideoFilter(content rect, outW, outH, fps int) string {
	parts := []string{fmt.Sprintf("fps=%d", fps)}

	w, h, x, y := content.W, content.H, content.X, content.Y
	if w <= 0 || h <= 0 {
		w, h, x, y = 0, 0, 0, 0
	}
	if w > 0 && h > 0 {
		// 1) Las bandas detectadas.
		parts = append(parts, fmt.Sprintf("crop=%d:%d:%d:%d", w, h, x, y))
	}

	// 2) El recorte para llenar: cuánto se perdería en cada eje.
	if w > 0 && h > 0 {
		fillW, fillH := w, h
		// Contenido más ancho que el destino: sobra ancho.
		if w*outH > h*outW {
			fillW = h * outW / outH
			if fillW > 0 && 1-float64(fillW)/float64(w) <= maxFillCrop {
				parts = append(parts, fmt.Sprintf("crop=%d:%d:%d:0", fillW, h, (w-fillW)/2))
			}
		} else if w*outH < h*outW {
			// Contenido más alto: sobra alto.
			fillH = w * outH / outW
			if fillH > 0 && 1-float64(fillH)/float64(h) <= maxFillCrop {
				parts = append(parts, fmt.Sprintf("crop=%d:%d:0:%d", w, fillH, (h-fillH)/2))
			}
		}
	}

	parts = append(parts,
		fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos", outW, outH),
		fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black", outW, outH))

	return strings.Join(parts, ",")
}

// detectContentRect corre cropdetect y devuelve el rectángulo de
// contenido del video.
//
// Si ffmpeg falla o no detecta nada, devuelve el video entero: un crop
// al video entero no cambia nada.
func detectContentRect(video string, videoW, videoH int) rect {
	out, _ := cli.RunCombined("ffmpeg", "-hide_banner", "-loglevel", "info",
		"-i", video, "-vf", "cropdetect=limit=24:round=2:reset=0",
		"-f", "null", "-")

	return parseCropDetect(out, videoW, videoH)
}

// parseCropDetect saca el último crop=W:H:X:Y de la salida de ffmpeg.
//
// cropdetect lo imprime muchas veces (una por bloque de frames); el
// último es el que corresponde al final del video, cuando ya vio todo.
func parseCropDetect(out string, videoW, videoH int) rect {
	entero := rect{W: videoW, H: videoH}

	idx := strings.LastIndex(out, "crop=")
	if idx < 0 {
		return entero
	}
	geo := out[idx+len("crop="):]
	if end := strings.IndexAny(geo, " \n\r"); end >= 0 {
		geo = geo[:end]
	}

	fields := strings.Split(geo, ":")
	if len(fields) != 4 {
		return entero
	}
	nums := make([]int, 4)
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return entero
		}
		nums[i] = n
	}

	w, h, x, y := nums[0], nums[1], nums[2], nums[3]
	// Un rectángulo inválido o vacío no sirve.
	if w <= 0 || h <= 0 || x < 0 || y < 0 || x+w > videoW || y+h > videoH {
		return entero
	}
	return rect{W: w, H: h, X: x, Y: y}
}
