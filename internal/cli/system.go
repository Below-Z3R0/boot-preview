package cli

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ── Sistema ─────────────────────────────────────────────────────────────────

// IsTTY dice si stdin es una terminal.
//
// Un comando que lee de stdin (el hook de pacman) no puede esperar un
// EOF que en una terminal nunca llega.
func IsTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// HaveCommand dice si un binario está en el PATH.
func HaveCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// CommandPath devuelve la ruta de un binario, o "" si no está.
func CommandPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// Run ejecuta un comando y devuelve su stdout. Si falla, devuelve el error.
func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

// RunCombined ejecuta un comando con stderr mezclado.
func RunCombined(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// RunTimeout ejecuta un comando y lo mata si tarda más de lo indicado.
//
// Algunos comandos hablan con un daemon y se quedan esperando la
// respuesta: `plymouth --quit` espera 30 s a un plymouthd que ya no
// contesta. Sin deadline, el comando que los llama hereda esa espera.
func RunTimeout(d time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// RunIn ejecuta un comando en un directorio y devuelve su salida.
//
// RunCombined corre en el cwd del proceso: `go build ./cmd/...` desde
// otro lado falla con "no se encontró el módulo".
func RunIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimRight(string(out), "\n"), err
}

// PackageVersion devuelve la versión instalada de un paquete de pacman, o ""
// si no está instalado.
func PackageVersion(name string) string {
	out, err := Run("pacman", "-Q", name)
	if err != nil {
		return ""
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return ""
	}
	return fields[len(fields)-1]
}

// IsRoot dice si el proceso corre como root.
func IsRoot() bool { return os.Geteuid() == 0 }

// SudoUser devuelve el usuario que lanzó sudo, o "" si no hay sudo.
func SudoUser() string {
	user := os.Getenv("SUDO_USER")
	if user == "root" {
		return ""
	}
	return user
}

// ── Tiempo ──────────────────────────────────────────────────────────────────

// Timestamp devuelve la marca para nombres de archivo (20260102-150405).
func Timestamp() string {
	return time.Now().Format("20060102-150405")
}

// NowSeconds devuelve el epoch en segundos.
func NowSeconds() int64 { return time.Now().Unix() }

// SleepMillis duerme milisegundos.
func SleepMillis(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
