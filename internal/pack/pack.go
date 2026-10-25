// Package pack maneja el empaquetado y el mantenimiento del tool:
// el bundle descargable, la actualización y el hook de pacman.
package pack

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// remoteURL es el repo del que salen las actualizaciones.
const remoteURL = "https://github.com/Below-Z3R0/boot-preview"

// ── Update ──────────────────────────────────────────────────────────────────

// Update trae la última versión del tool.
//
// Hay dos fuentes: si hay un repo local con remote, se actualiza con git
// (y se reinstalan los scripts). Si no hay repo, se descarga el binario
// suelto de GitHub.
func Update(paths cli.Paths) error {
	cli.Step("Buscando actualizaciones")

	if paths.Repo != "" && isGitRepo(paths.Repo) && hasRemote(paths.Repo) {
		cli.Say("Repo local: %s", paths.Repo)
		return updateFromGit(paths)
	}

	cli.Say("Sin repo local: descargando el binario de %s", remoteURL)
	return updateFromNetwork(paths)
}

// isGitRepo dice si el directorio es un repo de git.
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// hasRemote dice si el repo tiene un remote "origin".
func hasRemote(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	return cmd.Run() == nil
}

// updateFromGit hace git pull y reinstala.
func updateFromGit(paths cli.Paths) error {
	before := gitHead(paths.Repo)
	if out, err := cli.RunCombined("git", "-C", paths.Repo, "pull", "--ff-only"); err != nil {
		cli.Warn("git pull falló (¿cambios locales?)")
		cli.Say("%s", out)
		return nil
	}

	after := gitHead(paths.Repo)
	if before == after {
		cli.Ok("ya está al día (%s)", after)
		return nil
	}

	cli.Ok("actualizado: %s → %s", before, after)
	cli.Say("Reinstalando...")
	if err := reinstall(paths); err != nil {
		cli.Warn("no se pudo reinstalar: %s", err)
	}
	return nil
}

// updateFromNetwork descarga el binario de GitHub.
func updateFromNetwork(paths cli.Paths) error {
	if !cli.HaveCommand("curl") {
		cli.Warn("necesita curl")
		return fmt.Errorf("")
	}

	url := remoteURL + "/releases/latest/download/boot-preview-linux-amd64"
	tmp, err := os.CreateTemp("", "boot-preview.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	if out, err := cli.RunCombined("curl", "-fsSL", url, "-o", tmp.Name()); err != nil {
		cli.Warn("no se pudo descargar (¿sin red?)")
		cli.Say("%s", out)
		return fmt.Errorf("")
	}

	// Se comprueba que el binario funciona antes de pisar el instalado.
	if out, err := cli.RunCombined(tmp.Name(), "version"); err != nil {
		cli.Warn("el binario descargado no funciona")
		cli.Say("%s", out)
		return fmt.Errorf("")
	}

	dest := filepath.Join(paths.Bin, "boot-preview")
	if err := cli.CopyFile(tmp.Name(), dest); err != nil {
		return err
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return err
	}
	cli.Ok("binario actualizado")
	return nil
}

// reinstall reconstruye el binario y lo vuelve a instalar.
//
// El repo es Go: un git pull trae el código nuevo, pero el binario
// instalado sigue siendo el viejo. Hay que recompilar antes de instalar.
func reinstall(paths cli.Paths) error {
	if paths.Repo == "" {
		return fmt.Errorf("no hay repo")
	}

	// Si hay go.mod, se recompila.
	if _, err := os.Stat(filepath.Join(paths.Repo, "go.mod")); err == nil {
		if !cli.HaveCommand("go") {
			return fmt.Errorf("hace falta Go para recompilar (o instalá el binario de las releases)")
		}
		cli.Say("Recompilando...")
		if out, err := cli.RunCombined("go", "build", "-o", filepath.Join(paths.Repo, "dist", "boot-preview"), "./cmd/boot-preview"); err != nil {
			return fmt.Errorf("go build falló: %s", out)
		}
		cli.Ok("binario recompilado")
	}

	// La instalación copia el binario que se está ejecutando. Si se
	// recompiló, hay que instalar el nuevo.
	if _, err := os.Stat(filepath.Join(paths.Repo, "dist", "boot-preview")); err == nil {
		bin := filepath.Join(paths.Repo, "dist", "boot-preview")
		dest := filepath.Join(paths.Bin, "boot-preview")
		if err := os.MkdirAll(paths.Bin, 0o755); err != nil {
			return err
		}
		if err := cli.CopyFile(bin, dest); err != nil {
			return err
		}
		if err := os.Chmod(dest, 0o755); err != nil {
			return err
		}
		cli.Ok("instalado: %s", dest)
		return nil
	}

	return fmt.Errorf("no se encontró el binario en %s", filepath.Join(paths.Repo, "dist"))
}

// gitHead devuelve el commit corto de HEAD.
func gitHead(dir string) string {
	out, err := cli.Run("git", "-C", dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "?"
	}
	return out
}

// ── Hooks ───────────────────────────────────────────────────────────────────

// watchedPackages son los paquetes que, si se actualizan, conviene
// revalidar: el preview usa el software del sistema.
var watchedPackages = []string{
	"plymouth", "limine", "qemu-desktop", "libisoburn",
	"imagemagick", "socat", "ffmpeg", "xdotool", "xorg-xhost",
}

// hookDef es el archivo .hook que pacman lee.
//
// El Exec llama al binario con "hooks notify": los targets que dispararon
// el hook llegan por stdin, uno por línea (por el NeedsTargets).
const hookDef = `[Trigger]
Operation = Upgrade
Type = Package
Target = plymouth
Target = limine
Target = qemu-desktop
Target = libisoburn
Target = imagemagick
Target = socat
Target = ffmpeg
Target = xdotool
Target = xorg-xhost

[Action]
Description = Avisar que boot-preview puede necesitar re-verificación
When = PostTransaction
NeedsTargets
Exec = /usr/local/bin/boot-preview hooks notify
`

// Hooks instala o quita el hook de pacman.
func Hooks(args []string, paths cli.Paths) error {
	action := "install"
	if len(args) > 0 {
		action = args[0]
	}

	const (
		hookDir  = "/etc/pacman.d/hooks"
		hookFile = "/etc/pacman.d/hooks/boot-preview.hook"
	)

	switch action {
	case "install":
		cli.Step("Instalando el hook de pacman")

		// /etc es de root: se escribe con sudo.
		tmp, err := os.CreateTemp("", "boot-preview-hook.*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.WriteString(hookDef); err != nil {
			tmp.Close()
			return err
		}
		tmp.Close()

		if out, err := cli.RunCombined("sudo", "install", "-m", "644", tmp.Name(), hookFile); err != nil {
			return fmt.Errorf("no se pudo instalar el hook: %s", out)
		}
		cli.Ok("%s", hookFile)

		fmt.Println()
		cli.Say("A partir de ahora, cuando se actualice Plymouth, Limine o una")
		cli.Say("dependencia, pacman va a avisar que conviene revalidar.")
		return nil

	case "uninstall", "remove":
		cli.Step("Quitando el hook de pacman")
		if out, err := cli.RunCombined("sudo", "rm", "-f", hookFile); err != nil {
			cli.Warn("no se pudo borrar %s: %s", hookFile, out)
			return nil
		}
		cli.Ok("%s eliminado", hookFile)
		return nil

	case "status":
		cli.Step("Estado del hook de pacman")
		if _, err := os.Stat(hookFile); err != nil {
			cli.Say("no instalado (boot-preview hooks install)")
			return nil
		}
		cli.Ok("%s", hookFile)
		cli.Say("disparadores:")
		for _, line := range strings.Split(hookDef, "\n") {
			if strings.HasPrefix(line, "Target") {
				cli.Say("    %s", line)
			}
		}
		return nil

	case "notify":
		return notify()

	default:
		return fmt.Errorf("uso: boot-preview hooks [install|uninstall|status|notify]")
	}
}

// notify avisa que algo que boot-preview usa se actualizó.
//
// Lo llama el hook de pacman: los targets que lo dispararon llegan por
// stdin, uno por línea.
//
// Si boot-preview no está instalado, no dice nada: el hook corre como
// root y no tiene por qué molestar al que no lo usa.
func notify() error {
	configDir := filepath.Join(configHome(), "boot-preview")
	if _, err := os.Stat(configDir); err != nil {
		return nil
	}

	// Los targets que nos interesan.
	hit := map[string]bool{}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		target := strings.TrimSpace(scanner.Text())
		if target == "" {
			continue
		}
		for _, watched := range watchedPackages {
			if target == watched {
				hit[target] = true
			}
		}
	}
	if len(hit) == 0 {
		return nil
	}

	names := make([]string, 0, len(hit))
	for name := range hit {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("\n\033[38;2;235;194;186m▸\033[0m \033[1mboot-preview\033[0m: se actualizó %s\n", strings.Join(names, ", "))
	fmt.Printf("  \033[38;2;125;207;216m→\033[0m Corré \033[1mboot-preview verify\033[0m para revalidar los previews.\n\n")
	return nil
}

// configHome devuelve la carpeta de config del usuario.
func configHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

// ── Bundle ──────────────────────────────────────────────────────────────────

// Bundle arma el paquete descargable: dist/boot-preview.tar.gz.
//
// El paquete lleva el binario y los assets (los ejemplos que usa el
// instalador). Es lo que se descarga el usuario con un solo comando:
//
//	curl -fsSL .../releases/latest/download/boot-preview.tar.gz | tar xz
//	&& ./boot-preview/boot-preview install
func Bundle(paths cli.Paths) error {
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

// ── Utilidades ──────────────────────────────────────────────────────────────

// copyFile copia un archivo.

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
