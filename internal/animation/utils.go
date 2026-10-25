package animation

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// sleep espera un rato.
func sleep(d time.Duration) {
	time.Sleep(d)
}

// timeStamp devuelve el timestamp para los nombres de archivo.
func timeStamp() string {
	return time.Now().Format("20060102-150405")
}

// dirSize devuelve el tamaño total de una carpeta.
func dirSize(path string) string {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return cli.HumanSize(total)
}

// parseRes parsea la resolución pedida.
//
// Si no se puede, cae en 1920x1080: es la resolución de la mayoría de las
// pantallas y el tema se ve igual de bien que con cualquier otra.
func parseRes(res string) (int, int) {
	w, h, err := cli.ParseRes(res)
	if err != nil {
		return 1920, 1080
	}
	return w, h
}
