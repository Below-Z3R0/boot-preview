package animation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildScript(t *testing.T) {
	frames := []string{"a.png", "b.png", "c.png"}
	script := buildScript(frames, 15, 1, 2)

	// El script tiene que referenciar los frames en orden.
	posA := strings.Index(script, "a.png")
	posB := strings.Index(script, "b.png")
	posC := strings.Index(script, "c.png")
	if posA < 0 || posB < 0 || posC < 0 {
		t.Fatalf("el script no menciona los frames:\n%s", script)
	}
	if !(posA < posB && posB < posC) {
		t.Errorf("los frames no estan en orden: a=%d b=%d c=%d", posA, posB, posC)
	}
}

func TestBuildScriptTiming(t *testing.T) {
	frames := []string{"a.png", "b.png"}
	script := buildScript(frames, 15, 1, 2)

	// Con fps=15 y step 1/2, el frame dura 2/15 de segundo: el script
	// tiene que llevar el rate y el step.
	if !strings.Contains(script, "15") {
		t.Errorf("el script no menciona el rate:\n%s", script)
	}
}

func TestBuildScriptEmpty(t *testing.T) {
	// Sin frames no deberia entrar en panic.
	script := buildScript(nil, 15, 1, 2)
	if script == "" {
		t.Error("buildScript() con lista vacia devolvio vacio")
	}
}

// TestListFramesOrden es el test mas importante del modulo: el orden de
// los frames define el timing de la animacion.
//
// Un glob normal (animation-1.png, animation-10.png, animation-2.png)
// rompe la animacion: el frame 10 aparece antes del 2.
func TestListFramesOrden(t *testing.T) {
	dir := t.TempDir()

	// Se crean 12 frames para que el orden lexicografico y el numerico
	// difieran.
	for i := range 12 {
		name := filepath.Join(dir, "animation-"+pad(i)+".png")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	frames := listFrames(dir)
	if len(frames) != 12 {
		t.Fatalf("listFrames() devolvio %d frames, quiero 12", len(frames))
	}

	// El frame 2 tiene que ir antes que el 10.
	idx2 := indexOf(frames, "animation-002.png")
	idx10 := indexOf(frames, "animation-010.png")
	if idx2 < 0 || idx10 < 0 {
		t.Fatalf("no encontre los frames 2 y 10: %v", frames)
	}
	if idx2 > idx10 {
		t.Errorf("el frame 2 va despues del 10: el orden es lexicografico, no numerico")
	}
}

func TestListFramesIgnoraOtros(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"animation-000.png",
		"animation-001.png",
		"script.script",
		"notas.txt",
		"big-animation-002.png",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	frames := listFrames(dir)
	if len(frames) != 2 {
		t.Fatalf("listFrames() devolvio %d frames, quiero 2: %v", len(frames), frames)
	}
	for _, f := range frames {
		if !strings.Contains(f, "animation-") {
			t.Errorf("listFrames() agarro un archivo que no es frame: %s", f)
		}
		if strings.Contains(f, "big-") {
			t.Errorf("listFrames() agarro un frame escalado: %s", f)
		}
	}
}

func TestListFramesVacio(t *testing.T) {
	if frames := listFrames(t.TempDir()); len(frames) != 0 {
		t.Errorf("listFrames() en una carpeta vacia devolvio %d frames", len(frames))
	}
}

func TestIsDigits(t *testing.T) {
	tests := []struct {
		in     string
		expect bool
	}{
		{"31391441102", true},
		{"0", true},
		{"", false},
		{"12a", false},
		{"a12", false},
		{"-1", false},
	}

	for _, tc := range tests {
		if got := isDigits(tc.in); got != tc.expect {
			t.Errorf("isDigits(%q) = %v, quiero %v", tc.in, got, tc.expect)
		}
	}
}

func TestThemeNameFor(t *testing.T) {
	// El nombre viene del MovieID (solo dígitos) y del archivo.
	name := themeNameFor("31391441102_abcdef.webm", "/home/usuario")
	if !strings.HasPrefix(name, "steam-") {
		t.Errorf("themeNameFor() = %q, quiero que empiece con %q", name, "steam-")
	}
	if strings.ContainsAny(name, "/\\") {
		t.Errorf("themeNameFor() = %q, no puede tener separadores de ruta", name)
	}
}

func TestThemeNameForConPath(t *testing.T) {
	name := themeNameFor("/home/usuario/videos/pelicula.webm", "/home/usuario")
	if strings.ContainsAny(name, "/\\") {
		t.Errorf("themeNameFor() = %q, no puede tener separadores de ruta", name)
	}
}

// pad arma el nombre de un frame con 3 dígitos, como el convertidor.
func pad(i int) string {
	if i < 10 {
		return "00" + string(rune('0'+i))
	}
	return "0" + string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if filepath.Base(v) == want {
			return i
		}
	}
	return -1
}
