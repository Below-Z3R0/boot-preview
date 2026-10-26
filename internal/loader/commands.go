package loader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// mapCtrlQ manda ctrl+q al QEMU por el socket de QMP.
//
// El backend SDL no trae el atajo (era del backend GTK). QMP es
// independiente del display: send-key inyecta la tecla en el QEMU y el
// quit de QMP lo cierra.
func mapCtrlQ(qmpSock string) {
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(qmpSock); err == nil {
			break
		}
		sleepMillis(100)
	}

	// El handshake de QMP va por el mismo socket.
	handshake := `{"execute":"qmp_capabilities"}
{"execute":"send-key","arguments":{"keys":[{"type":"qcode","data":"ctrl"},{"type":"qcode","data":"q"}]}}
`
	cmd := exec.Command("socat", "-", "UNIX-CONNECT:"+qmpSock)
	cmd.Stdin = strings.NewReader(handshake)
	_ = cmd.Run()

	// Se espera a que el QEMU procese la tecla y después se cierra.
	time.Sleep(200 * time.Millisecond)
	quit := exec.Command("socat", "-", "UNIX-CONNECT:"+qmpSock)
	quit.Stdin = strings.NewReader(`{"execute":"quit"}` + "\n")
	_ = quit.Run()
}

// sleepMillis espera un rato.
func sleepMillis(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

// nowSeconds devuelve el epoch en segundos.
func nowSeconds() int64 {
	return time.Now().Unix()
}

// timestamp devuelve el timestamp para los nombres de archivo.
func timestamp() string {
	return time.Now().Format("20060102-150405")
}

// ensureDir crea un directorio si no existe.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// dirOf devuelve el directorio de un path.
func dirOf(path string) string {
	return filepath.Dir(path)
}

// unusedFmt evita que el import de fmt se marque como no usado si más
// adelante no hace falta.
var _ = fmt.Sprintf

// writeFile escribe un archivo, con sudo si hace falta.
func writeFile(path, content string, needsSudo bool) error {
	if !needsSudo {
		return os.WriteFile(path, []byte(content), 0o644)
	}
	tmp, err := os.CreateTemp("", "boot-preview-write.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		return err
	}
	tmp.Close()
	_, err = cli.RunCombined("sudo", "cp", tmp.Name(), path)
	return err
}
