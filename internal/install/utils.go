package install

import (
	"time"
)

// Version es la versión del tool.
//
// Se inyecta en el build con -ldflags "-X ...install.Version=v1.2.3".
// Antes era una FUNCIÓN, y -X solo reemplaza strings: la versión quedaba
// siempre en "dev".
var Version = "dev"

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
	return time.Now().Format("2006-01-02")
}
