package animation

import (
	"fmt"
	"path/filepath"
	"strings"
)

func buildScript(frames []string, fps int) string {
	var b strings.Builder

	// Sin frames no hay animacion: se devuelve un script vacio en vez de
	// entrar en panic con frames[0].
	if len(frames) == 0 {
		return "# Sin frames: no hay nada que reproducir.\n"
	}

	b.WriteString("# Animación generada desde un video de Steam.\n")
	b.WriteString("# Plugin \"script\" de Plymouth: loop libre, no atado al boot real.\n")
	b.WriteString("#\n")
	fmt.Fprintf(&b, "# Timing: Plymouth refresca a %d Hz y el video va a %d fps.\n", refreshRate, fps)
	fmt.Fprintf(&b, "# El script acumula (step = %d/%d) y avanza 0, 1 o 2 frames por\n", refreshRate, fps)
	b.WriteString("# refresh, asi la duracion es exacta sin duplicar frames.\n")
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
	b.WriteString("acc = 0;\n")
	// Cuántos frames de video tocan por cada refresh de Plymouth. Con
	// 50 Hz y un video de 30 fps son 1.667: algunos refreshes avanzan 1
	// frame y otros 2, así la duración es exacta y no hace falta
	// duplicar frames al extraer.
	step := float64(refreshRate) / float64(fps)
	if fps <= 0 || step <= 0 {
		step = 1
	}
	fmt.Fprintf(&b, "step = %.6g;\n", step)
	b.WriteString("\n")
	b.WriteString("fun Refresh ()\n")
	b.WriteString("{\n")
	b.WriteString("  acc = acc + step;\n")
	b.WriteString("  while (acc >= 1)\n")
	b.WriteString("  {\n")
	fmt.Fprintf(&b, "    frame = (frame + 1) %% %d;\n", len(frames))
	b.WriteString("    acc = acc - 1;\n")
	b.WriteString("  }\n")
	b.WriteString("  sprite.SetImage(Image(frames[frame]));\n")
	b.WriteString("}\n")
	b.WriteString("\n")
	b.WriteString("Plymouth.SetRefreshFunction(Refresh);\n")

	return b.String()
}

// detectVideoProps saca el fps, el ancho y el alto del video.
//
// ffprobe devuelve los campos en orden alfabético, así que se pide cada
// uno por separado: el orden no se asume.
