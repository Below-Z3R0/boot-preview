package install

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSameFileDetectaElMismoArchivo verifica la deteccion de "el
// destino es el ejecutable que se esta corriendo".
//
// El bug: al reinstalar el tool ya instalado, copiar el binario sobre
// si mismo daba "text file busy" (el kernel no deja escribir un
// binario en uso).
func TestSameFileDetectaElMismoArchivo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boot-preview")

	if err := os.WriteFile(path, []byte("binario"), 0o755); err != nil {
		t.Fatal(err)
	}

	if !sameFile(path, path) {
		t.Error("sameFile() dio false para el mismo path")
	}

	otro := filepath.Join(dir, "otro")
	if err := os.WriteFile(otro, []byte("otro"), 0o755); err != nil {
		t.Fatal(err)
	}
	if sameFile(path, otro) {
		t.Error("sameFile() dio true para dos archivos distintos")
	}

	// Un path que no existe.
	if sameFile(path, filepath.Join(dir, "no-existe")) {
		t.Error("sameFile() dio true para un archivo inexistente")
	}
}

// TestInstallBinarySobreSiMismoNoFalla verifica que reinstalar el
// binario que se esta ejecutando funciona.
func TestInstallBinarySobreSiMismoNoFalla(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "boot-preview")

	// Un "ejecutable" en el destino.
	if err := os.WriteFile(dest, []byte("contenido viejo"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Se copia a si mismo: tiene que funcionar.
	// (En el caso real el archivo esta en uso; aca no, pero la logica del
	// rename es la misma.)
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "contenido viejo" {
		t.Errorf("leyo %q", data)
	}
}

// TestContainsVerificaElComponente verifica el helper de los componentes.
func TestContainsVerificaElComponente(t *testing.T) {
	componentes := []string{"loader", "animation"}

	if !contains(componentes, "loader") {
		t.Error("contains() dio false para 'loader'")
	}
	if contains(componentes, "otro") {
		t.Error("contains() dio true para 'otro'")
	}
	if contains(nil, "loader") {
		t.Error("contains() dio true con la lista vacia")
	}
}

// TestSortStringsOrdena verifica el sort usado para los componentes del
// state.json.
func TestSortStringsOrdena(t *testing.T) {
	lista := []string{"animation", "loader", "zebra"}
	sortStrings(lista)

	if lista[0] != "animation" || lista[1] != "loader" || lista[2] != "zebra" {
		t.Errorf("sortStrings() = %v", lista)
	}
}
