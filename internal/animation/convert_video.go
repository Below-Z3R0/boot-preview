package animation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

func convertVideoToTheme(video, name string) error {
	if !cli.HaveCommand("ffmpeg") {
		return fmt.Errorf("falta: ffmpeg")
	}

	dest := filepath.Join(themesDir, name)
	tmp, err := os.MkdirTemp("", "plymouth-theme.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Lo que se pide se usa; si no, se detecta del video. Un video de la
	// tienda de Steam viene en 1920x1200 a 30 fps; uno del Deck, en 450x450
	// a 60 fps. Hardcodear un valor hacia que la animación se reproduzca
	// rápido y no llene la pantalla.
	videoFPS, videoW, videoH := detectVideoProps(video)
	fps := videoFPS
	res := ""
	if env := os.Getenv("STEAM_FPS"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			fps = v
		}
	}
	if env := os.Getenv("STEAM_RES"); env != "" {
		// Se valida acá: con un valor basura, parseRes caía en 1920x1080
		// en silencio y el tema salía a una resolución que nadie pidió.
		if _, _, err := cli.ParseRes(env); err != nil {
			return err
		}
		res = env
	}

	// El tamaño de la pantalla: el plugin script dibuja el sprite a su
	// tamaño nativo, así que los frames tienen que salir a ese tamaño.
	screenW, screenH := screenSize()

	// El destino es la pantalla, NO el tamaño del video. Antes se
	// extraía al tamaño del video y después se escalaba forzado a la
	// pantalla: un video 1920x1200 quedaba aplastado un 10%.
	outW, outH := screenW, screenH
	if res != "" {
		outW, outH = parseRes(res)
	}
	res = fmt.Sprintf("%dx%d", outW, outH)

	cli.Step("Extrayendo frames (%s, %dfps, %dx%d -> %s)", filepath.Base(video), fps, videoW, videoH, res)

	// Crop centrado al aspecto pedido, escalado Lanczos. Sin audio.
	//
	// El crop NO se pierde nada: los videos de Steam (1920x1200) traen la
	// imagen centrada con bordes negros, así que recortar a 16:9 la conserva.
	//
	// Los frames se extraen a la TASA DE REFRESCO de Plymouth (50 Hz), no
	// a la del video. Plymouth refresca 50 veces por segundo y el script
	// avanza un frame por refresh: si los frames fueran del video (30
	// fps) y el script avanzara 1 por refresh, la animación correría 1.67x
	// más rápido. Al extraer a 50 fps, ffmpeg duplica los frames que
	// hacen falta y el script avanza 1 por refresh: el ojo ve los 30 fps
	// reales del video, con la duración exacta.
	//
	// El crop va al ASPECTO DE LA PANTALLA, no al del video: si se
	// escalara 1920x1200 a 1920x1080 sin recortar, la imagen se estiraría
	// verticalmente (un 10% más alta).
	//
	// El crop aplica solo si el video es MAS ANCHO que la pantalla (que es
	// el caso normal: 1920x1200 en una 1920x1080). Si el video es mas
	// angosto (los del Steam Deck son cuadrados, 450x450), no se recorta:
	// se escala con barras, porque recortar el 44% de la imagen pierde
	// el contenido.
	//
	//   video 1920x1200 -> crop a 1920x1080 (las barras negras se van)
	//   video  450x450  -> escala a 450x253 con barras (nada se pierde)
	// Las bandas negras se detectan mirando el video, no por su aspecto.
	content := detectContentRect(video, videoW, videoH)
	if content.W < videoW || content.H < videoH {
		cli.Say("bandas negras: %dx%d -> %dx%d", videoW, videoH, content.W, content.H)
	}
	filter := buildVideoFilter(content, outW, outH, fps)

	ffmpeg := exec.Command("ffmpeg", "-loglevel", "error", "-y", "-i", video,
		"-vf", filter, "-start_number", "0", filepath.Join(tmp, "animation-%03d.png"))
	if out, err := ffmpeg.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg falló: %s", strings.TrimSpace(string(out)))
	}

	// Los frames, ordenados por número. El glob de bash ordena
	// LEXICALMENTE: con más de 999 frames, animation-1000.png vendría
	// antes que animation-999.png y la animación se reproducia desordenada.
	frames := listFrames(tmp)
	if len(frames) == 0 {
		return fmt.Errorf("no se pudo extraer ningún frame")
	}
	cli.Ok("%d frames", len(frames))

	// Throbber: se reusan los del tema base (el spinner de progreso).
	base := filepath.Join(themesDir, "cachyos-bootanimation")
	if entries, err := os.ReadDir(base); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "throbber-") {
				_ = cli.CopyFile(filepath.Join(base, entry.Name()), filepath.Join(tmp, entry.Name()))
			}
		}
	}

	// EL TAMAÑO: el plugin script NO tiene SetScale. El sprite se dibuja
	// a su tamaño nativo, así que para que llene la pantalla los frames se
	// generan al tamaño de la pantalla (por defecto, la de Plymouth).
	if outW != screenW || outH != screenH {
		cli.Step("Escalando a %dx%d (el plugin script no puede escalar el sprite)", screenW, screenH)
		scaled := make([]string, 0, len(frames))
		for _, frame := range frames {
			dest := filepath.Join(tmp, "big-"+filepath.Base(frame))
			if err := scaleImage(frame, dest, screenW, screenH); err != nil {
				return err
			}
			scaled = append(scaled, dest)
		}
		frames = scaled
	}

	// EL TIMING: Plymouth refresca a 50 Hz (FRAMES_PER_SECOND del plugin
	// script). El script avanza 1 frame por refresh, así que los frames
	// tienen que estar a 50 fps para que la animación vaya a la velocidad
	// del video. Ya se extrajeron así (ver el filtro de ffmpeg).
	script := buildScript(frames, fps)
	if err := os.WriteFile(filepath.Join(tmp, "script.script"), []byte(script), 0o644); err != nil {
		return err
	}

	plymouth := fmt.Sprintf(`[Plymouth Theme]
Name=%s
Description=Generado desde %s
ModuleName=script

[script]
ImageDir=%s
ScriptFile=%s/script.script
`, name, filepath.Base(video), dest, dest)
	if err := os.WriteFile(filepath.Join(tmp, name+".plymouth"), []byte(plymouth), 0o644); err != nil {
		return err
	}

	cli.Step("Instalando en %s", dest)
	_, _ = cli.RunCombined("sudo", "rm", "-rf", dest)
	// El directorio de temas es de root: el mkdir necesita sudo.
	if out, err := cli.RunCombined("sudo", "mkdir", "-p", dest); err != nil {
		return fmt.Errorf("no se pudo crear %s: %s", dest, out)
	}
	// Un solo sudo para todos los frames: uno por frame son 350 forks de
	// sudo (y 350 pedidos de credenciales) para nada.
	copyArgs := append([]string{"cp", "-t", dest}, frames...)
	if out, err := cli.RunCombined("sudo", copyArgs...); err != nil {
		return fmt.Errorf("no se pudo copiar los frames: %s", out)
	}
	for _, name := range []string{"throbber-00.png"} {
		src := filepath.Join(tmp, name)
		if _, err := os.Stat(src); err == nil {
			_, _ = cli.RunCombined("sudo", "cp", src, dest+"/")
		}
	}
	// El resto de los throbbers, si hay.
	throbbers, _ := filepath.Glob(filepath.Join(tmp, "throbber-*.png"))
	for _, src := range throbbers {
		_, _ = cli.RunCombined("sudo", "cp", src, dest+"/")
	}
	if out, err := cli.RunCombined("sudo", "cp", filepath.Join(tmp, name+".plymouth"), dest+"/"); err != nil {
		return fmt.Errorf("no se pudo copiar la config: %s", out)
	}
	if out, err := cli.RunCombined("sudo", "cp", filepath.Join(tmp, "script.script"), dest+"/"); err != nil {
		return fmt.Errorf("no se pudo copiar el script: %s", out)
	}

	cli.Ok("Tema '%s' instalado (%s)", name, dirSize(dest))
	return nil
}

// buildScript genera el script.script del tema.

// rect es el rectángulo de contenido de un video, sin las bandas negras.
type rect struct {
	W, H, X, Y int
}

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
