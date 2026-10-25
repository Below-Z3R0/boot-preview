package pack

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── Bundle ──────────────────────────────────────────────────────────────────

// Bundle arma el paquete descargable: dist/boot-preview.tar.gz.
//
// El paquete lleva el binario y los assets (los ejemplos que usa el
// instalador). Es lo que se descarga el usuario con un solo comando:
//
//	curl -fsSL .../releases/latest/download/boot-preview.tar.gz | tar xz
//	&& ./boot-preview/boot-preview install
func bundle(paths cli.Paths) error {
	cli.Step("Armando el paquete")

	if paths.Repo == "" {
		return fmt.Errorf("no encuentro el repo")
	}

	dist := filepath.Join(paths.Repo, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}

	stage, err := os.MkdirTemp("", "boot-preview-bundle.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	root := filepath.Join(stage, "boot-preview")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		return err
	}

	// El binario.
	if err := cli.CopyFile(self, filepath.Join(root, "boot-preview")); err != nil {
		return err
	}

	// Los assets: los ejemplos que usa el instalador.
	examples := filepath.Join(paths.Repo, "examples")
	if err := copyTree(examples, filepath.Join(root, "assets")); err != nil {
		return err
	}

	// El instalador, que el usuario corre tras descomprimir.
	// El README del bundle: cómo instalarlo.
	readme := `# boot-preview

Previsualiza el arranque de Linux sin reiniciar.

## Instalar

    ./boot-preview install

Para instalar solo una parte:

    ./boot-preview install loader     # el menú de Limine
    ./boot-preview install animation  # el splash de Plymouth

## Verificar

    ./boot-preview verify

## Usar

    ./boot-preview loader      # el menú de Limine
    ./boot-preview animation   # el splash de Plymouth
`
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}

	out := filepath.Join(dist, "boot-preview.tar.gz")
	if err := writeTarGz(out, root); err != nil {
		return err
	}

	size := "?"
	if info, err := os.Stat(out); err == nil {
		size = fmt.Sprintf("%.0f KB", float64(info.Size())/1024)
	}
	cli.Ok("%s (%s)", out, size)
	return nil
}

// writeTarGz escribe un directorio como tar.gz.
func writeTarGz(dest, root string) error {
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	base := filepath.Dir(root)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		// Timestamp fijo: el build es reproducible.
		header.ModTime = fixedTime()
		header.AccessTime = fixedTime()
		header.ChangeTime = fixedTime()
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			src, err := os.Open(path)
			if err != nil {
				return err
			}
			defer src.Close()
			if _, err := io.Copy(tw, src); err != nil {
				return err
			}
		}
		return nil
	})
}

// fixedTime es el timestamp fijo del build reproducible: dos builds del
// mismo código dan el mismo tarball.
func fixedTime() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

// copyTree copia un directorio entero.
func copyTree(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			if err := copyTree(srcPath, destPath); err != nil {
				return err
			}
			continue
		}
		if err := cli.CopyFile(srcPath, destPath); err != nil {
			return err
		}
	}
	return nil
}
