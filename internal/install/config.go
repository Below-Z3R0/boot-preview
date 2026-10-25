package install

import (
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── La config ────────────────────────────────────────────────────────────────

func installConfig(paths cli.Paths, wantLoader, wantAnimation bool) error {
	cli.Step("3/4  La config")

	if wantLoader {
		loaderDir := paths.LoaderConfig()
		if err := os.MkdirAll(filepath.Join(loaderDir, "wallpaper"), 0o755); err != nil {
			return err
		}

		// La config de prueba: se instala desde el asset, pero si el
		// usuario ya tenía una se guarda como .bak (no se pisa).
		conf := filepath.Join(loaderDir, "limine.conf")
		if _, err := os.Stat(conf); err == nil {
			_ = cli.CopyFile(conf, conf+".bak")
			cli.Say("hay una config previa: guardada como limine.conf.bak")
		}
		src := filepath.Join(paths.Lib, "assets", "loader.conf")
		if err := cli.CopyFile(src, conf); err != nil {
			return err
		}
		cli.Ok("%s", conf)

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
