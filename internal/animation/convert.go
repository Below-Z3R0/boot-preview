// La conversión de un video de Steam en un tema de Plymouth.
//
// El video se recorta al aspecto de la pantalla, se escala y se saca
// frame a frame. El script de Plymouth reproduce esos frames.
//
//	go: cmdSteamConvert   el comando
//	steam.go              el parser de los videos de Steam
package animation

import (
	"fmt"
	"os"
	"path/filepath"

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
