package install

import (
	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// Version devuelve la versión del tool.
//
// Se inyecta en el build con -ldflags "-X ...install.Version=v1.2.3".
var Version = func() string { return "dev" }

// sortStrings ordena la lista en el lugar (por referencia).
func sortStrings(list []string) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// today devuelve la fecha de hoy, en ISO.
func today() string {
	out, _ := cli.Run("date", "+%Y-%m-%d")
	return out
}
