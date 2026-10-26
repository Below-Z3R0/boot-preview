package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── El binario y los assets ──────────────────────────────────────────────────

func installBinaries(paths cli.ToolPaths, wantLoader, wantAnimation bool) error {
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
	// Vienen de dos lugares:
	//   - del repo (desarrollo): examples/
	//   - del directorio del binario (instalado desde el bundle): assets/
	//
	// El bundle trae los assets al lado del binario, así que si se
	// instala desde ahí no hay repo de donde sacarlos.
	assets := filepath.Join(paths.Lib, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return err
	}

	// Del repo si tiene examples. Si no (repo a medias, o instalado
	// desde el bundle), al lado del binario que se está ejecutando.
	origen := ""
	if paths.Repo != "" && dirExists(filepath.Join(paths.Repo, "examples")) {
		origen = filepath.Join(paths.Repo, "examples")
	}
	if origen == "" {
		if exe, err := os.Executable(); err == nil {
			candidata := filepath.Join(filepath.Dir(exe), "assets")
			if dirExists(candidata) {
				origen = candidata
			}
		}
	}

	if origen == "" {
		// Sin assets, `install loader` fallaba más adelante con un "no
		// such file" del copiado de loader.conf, dejando el binario y el
		// shim instalados a medias. Se corta acá.
		if wantLoader {
			return fmt.Errorf("faltan los assets (loader.conf, wallpaper):\n" +
				"  buscá un repo con examples/ o un bundle con assets/ junto al binario")
		}
		cli.Warn("No encuentro los assets: solo se instala el módulo de Plymouth")
	} else {
		if err := cli.CopyDir(origen, assets); err != nil {
			return err
		}
		cli.Ok("%s (desde %s)", assets, origen)
	}

	// Un shim en /usr/local/bin para que `sudo boot-preview` funcione:
	// ~/.local/bin no está en el secure_path de sudo.
	return installSudoShim(paths)
}

// installBinary copia el binario compilado a su destino.
func installBinary(paths cli.ToolPaths, name, dest string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}

	// Si el ejecutable actual ES el destino, copiarlo da "text file busy":
	// el kernel no deja escribir un binario que se está corriendo.
	//
	// Se copia a un temporal y se mueve: `mv` dentro del mismo sistema de
	// archivos es un rename, que sí funciona con el binario en uso.
	if cli.SamePath(self, dest) {
		tmp, err := os.CreateTemp(filepath.Dir(dest), ".boot-preview-install.*")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()
		tmp.Close()
		defer os.Remove(tmpName)

		data, err := os.ReadFile(self)
		if err != nil {
			return err
		}
		if err := os.WriteFile(tmpName, data, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmpName, dest); err != nil {
			return err
		}
		return os.Chmod(dest, 0o755)
	}

	// Si ya estamos instalados, el ejecutable actual es el bueno.
	if err := cli.CopyFile(self, dest); err != nil {
		return err
	}
	return os.Chmod(dest, 0o755)
}

// installSudoShim deja un wrapper en /usr/local/bin.
//
// El path del binario se embebe al instalar: dentro de sudo, $HOME es
// /root, asi que "${HOME}/.local/bin/boot-preview" no existe. Antes el
// fallback era un path hardcodeado al home de un usuario en particular.
func installSudoShim(paths cli.ToolPaths) error {
	script := filepath.Join(paths.Bin, "boot-preview")

	shim := sudoShim(script)

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

// dirExists dice si el path existe y es un directorio.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// sudoShim arma el wrapper de /usr/local/bin.
//
// Va aparte para poder testearlo: installSudoShim escribe en
// /usr/local/bin, que es del sistema.
func sudoShim(script string) string {
	// %q deja el path entre comillas y escapado para bash.
	return fmt.Sprintf(`#!/usr/bin/env bash
# Wrapper de boot-preview para poder correrlo con sudo.
# 'sudo boot-preview' falla con "command not found" porque ~/.local/bin
# no esta en el secure_path de sudo. Si lo invoca root, baja al usuario
# que lo lanzo: QEMU no debe correr como root (no tiene acceso al socket
# grafico).
set -euo pipefail
script=%q
if [[ "$(id -u)" == "0" && -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
  exec sudo -u "$SUDO_USER" "$script" "$@"
fi
exec "$script" "$@"
`, script)
}
