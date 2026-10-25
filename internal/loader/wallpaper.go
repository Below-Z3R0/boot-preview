package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
	"github.com/Below-Z3R0/boot-preview/internal/wallpaper"
)

// ── wallpapers ──────────────────────────────────────────────────────────────

// cmdWallpapers genera las dos versiones del wallpaper.
//
// Limine NO puede difuminar una imagen: la dibuja tal cual. Así que el
// blur se aplica ANTES, y lo que llega a Limine ya viene difuminado.
func cmdWallpapers(opts options, paths cli.ToolPaths) error {
	wallDir := filepath.Join(paths.LoaderConfig(), "wallpaper")
	src := filepath.Join(wallDir, "rei.png")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("no hay wallpaper de ejemplo en %s", src)
	}

	cli.Step("La real (más detalle, sin blur)")
	real := filepath.Join(wallDir, "rei.png")
	if err := wallpaper.Prepare(wallpaper.Options{Src: src, Out: real, Blur: "0x0", Format: "png"}); err != nil {
		return err
	}

	cli.Step("La difuminada (ya viene con el blur aplicado)")
	blurred := filepath.Join(wallDir, "rei-blur.png")
	if err := wallpaper.Prepare(wallpaper.Options{Src: src, Out: blurred, Blur: "0x30", Format: "png"}); err != nil {
		return err
	}

	fmt.Println()
	cli.Say("  %s", real)
	cli.Say("  %s", blurred)
	fmt.Println()
	cli.Say("  La difuminada quedo como la activa (wallpaper.png):")
	cli.Say("      boot-preview loader                  → preview con ella")
	cli.Say("      boot-preview loader apply            → wallpaper + config al Limine real")
	fmt.Println()
	cli.Say("  Para volver a la nitida:")
	cli.Say("      cp %s %s", real, filepath.Join(wallDir, "wallpaper.png"))
	cli.Say("      boot-preview loader apply wallpaper")
	return nil
}
