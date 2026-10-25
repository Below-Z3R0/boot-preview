package install

import (
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── El binario y los assets ──────────────────────────────────────────────────

func installBinaries(paths cli.Paths, wantLoader, wantAnimation bool) error {
	cli.Step("2/4  Los scripts")

	if err := os.MkdirAll(paths.Bin, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.Lib, 0o755); err != nil {
		return err
	}

	// El dispatcher: un solo comando, `boot-preview`.
	if err := installBinary(paths, "boot-preview", filepath.Join(paths.Bin, "boot-preview")); err != nil {
		return err
	}
	cli.Ok("%s", filepath.Join(paths.Bin, "boot-preview"))

	// Los recursos (los ejemplos que el instalador usa) van junto al
	// binario, en lib/.
	//
	// Si no hay repo (el tool se instaló desde el bundle), los assets ya
	// están instalados y no hay nada que copiar.
	assets := filepath.Join(paths.Lib, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return err
	}
	if paths.Repo != "" {
		if err := cli.CopyDir(filepath.Join(paths.Repo, "examples"), assets); err != nil {
			return err
		}
		cli.Ok("%s", assets)
	} else {
		cli.Ok("%s (ya instalados)", assets)
	}

	// Un shim en /usr/local/bin para que `sudo boot-preview` funcione:
	// ~/.local/bin no está en el secure_path de sudo.
	return installSudoShim(paths)
}

// installBinary copia el binario compilado a su destino.
func installBinary(paths cli.Paths, name, dest string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	// Si ya estamos instalados, el ejecutable actual es el bueno.
	if err := cli.CopyFile(self, dest); err != nil {
		return err
	}
	return os.Chmod(dest, 0o755)
}

// installSudoShim deja un wrapper en /usr/local/bin.
func installSudoShim(paths cli.Paths) error {
	const shim = `#!/usr/bin/env bash
# Wrapper de boot-preview para poder correrlo con ` + "`sudo`" + `.
# ` + "`sudo boot-preview`" + ` falla con "command not found" porque ~/.local/bin
# no está en el secure_path de sudo. Si lo invoca root, baja al usuario que
# lo lanzó: QEMU no debe correr como root (no tiene acceso al socket gráfico).
set -euo pipefail
script="${HOME}/.local/bin/boot-preview"
if [[ ! -x "$script" ]]; then
  script="/home/b3low-z3r0/.local/bin/boot-preview"
fi
if [[ "$(id -u)" == "0" && -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
  exec sudo -u "$SUDO_USER" "$script" "$@"
fi
exec "$script" "$@"
`

	tmp, err := os.CreateTemp("", "boot-preview-shim.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(shim); err != nil {
		return err
	}
	tmp.Close()

	dest := "/usr/local/bin/boot-preview"
	if out, err := cli.RunCombined("sudo", "install", "-m", "755", tmp.Name(), dest); err != nil {
		cli.Warn("no se pudo instalar el shim en /usr/local/bin (sudo?)")
		cli.Say("%s", out)
		return nil
	}
	cli.Ok("%s (shim para sudo)", dest)
	return nil
}

// installConfig crea la configuración de lo instalado.
