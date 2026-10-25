package animation

import (
	"os"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// printLogTail muestra las últimas líneas del log de plymouthd.
//
// El log es larguísimo (el daemon es verboso con --debug), así que solo
// se ve el final: es donde está el error.
func printLogTail(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	for _, line := range lines {
		cli.Say("    %s", line)
	}
}
