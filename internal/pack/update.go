package pack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
func update(paths cli.Paths) error {
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
