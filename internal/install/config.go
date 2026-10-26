package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── La config ────────────────────────────────────────────────────────────────

func installConfig(paths cli.ToolPaths, wantLoader, wantAnimation bool) error {
	cli.Step("3/4  La config")

	if wantLoader {
		loaderDir := paths.LoaderConfig()
		if err := os.MkdirAll(filepath.Join(loaderDir, "wallpaper"), 0o755); err != nil {
			return err
		}

		// La config de prueba: se instala desde el asset SOLO si no hay
		// una. Si el usuario ya la tenía (la editó con `loader edit`), se
		// deja como está.
		//
		// Antes se hacía un .bak y se pisaba igual: cada `install` o
		// `update` borraba el tema que el usuario había configurado.
		conf := filepath.Join(loaderDir, "limine.conf")
		if _, err := os.Stat(conf); err == nil {
			cli.Ok("%s (ya estaba, no se toca)", conf)
		} else {
			src := filepath.Join(paths.Lib, "assets", "loader.conf")
			if _, err := os.Stat(src); err != nil {
				return fmt.Errorf("falta el asset %s: instalá desde el repo o desde el bundle", src)
			}
			if err := cli.CopyFile(src, conf); err != nil {
				return err
			}
			cli.Ok("%s", conf)
		}

		// El wallpaper de ejemplo.
		rei := filepath.Join(loaderDir, "wallpaper", "rei.png")
		if _, err := os.Stat(rei); err != nil {
			if err := cli.CopyFile(filepath.Join(paths.Lib, "assets", "wallpaper", "rei.png"), rei); err != nil {
				return err
			}
			cli.Ok("%s", rei)
		} else {
			cli.Ok("%s (ya estaba)", rei)
		}

		// La ACTIVA: la que usan el preview y `apply`. Si no existe, se
		// copia la nítida; la difuminada la genera `loader wallpapers`.
		active := filepath.Join(loaderDir, "wallpaper", "wallpaper.png")
		if _, err := os.Stat(active); err != nil {
			if err := cli.CopyFile(rei, active); err != nil {
				return err
			}
			cli.Ok("%s (la activa, copiada de rei.png)", active)
		} else {
			cli.Ok("%s (la activa, ya estaba)", active)
		}
	}

	if wantAnimation {
		if err := os.MkdirAll(paths.AnimationConfig(), 0o755); err != nil {
			return err
		}
		cli.Ok("%s", paths.AnimationConfig())
	}

	// La ruta del repo: la usa el CLI para saber dónde van las capturas.
	//
	// Si no hay repo (instalación desde el bundle), las capturas van a
	// ~/.local/share/boot-preview/previews.
	repoFile := filepath.Join(paths.Config, "repo")
	repo := paths.Repo
	if repo == "" {
		repo = paths.PreviewsRoot()
	}
	if err := os.WriteFile(repoFile, []byte(repo+"\n"), 0o644); err != nil {
		return err
	}
	cli.Ok("%s -> %s", repoFile, repo)

	// Las carpetas de capturas.
	for _, dir := range []string{paths.LoaderPreviews(), paths.AnimationPreviews()} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	cli.Ok("%s", filepath.Join(repo, "previews", "{loader,animation}"))

	return nil
}

// writeState guarda el snapshot de versiones de las dependencias.
