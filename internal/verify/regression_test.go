package verify

import "testing"

// TestContainsVerificaElComponente verifica el helper que decide qué
// módulos hay que verificar.
//
// El bug: el verify exigia los dos modulos, pero son independientes (se
// puede instalar solo el de Plymouth).
func TestContainsVerificaElComponente(t *testing.T) {
	componentes := []string{"animation"}

	if !contains(componentes, "animation") {
		t.Error("contains() dio false para 'animation'")
	}
	if contains(componentes, "loader") {
		t.Error("contains() dio true para 'loader', que no esta instalado")
	}
	if contains(nil, "loader") {
		t.Error("contains() dio true con la lista vacia")
	}
}

// TestRowOptionalNoEsUnFallo documenta que una fila opcional no cuenta
// como fallo.
func TestRowOptionalNoEsUnFallo(t *testing.T) {
	// La funcion solo imprime: lo importante es que exista y no entre en
	// panic con un texto raro.
	rowOptional("animation/theme", "sin tema propio (usa el del sistema)")
}
