package animation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// TestDetectContentRectRealDistingueBandas verifica el detector contra
// videos reales: uno con bandas negras y uno cuadrado sin bandas.
//
// Es la prueba que decide el comportamiento del filtro: si esto se
// equivoca, se pierde imagen (cuadrado) o quedan bandas (16:10).
func TestDetectContentRectRealDistingueBandas(t *testing.T) {
	if !cli.HaveCommand("ffmpeg") || !cli.HaveCommand("ffprobe") {
		t.Skip("sin ffmpeg")
	}

	dir := t.TempDir()

	// 1920x1200 con el contenido (blanco) a 1920x1080 en el centro:
	// bandas negras de 60 px arriba y abajo.
	conBandas := filepath.Join(dir, "bandas.webm")
	cmd := exec.Command("ffmpeg", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "color=black:s=1920x1200:d=0.5",
		"-vf", "drawbox=x=0:y=60:w=1920:h=1080:color=white:t=fill",
		"-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", conBandas)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("no se pudo generar el video: %s", out)
	}

	r := detectContentRect(conBandas, 1920, 1200)
	if r.H >= 1200 {
		t.Errorf("no detecto las bandas: %+v (quiero alto ~1080)", r)
	}
	if r.Y < 40 || r.Y > 80 {
		t.Errorf("el offset vertical no es el de las bandas: %+v", r)
	}

	// 450x450 sin bandas: el rect tiene que ser el video entero.
	cuadrado := filepath.Join(dir, "cuadrado.webm")
	cmd2 := exec.Command("ffmpeg", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=450x450:duration=0.5:rate=10",
		"-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", cuadrado)
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Skipf("no se pudo generar el video cuadrado: %s", out)
	}

	r2 := detectContentRect(cuadrado, 450, 450)
	if r2.W != 450 || r2.H != 450 {
		t.Errorf("recorto un video sin bandas: %+v (quiero 450x450)", r2)
	}

	// Y el frame final no puede quedar recortado.
	filtro := buildVideoFilter(r2, 1920, 1080, 50)
	if filtro == "" {
		t.Error("filtro vacio")
	}
	_ = os.Remove(conBandas)
}
