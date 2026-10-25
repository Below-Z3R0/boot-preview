package animation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
	"github.com/Below-Z3R0/boot-preview/internal/steam"
)

// ── La conversión de un video a tema ───────────────────────────────────────

// cmdSteamConvert convierte un video de Steam en un tema de Plymouth.
//
// El video se convierte a frames PNG con ffmpeg y se arma un tema con el
// plugin "script" (no "two-step": two-step ata la animación al final del
// boot y sin un boot real nunca arranca).
func cmdSteamConvert(args []string, paths cli.Paths, apply bool) error {
	video := "auto"
	if len(args) > 0 && args[0] != "" {
		video = args[0]
	}

	home := cli.Home()
	if video == "auto" {
		video = steam.ActiveVideo(home)
		if video == "" {
			video = steam.DefaultVideo(home)
		}
	}

	// Se acepta un nombre relativo dentro de movies/.
	if _, err := os.Stat(video); err != nil {
		for _, root := range []string{
			filepath.Join(home, ".local", "share", "Steam", "steamui", "movies"),
			filepath.Join(home, ".steam", "steam", "steamui", "movies"),
		} {
			candidate := filepath.Join(root, video)
			if _, err := os.Stat(candidate); err == nil {
				video = candidate
				break
			}
		}
	}

	if _, err := os.Stat(video); err != nil {
		return fmt.Errorf("no existe el video: %s (ver los disponibles: boot-preview animation steam-source)", video)
	}

	// El nombre del tema: del nombre real del item si es una comprada.
	name := themeNameFor(video, home)

	if err := convertVideoToTheme(video, name); err != nil {
		return err
	}

	if apply {
		return cmdUse([]string{name}, paths)
	}

	fmt.Println()
	cli.Say("Para verlo:     boot-preview animation preview %s", name)
	cli.Say("Para aplicarlo: boot-preview animation use %s", name)
	return nil
}

// themeNameFor arma el nombre del tema a partir del video.
//
// Las compradas se llaman <item_id>_<hash>.webm: se usa el nombre real
// ("Elden Ring Ranni" -> steam-elden-ring-ranni).
func themeNameFor(video, home string) string {
	base := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))

	if id, _, ok := strings.Cut(base, "_"); ok && isDigits(id) {
		for _, m := range steam.Movies(home) {
			if m.ID == id && m.Name != "" {
				base = m.Name
				break
			}
		}
		if base == id {
			base = "item-" + id
		}
	}

	// A slug: minúsculas, espacios y símbolos a guiones.
	slug := strings.ToLower(base)
	var b strings.Builder
	lastDash := false
	for _, r := range slug {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug = strings.Trim(b.String(), "-")

	if strings.HasPrefix(slug, "steam") {
		return slug
	}
	return "steam-" + slug
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// convertVideoToTheme extrae los frames y arma el tema.
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

	cli.Step("Extrayendo frames (%s, %dfps, %s)", filepath.Base(video), fps, res)

	// Crop centrado al aspecto pedido, escalado Lanczos. Sin audio.
	//
	// El crop NO se pierde nada: los videos de Steam (1920x1200) traen la
	// imagen centrada con bordes negros, así que recortar a 16:9 la conserva.
	filter := fmt.Sprintf(
		"fps=%d,crop=iw:min(ih\\,iw*%d/%d):0:(ih-min(ih\\,iw*%d/%d))/2,scale=%d:%d:flags=lanczos",
		fps, h, w, h, w, w, h)

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
	screenW, screenH := screenSize()
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

	// EL TIMING: Plymouth refresca a FRAMES_PER_SECOND (50 en el plugin
	// script). Si el script avanza 1 frame por refresh, una animación de
	// 30 fps se reproduce 1.67x más rápido y se repite a la mitad.
	//
	// Para que la duración sea la del video hay que avanzar fps/50 frames
	// por refresh. Como eso no siempre es un entero, se usan dos
	// contadores: uno lleva la parte fraccionaria y el otro avanza los
	// frames completos.
	stepNum, stepDen := timingFor(fps)

	script := buildScript(frames, fps, stepNum, stepDen)
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
func buildScript(frames []string, fps, stepNum, stepDen int) string {
	var b strings.Builder

	// Sin frames no hay animacion: se devuelve un script vacio en vez de
	// entrar en panic con frames[0].
	if len(frames) == 0 {
		return "# Sin frames: no hay nada que reproducir.\n"
	}

	b.WriteString("# Animación generada desde un video de Steam.\n")
	b.WriteString("# Plugin \"script\" de Plymouth: loop libre, no atado al boot real.\n")
	b.WriteString("#\n")
	b.WriteString("# Timing: Plymouth refresca a 50 fps (FRAMES_PER_SECOND del plugin\n")
	b.WriteString("# script). El video va a ")
	fmt.Fprintf(&b, "%d", fps)
	b.WriteString(" fps, así que se avanzan ")
	fmt.Fprintf(&b, "%d", stepNum)
	b.WriteString(" frames\n")
	b.WriteString("# cada ")
	fmt.Fprintf(&b, "%d", stepDen)
	b.WriteString(" refreshes para que el timing sea el del video.\n")
	b.WriteString("\n")
	b.WriteString("frames = [\n")
	for _, frame := range frames {
		fmt.Fprintf(&b, "  \"%s\",\n", filepath.Base(frame))
	}
	b.WriteString("];\n")
	b.WriteString("\n")
	b.WriteString("Window.SetBackgroundTopColor(0, 0, 0);\n")
	b.WriteString("Window.SetBackgroundBottomColor(0, 0, 0);\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "sprite = Sprite(Image(\"%s\"));\n", filepath.Base(frames[0]))
	b.WriteString("sprite.SetZ(0);\n")
	b.WriteString("sprite.SetOpacity(1);\n")
	b.WriteString("\n")
	b.WriteString("frame = 0;\n")
	b.WriteString("tick = 0;\n")
	b.WriteString("\n")
	b.WriteString("fun Refresh ()\n")
	b.WriteString("{\n")
	if stepNum == 1 && stepDen == 1 {
		fmt.Fprintf(&b, "  frame = (frame + 1) %% %d;\n", len(frames))
	} else {
		fmt.Fprintf(&b, "  tick = (tick + 1) %% %d;\n", stepDen)
		b.WriteString("  if (tick == 0) {\n")
		fmt.Fprintf(&b, "    frame = (frame + %d) %% %d;\n", stepNum, len(frames))
		b.WriteString("  }\n")
	}
	b.WriteString("  sprite.SetImage(Image(frames[frame]));\n")
	b.WriteString("}\n")
	b.WriteString("\n")
	b.WriteString("Plymouth.SetRefreshFunction(Refresh);\n")

	return b.String()
}

// timingFor calcula cuántos frames avanza por cada refresh.
//
// El playback real = refresh_rate * frames_por_refresh. Para que la
// animación dure lo mismo que el video, frames_por_refresh = fps/50.
//
//	30 fps -> 3 frames cada 5 refreshes  (3/5 = 0.6 = 30/50)
//	60 fps -> 6 frames cada 5 refreshes  (6/5 = 1.2 = 60/50)
//	50 fps -> 1 frame por refresh        (exacto)
func timingFor(fps int) (num, den int) {
	if fps <= 0 {
		fps = 30
	}
	if fps == refreshRate {
		return 1, 1
	}

	// Se busca la fracción más pequeña que represente fps/50.
	for den := 1; den <= 60; den++ {
		for num := 1; num <= 120; num++ {
			if num*refreshRate == fps*den {
				return num, den
			}
		}
	}

	// Si no hay fracción exacta, se redondea.
	return max(1, fps*den/refreshRate), 1
}

// detectVideoProps saca el fps, el ancho y el alto del video.
//
// ffprobe devuelve los campos en orden alfabético, así que se pide cada
// uno por separado: el orden no se asume.
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
func screenSize() (int, int) {
	w, h := 1920, 1080
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
	return w, h
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

// ── Utilidades ──────────────────────────────────────────────────────────────

// sleep espera un rato.
func sleep(d time.Duration) {
	time.Sleep(d)
}

// timeStamp devuelve el timestamp para los nombres de archivo.
func timeStamp() string {
	return time.Now().Format("20060102-150405")
}

// copyFile copia un archivo.

// fileSize devuelve el tamaño de un archivo, en formato legible.

// dirSize devuelve el tamaño de un directorio, en formato legible.
func dirSize(path string) string {
	out, err := cli.RunCombined("du", "-sh", path)
	if err != nil {
		return "?"
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "?"
	}
	return fields[0]
}

// parseRes separa "WxH".
// parseRes parsea la resolución del tema.
//
// Si no se puede, cae en 1920x1080: es la resolución de la mayoría de las
// pantallas y el tema se ve igual de bien que con cualquier otra.
func parseRes(res string) (int, int) {
	w, h, err := cli.ParseRes(res)
	if err != nil {
		return 1920, 1080
	}
	return w, h
}
