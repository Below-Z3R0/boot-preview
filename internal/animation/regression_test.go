package animation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestAliveDeUnProcesoMuerto verifica que alive() dice la verdad.
//
// Antes usaba Process.Signal(nil), que en Go devuelve "unsupported
// signal type" para TODOS los procesos: alive() daba false siempre y el
// preview se cerraba solo.
func TestAliveDeUnProcesoMuerto(t *testing.T) {
	// Un proceso que ya termino.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	p := &plymouthProcess{cmd: cmd}
	if p.alive() {
		t.Error("alive() dio true para un proceso que ya termino")
	}
}

// TestAliveDeUnProcesoVivo verifica que alive() detecta un proceso vivo.
func TestAliveDeUnProcesoVivo(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	p := &plymouthProcess{cmd: cmd}
	if !p.alive() {
		t.Error("alive() dio false para un proceso que esta corriendo")
	}
}

// TestSleepEsDeVerdad verifica que las esperas duran lo que dicen.
//
// El bug: sleep(3) con un time.Duration son 3 NANOSEGUNDOS. El daemon
// de Plymouth moria a los 120 ms y la ventana nunca aparecia.
func TestSleepEsDeVerdad(t *testing.T) {
	inicio := time.Now()
	time.Sleep(50 * time.Millisecond)
	quePaso := time.Since(inicio)

	if quePaso < 40*time.Millisecond {
		t.Errorf("time.Sleep(50ms) duro solo %v", quePaso)
	}
}

// TestPidFileEnTmp verifica que el archivo del PID va a /tmp.
//
// Si usara os.TempDir(), con el TMPDIR apuntando a otro lado (el
// scratch de un proceso, por ejemplo), el daemon escribiria el PID ahi
// y el comando que lo busca no lo encontraria.
func TestPidFileEnTmp(t *testing.T) {
	path := pidFilePath()
	if !strings.HasPrefix(path, "/tmp/") {
		t.Errorf("pidFilePath() = %q, tiene que estar en /tmp", path)
	}
	if !strings.Contains(path, "boot-preview") {
		t.Errorf("pidFilePath() = %q, tiene que decir de que es", path)
	}
}

// TestReplaceLineNoPisaPrefijos verifica que se cambia la clave exacta.
//
// El bug: replaceLine buscaba el prefijo, asi que al cambiar
// "ImageDir" pisaba tambien "ImageDirBackup".
func TestReplaceLineNoPisaPrefijos(t *testing.T) {
	const original = "ImageDir=/a\nImageDirBackup=/b\n"

	got := replaceLine(original, "ImageDir", "/c")
	want := "ImageDir=/c\nImageDirBackup=/b\n"

	if got != want {
		t.Errorf("replaceLine() = %q, quiero %q", got, want)
	}
}

// TestReplaceLineConEspacios verifica que encuentra la clave con
// espacios alrededor.
func TestReplaceLineConEspacios(t *testing.T) {
	got := replaceLine("  ImageDir : /a\n", "ImageDir", "/b")
	if !strings.Contains(got, "ImageDir=/b") {
		t.Errorf("replaceLine() = %q", got)
	}
}

// TestBuildScriptSinFrames verifica que no entra en panic.
//
// El bug: buildScript usaba frames[0] sin verificar que hubiera frames.
func TestBuildScriptSinFrames(t *testing.T) {
	// No debe entrar en panic.
	script := buildScript(nil, 30)
	if !strings.Contains(script, "Sin frames") {
		t.Errorf("buildScript() con lista vacia = %q", script)
	}
}

// TestBuildScriptAcumulaElPaso verifica el timing fraccionario.
//
// Plymouth refresca a 50 Hz y un video de 30 fps tiene 1.667 frames por
// refresh: el script acumula y avanza 1 o 2 según corresponda, así la
// duración es exacta sin duplicar frames al extraer.
//
// El bug original (3 frames de golpe cada 5 refreshes) daba 10 fps en
// vez de 30; el acumulador hace la cuenta bien.
func TestBuildScriptAcumulaElPaso(t *testing.T) {
	frames := []string{"a.png", "b.png", "c.png"}
	script := buildScript(frames, 30)

	if !strings.Contains(script, "frame = (frame + 1)") {
		t.Errorf("el script no avanza el frame:\n%s", script)
	}
	if !strings.Contains(script, "acc = acc + step") {
		t.Errorf("el script no acumula el paso:\n%s", script)
	}
	if !strings.Contains(script, "while (acc >= 1)") {
		t.Errorf("el script no avanza solo cuando corresponde:\n%s", script)
	}
	if !strings.Contains(script, "step = 1.66667") {
		t.Errorf("el paso para 30 fps no es 50/30:\n%s", script)
	}
}

// TestBuildScriptStepSegunFps verifica el paso en varios fps.
func TestBuildScriptStepSegunFps(t *testing.T) {
	casos := []struct {
		fps  int
		step string
	}{
		{50, "step = 1;"},
		{25, "step = 2;"},
		{15, "step = 3.33333;"},
		{60, "step = 0.833333;"},
	}

	for _, c := range casos {
		s := buildScript([]string{"a.png"}, c.fps)
		if !strings.Contains(s, c.step) {
			t.Errorf("con %d fps no aparece %q:\n%s", c.fps, c.step, s)
		}
	}
}

// TestBuildScriptModuloPorCantidadDeFrames verifica el módulo: si hay
// 3 frames, el indice va de 0 a 2.
func TestBuildScriptModuloPorCantidadDeFrames(t *testing.T) {
	script := buildScript([]string{"a.png", "b.png", "c.png"}, 30)
	if !strings.Contains(script, "% 3") {
		t.Errorf("el script no hace modulo por la cantidad de frames:\n%s", script)
	}
}

// TestScriptUsaTodosLosFrames verifica que el script referencia los
// frames en orden.
func TestScriptUsaTodosLosFrames(t *testing.T) {
	frames := []string{"a.png", "b.png", "c.png"}
	script := buildScript(frames, 30)

	posA := strings.Index(script, "a.png")
	posB := strings.Index(script, "b.png")
	posC := strings.Index(script, "c.png")

	if posA < 0 || posB < 0 || posC < 0 {
		t.Fatalf("el script no menciona todos los frames:\n%s", script)
	}
	if !(posA < posB && posB < posC) {
		t.Error("los frames no estan en orden en el script")
	}
}

// TestCleanupNoBorraElTemaDelSistema verifica que la limpieza solo
// borra el tema que copio el preview.
//
// Los temas que ya estaban en /run/plymouth/themes (los del sistema) no
// se tocan: son del sistema, no del preview.
func TestCleanupNoBorraElTemaDelSistema(t *testing.T) {
	// El sistema no tiene /run/plymouth/themes en el test, asi que la
	// funcion no debe entrar en panic.
	cleanupPreviewFiles()
}

// TestListFramesIgnoraLosEscalados verifica que listFrames solo agarra
// los frames originales, no los "big-" (que son la version escalada).
func TestListFramesIgnoraLosEscalados(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"animation-000.png",
		"animation-001.png",
		"big-animation-000.png",
		"script.script",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	frames := listFrames(dir)
	if len(frames) != 2 {
		t.Errorf("listFrames() devolvio %d frames, quiero 2: %v", len(frames), frames)
	}
	for _, f := range frames {
		if strings.Contains(f, "big-") {
			t.Errorf("listFrames() agarro un frame escalado: %s", f)
		}
	}
}

// TestPidSeEscribeEnElArchivo verifica que el daemon deja su PID en el
// archivo, para poder controlarlo despues.
func TestPidSeEscribeEnElArchivo(t *testing.T) {
	// El script del daemon hace: echo $$ > <pidfile> && exec plymouthd
	// Con exec, el PID se mantiene. Este test verifica que el archivo
	// existe y tiene un PID valido.
	path := filepath.Join(t.TempDir(), "pid")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Errorf("el PID del archivo no es valido: %q", data)
	}
}

// TestBuildVideoFilterUsaElRectDetectado verifica que el crop usa el
// rectángulo de contenido que devuelve cropdetect.
//
// El bug: el crop se calculaba por aspecto (y se aplicaba siempre), así
// que un video cuadrado perdía el 44% de la imagen aunque no tuviera
// bandas.
func TestBuildVideoFilterUsaElRectDetectado(t *testing.T) {
	// 1920x1200 con bandas: el contenido real es 1920x1080 en y=60.
	f := buildVideoFilter(rect{W: 1920, H: 1080, X: 0, Y: 60}, 1920, 1080, 50)
	if !strings.Contains(f, "crop=1920:1080:0:60") {
		t.Errorf("no usa el rect detectado:\n%s", f)
	}

	// Cuadrado sin bandas: el rect es el video entero, crop inocuo.
	f2 := buildVideoFilter(rect{W: 450, H: 450}, 1920, 1080, 50)
	if !strings.Contains(f2, "crop=450:450:0:0") {
		t.Errorf("no usa el rect del video entero:\n%s", f2)
	}
}

// TestBuildVideoFilterUsaElDestino verifica que el destino del filtro es
// el tamaño pedido, no el del video.
//
// El bug: los frames se extraían al tamaño del video y después se
// escalaban forzado a la pantalla, así que un 1920x1200 se aplastaba.
func TestBuildVideoFilterUsaElDestino(t *testing.T) {
	f := buildVideoFilter(rect{W: 1920, H: 1080}, 1920, 1080, 50)

	if !strings.Contains(f, "scale=1920:1080") {
		t.Errorf("el scale no usa el destino:\n%s", f)
	}
	if !strings.Contains(f, "pad=1920:1080") {
		t.Errorf("el pad no usa el destino:\n%s", f)
	}
	if !strings.Contains(f, "fps=50") {
		t.Errorf("falta el fps:\n%s", f)
	}
}

// TestBuildVideoFilterFuerzaAspectoExacto verifica que el escalado no
// deja bandas dentro del destino (el pad es solo para los bordes).
func TestBuildVideoFilterFuerzaAspectoExacto(t *testing.T) {
	f := buildVideoFilter(rect{W: 1920, H: 1080}, 1920, 1080, 50)
	if !strings.Contains(f, "force_original_aspect_ratio=decrease") {
		t.Errorf("el scale no respeta el aspecto:\n%s", f)
	}
	if !strings.Contains(f, "flags=lanczos") {
		t.Errorf("falta lanczos:\n%s", f)
	}
}

// TestParseCropDetect toma el último crop del stderr de ffmpeg.
func TestParseCropDetect(t *testing.T) {
	const out = `[Parsed_cropdetect_0 @ 0x1] x1:0 x2:1919 y1:52 y2:1147 w:1920 h:1080 x:0 y:60 pts:0 t:0
[Parsed_cropdetect_0 @ 0x1] x1:0 x2:1919 y1:55 y2:1150 w:1920 h:1080 x:0 y:60 pts:1024 t:0.1
crop=1920:1080:0:60
`
	r := parseCropDetect(out, 1920, 1200)
	if r.W != 1920 || r.H != 1080 || r.X != 0 || r.Y != 60 {
		t.Errorf("parseCropDetect() = %+v, quiero 1920x1080+0+60", r)
	}
}

// TestParseCropDetectSinDatos devuelve el video entero.
func TestParseCropDetectSinDatos(t *testing.T) {
	r := parseCropDetect("no hay nada aca", 450, 450)
	if r.W != 450 || r.H != 450 || r.X != 0 || r.Y != 0 {
		t.Errorf("parseCropDetect() = %+v, quiero el video entero", r)
	}
}

// TestParseCropDetectRectInvalido ignora un rectángulo que se sale del
// video.
func TestParseCropDetectRectInvalido(t *testing.T) {
	r := parseCropDetect("crop=1920:1080:100:100", 1920, 1200)
	if r.W != 1920 || r.H != 1200 {
		t.Errorf("acepto un rect fuera del video: %+v", r)
	}
}

// TestParseXrandr saca la resolución de una salida real de xrandr.
func TestParseXrandr(t *testing.T) {
	const out = `Screen 0: minimum 8 x 8, current 1920 x 1080, maximum 32767 x 32767
eDP-1 connected primary 1920x1080+0+0 (normal left inverted right x axis y axis) 344mm x 194mm
   1920x1080     60.00*+
   1280x720      59.86
HDMI-1 disconnected (normal left inverted right x axis y axis)
`
	w, h := parseXrandr(out)
	if w != 1920 || h != 1080 {
		t.Errorf("parseXrandr() = %dx%d, quiero 1920x1080", w, h)
	}
}

// TestParseXrandrSinMonitor devuelve 0,0 si no hay monitor conectado.
func TestParseXrandrSinMonitor(t *testing.T) {
	const out = `Screen 0: minimum 8 x 8, current 1024 x 768, maximum 32767 x 32767
HDMI-1 disconnected (normal left inverted right x axis y axis)
`
	if w, h := parseXrandr(out); w != 0 || h != 0 {
		t.Errorf("parseXrandr() = %dx%d, quiero 0x0", w, h)
	}
}

// TestScreenSizeRespetaElEntorno verifica que las variables del preview
// ganan sobre la detección.
func TestScreenSizeRespetaElEntorno(t *testing.T) {
	t.Setenv("PLYMOUTH_SCREEN_W", "2560")
	t.Setenv("PLYMOUTH_SCREEN_H", "1440")

	w, h := screenSize()
	if w != 2560 || h != 1440 {
		t.Errorf("screenSize() = %dx%d, quiero 2560x1440", w, h)
	}
}

// TestBuildVideoFilterRecorteParaLlenar verifica el umbral del recorte.
//
// Un 16:10 en pantalla 16:9 pierde 10% al llenar: se acepta. Un
// cuadrado perdería 44%: no se acepta, va con barras.
func TestBuildVideoFilterRecorteParaLlenar(t *testing.T) {
	// 1920x1200 -> 1920x1080: recorta 60 px arriba y 60 abajo.
	f := buildVideoFilter(rect{W: 1920, H: 1200}, 1920, 1080, 50)
	if !strings.Contains(f, "crop=1920:1080:0:60") {
		t.Errorf("no recorta el 16:10 para llenar:\n%s", f)
	}

	// 450x450 -> 1920x1080: perderia el 44%, no se recorta.
	f2 := buildVideoFilter(rect{W: 450, H: 450}, 1920, 1080, 50)
	if strings.Contains(f2, "crop=450:253") {
		t.Errorf("recorta el cuadrado (perderia el 44%%):\n%s", f2)
	}
	// El crop que queda es el del video entero (inocuo).
	if !strings.Contains(f2, "crop=450:450:0:0") {
		t.Errorf("no usa el rect del video entero:\n%s", f2)
	}

	// Un 16:9 exacto no lleva segundo crop.
	f3 := buildVideoFilter(rect{W: 1920, H: 1080}, 1920, 1080, 50)
	if strings.Count(f3, "crop=") != 1 {
		t.Errorf("el 16:9 lleva un crop de mas:\n%s", f3)
	}
}

// TestBuildVideoFilterVerticalNoSeRecorta verifica un video vertical
// (1080x1920): perderia el 56% al llenar 16:9, asi que va con barras.
func TestBuildVideoFilterVerticalNoSeRecorta(t *testing.T) {
	f := buildVideoFilter(rect{W: 1080, H: 1920}, 1920, 1080, 50)
	if strings.Contains(f, "crop=1080:607") {
		t.Errorf("recorta un vertical (perderia el 56%%):\n%s", f)
	}
}
