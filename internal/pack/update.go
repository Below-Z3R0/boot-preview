package pack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
func update(paths cli.ToolPaths) error {
	cli.Step("Buscando actualizaciones")

	if paths.Repo != "" && isGitRepo(paths.Repo) && hasRemote(paths.Repo) {
		cli.Say("Repo local: %s", paths.Repo)
		return updateFromGit(paths)
	}

	cli.Say("Sin repo local: descargando de %s", remoteURL)
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
func updateFromGit(paths cli.ToolPaths) error {
	before := gitHead(paths.Repo)
	if out, err := cli.RunCombined("git", "-C", paths.Repo, "pull", "--ff-only"); err != nil {
		cli.Warn("No se pudo actualizar: el repo tiene cambios locales.")
		cli.Say("%s", strings.TrimSpace(out))
		cli.Say("")
		cli.Say("Para actualizar sin perderlos:")
		cli.Say("    git -C %s stash       # guarda los cambios", paths.Repo)
		cli.Say("    boot-preview update    # trae la versión nueva")
		cli.Say("    git -C %s stash pop   # los recupera", paths.Repo)
		return fmt.Errorf("no se pudo actualizar: el repo tiene cambios locales")
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
		return err
	}
	return nil
}

// updateFromNetwork descarga la última versión de GitHub.
//
// La release publica el bundle (boot-preview.tar.gz) con el binario y los
// assets adentro. El binario suelto es opcional: si está, se usa; si no, se
// extrae del bundle.
//
// Antes se pedía siempre "boot-preview-linux-amd64", que la release no
// publica: el update daba 404 y el camino de red no funcionaba nunca.
func updateFromNetwork(paths cli.ToolPaths) error {
	if !cli.HaveCommand("curl") {
		cli.Warn("necesita curl")
		return fmt.Errorf("falta curl")
	}

	tmp, err := os.MkdirTemp("", "boot-preview-update.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	bin, err := downloadRelease(tmp)
	if err != nil {
		return err
	}

	// Se comprueba que funciona antes de pisar el instalado.
	if out, err := cli.RunCombined(bin, "version"); err != nil {
		cli.Warn("el binario descargado no funciona")
		cli.Say("%s", out)
		return fmt.Errorf("el binario descargado no funciona")
	}

	dest := filepath.Join(paths.Bin, "boot-preview")
	if err := os.MkdirAll(paths.Bin, 0o755); err != nil {
		return err
	}
	// ReplaceFile: si estás actualizando desde el binario instalado,
	// copiar sobre él da "text file busy".
	if err := cli.ReplaceFile(bin, dest); err != nil {
		return err
	}
	cli.Ok("binario actualizado")
	return nil
}

// downloadRelease deja el binario de la última release en dir y devuelve
// su ruta.
func downloadRelease(dir string) (string, error) {
	base := remoteURL + "/releases/latest/download"

	// 1) El binario suelto, si la release lo trae.
	suelto := filepath.Join(dir, "boot-preview")
	if _, err := cli.RunCombined("curl", "-fsSL", base+"/boot-preview-linux-amd64", "-o", suelto); err == nil {
		return suelto, nil
	}

	// 2) El bundle, que es lo que sí publica.
	if !cli.HaveCommand("tar") {
		return "", fmt.Errorf("falta tar (para extraer el binario del bundle)")
	}
	bundle := filepath.Join(dir, "bundle.tar.gz")
	if out, err := cli.RunCombined("curl", "-fsSL", base+"/boot-preview.tar.gz", "-o", bundle); err != nil {
		cli.Warn("no se pudo descargar (¿sin red?)")
		cli.Say("%s", out)
		return "", fmt.Errorf("no se pudo descargar la actualización")
	}
	return extractBinary(bundle, dir)
}

// extractBinary extrae el bundle y devuelve la ruta del binario.
//
// No se asume el layout: el bundle viejo (v0.1.x) traía solo scripts Bash y
// el nuevo trae boot-preview/boot-preview. Se busca el ejecutable llamado
// "boot-preview" en cualquier nivel.
func extractBinary(bundle, dir string) (string, error) {
	extraido := filepath.Join(dir, "extraido")
	if err := os.MkdirAll(extraido, 0o755); err != nil {
		return "", err
	}
	if out, err := cli.RunCombined("tar", "-xzf", bundle, "-C", extraido); err != nil {
		cli.Say("%s", out)
		return "", fmt.Errorf("el bundle no se pudo extraer")
	}

	var encontrado string
	_ = filepath.Walk(extraido, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "boot-preview" && info.Mode()&0o111 != 0 {
			encontrado = path
			return filepath.SkipAll
		}
		return nil
	})

	if encontrado == "" {
		return "", fmt.Errorf("el bundle no trae un binario (¿es un release viejo?)")
	}
	return encontrado, nil
}

// reinstall reconstruye el binario y lo vuelve a instalar.
//
// El repo es Go: un git pull trae el código nuevo, pero el binario
// instalado sigue siendo el viejo. Hay que recompilar antes de instalar.
func reinstall(paths cli.ToolPaths) error {
	if paths.Repo == "" {
		return fmt.Errorf("no hay repo")
	}

	// Si hay go.mod, se recompila.
	if _, err := os.Stat(filepath.Join(paths.Repo, "go.mod")); err == nil {
		if !cli.HaveCommand("go") {
			return fmt.Errorf("hace falta Go para recompilar (o instalá el binario de las releases)")
		}
		cli.Say("Recompilando...")
		if out, err := cli.RunIn(paths.Repo, "go", "build", "-o", filepath.Join(paths.Repo, "dist", "boot-preview"), "./cmd/boot-preview"); err != nil {
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
		if err := cli.ReplaceFile(bin, dest); err != nil {
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
