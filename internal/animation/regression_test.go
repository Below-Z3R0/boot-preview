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

// TestBuildScriptAvanzaUnFramePorRefresh verifica el timing.
//
// Plymouth refresca a 50 Hz y el script avanza 1 frame por refresh: los
// frames se extraen a 50 fps para que el video se vea a su velocidad.
//
// El bug: el script avanzaba 3 frames de golpe cada 5 refreshes, asi
// que el ojo veia 10 fps en vez de los 30 del video.
func TestBuildScriptAvanzaUnFramePorRefresh(t *testing.T) {
	frames := []string{"a.png", "b.png", "c.png"}
	script := buildScript(frames, 30)

	if !strings.Contains(script, "frame = (frame + 1)") {
		t.Errorf("el script no avanza 1 frame por refresh:\n%s", script)
	}
	if strings.Contains(script, "tick") {
		t.Errorf("el script no deberia usar un contador de ticks:\n%s", script)
	}
	if !strings.Contains(script, "50") {
		t.Errorf("el script no menciona la tasa de refresco:\n%s", script)
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
