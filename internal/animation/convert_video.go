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
		res = env
	}

	w, h := videoW, videoH
	if res != "" {
		w, h = parseRes(res)
	} else {
		// El aspect ratio del video se respeta.
		res = fmt.Sprintf("%dx%d", w, h)
	}

	// El tamaño de la pantalla: el plugin script dibuja el sprite a su
	// tamaño nativo, así que los frames tienen que salir al tamaño de la
	// pantalla para llenarla.
	screenW, screenH := screenSize()

	cli.Step("Extrayendo frames (%s, %dfps, %s)", filepath.Base(video), fps, res)

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
	filter := fmt.Sprintf(
		"fps=%d,crop=iw:min(ih\\,iw*%d/%d):0:(ih-min(ih\\,iw*%d/%d))/2,"+
			"scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black",
		refreshRate, screenH, screenW, screenH, screenW, w, h, w, h)

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
	if w != screenW || h != screenH {
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
	for _, frame := range frames {
		if out, err := cli.RunCombined("sudo", "cp", frame, dest+"/"); err != nil {
			return fmt.Errorf("no se pudo copiar %s: %s", filepath.Base(frame), out)
		}
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
