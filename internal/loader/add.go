package loader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
	"github.com/Below-Z3R0/boot-preview/internal/wallpaper"
)

// ── add / edit ──────────────────────────────────────────────────────────────

// cmdAdd pone una imagen como wallpaper del preview.
func cmdAdd(opts options, paths cli.Paths) error {
	if _, err := os.Stat(opts.addImage); err != nil {
		return fmt.Errorf("no existe: %s", opts.addImage)
	}

	wallDir := filepath.Join(paths.LoaderConfig(), "wallpaper")
	if err := os.MkdirAll(wallDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(wallDir, "wallpaper.png")

	// Limine no puede difuminar: dibuja la imagen tal cual. Si el blur
	// está pedido, se aplica ANTES, al preparar el fichero.
	if opts.blur != "0x0" || opts.dim > 0 || opts.overlay != "" {
		cli.Step("Preparando el wallpaper")
		if err := wallpaper.Prepare(wallpaper.Options{
			Src:     opts.addImage,
			Out:     target,
			Blur:    opts.blur,
			Res:     opts.resolution,
			Format:  opts.format,
			Dim:     opts.dim,
			Overlay: opts.overlay,
		}); err != nil {
			return err
		}
	} else if err := cli.CopyFile(opts.addImage, target); err != nil {
		return err
	}

	cli.Ok("%s  ->  %s", opts.addImage, target)
	fmt.Println()
	cli.Say("Para verlo:  boot-preview loader")
	cli.Say("Para aplicarlo al Limine real:  boot-preview loader apply wallpaper")
	return nil
}

// cmdEdit abre la config del preview en un editor.
func cmdEdit(opts options, paths cli.Paths) error {
	editor := os.Getenv("VISUAL_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		for _, candidate := range []string{"code", "nvim", "micro", "nano", "vim", "helix", "zed"} {
			if cli.HaveCommand(candidate) {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no encuentro un editor (proba con EDITOR=...)")
	}

	conf := filepath.Join(paths.LoaderConfig(), "limine.conf")
	cli.Say("Abriendo %s con %s", conf, editor)

	cmd := exec.Command(editor, conf)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
