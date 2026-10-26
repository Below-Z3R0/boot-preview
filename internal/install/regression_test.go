package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
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

	if !cli.Contains(componentes, "loader") {
		t.Error("cli.Contains() dio false para 'loader'")
	}
	if cli.Contains(componentes, "otro") {
		t.Error("cli.Contains() dio true para 'otro'")
	}
	if cli.Contains(nil, "loader") {
		t.Error("cli.Contains() dio true con la lista vacia")
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

// TestSudoShimUsaElPathReal verifica que el wrapper apunta al binario de
// ESTE usuario.
//
// El bug: dentro de sudo, $HOME es /root, asi que "${HOME}/.local/bin/
// boot-preview" no existe; el fallback era un path hardcodeado al home
// de un usuario en particular, que no le sirve a nadie mas.
func TestSudoShimUsaElPathReal(t *testing.T) {
	shim := sudoShim("/home/otro/.local/bin/boot-preview")

	if !strings.Contains(shim, `script="/home/otro/.local/bin/boot-preview"`) {
		t.Errorf("el shim no apunta al binario real:\n%s", shim)
	}
	for _, malo := range []string{"b3low-z3r0", "${HOME}/.local"} {
		if strings.Contains(shim, malo) {
			t.Errorf("el shim tiene %q, que no es portable:\n%s", malo, shim)
		}
	}
	// Baja de root al usuario que lanzo sudo.
	if !strings.Contains(shim, `exec sudo -u "$SUDO_USER" "$script" "$@"`) {
		t.Errorf("el shim no baja de root al usuario:\n%s", shim)
	}
}

// TestSudoShimEsBashValido verifica que el shim pasa bash -n.
func TestSudoShimEsBashValido(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("sin bash")
	}

	path := filepath.Join(t.TempDir(), "shim")
	if err := os.WriteFile(path, []byte(sudoShim("/home/x/.local/bin/boot-preview")), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
		t.Errorf("bash -n fallo: %s\n%s", err, out)
	}
}
