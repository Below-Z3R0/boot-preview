package pack

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// arbol arma un árbol como el del bundle: el binario en la raíz y los
// assets en assets/.
func arbol(t *testing.T) (dir, root string) {
	t.Helper()
	dir = t.TempDir()
	root = filepath.Join(dir, "boot-preview")

	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "boot-preview"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# boot-preview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "loader.conf"), []byte("timeout: no\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, root
}

// TestWriteTarGzEsReproducible verifica que dos corridas den el mismo
// archivo.
//
// El bug: el tar guardaba el uid/gid de quien lo armaba y el mtime de
// cada archivo, así que el mismo código daba hashes distintos (y los
// archivos quedaban de un uid ajeno para quien lo descargara).
func TestWriteTarGzEsReproducible(t *testing.T) {
	dir, root := arbol(t)

	uno := filepath.Join(dir, "uno.tar.gz")
	dos := filepath.Join(dir, "dos.tar.gz")

	if err := writeTarGz(uno, root); err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}
	if err := writeTarGz(dos, root); err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}

	if h1, h2 := hash(t, uno), hash(t, dos); h1 != h2 {
		t.Errorf("no es reproducible:\n  %s\n  %s", h1, h2)
	}
}

// TestWriteTarGzNormalizaLosMetadatos verifica uid/gid 0 y el bit +x.
func TestWriteTarGzNormalizaLosMetadatos(t *testing.T) {
	dir, root := arbol(t)
	out := filepath.Join(dir, "b.tar.gz")

	if err := writeTarGz(out, root); err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	vistos := map[string]*tar.Header{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		vistos[h.Name] = h
	}

	bin, ok := vistos["boot-preview/boot-preview"]
	if !ok {
		t.Fatalf("falta el binario en el tar: %v", claves(vistos))
	}
	if bin.Uid != 0 || bin.Gid != 0 {
		t.Errorf("uid/gid = %d/%d, quiero 0/0", bin.Uid, bin.Gid)
	}
	if bin.Uname != "root" || bin.Gname != "root" {
		t.Errorf("uname/gname = %q/%q, quiero root", bin.Uname, bin.Gname)
	}
	if bin.Mode&0o111 == 0 {
		t.Errorf("el binario perdió el bit +x: modo %o", bin.Mode)
	}

	conf, ok := vistos["boot-preview/assets/loader.conf"]
	if !ok {
		t.Fatalf("falta el asset: %v", claves(vistos))
	}
	if conf.Mode&0o111 != 0 {
		t.Errorf("un archivo común quedó ejecutable: modo %o", conf.Mode)
	}
}

// TestWriteTarGzNoAcumulaDescriptors es una prueba de humo del Walk: con
// muchos archivos, la función tiene que terminar (antes los fd se
// abrían con defer y se acumulaban).
func TestWriteTarGzConMuchosArchivos(t *testing.T) {
	dir, root := arbol(t)
	for i := 0; i < 200; i++ {
		name := filepath.Join(root, "assets", "f"+string(rune('a'+i%26))+string(rune('a'+i/26))+".conf")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := filepath.Join(dir, "muchos.tar.gz")
	if err := writeTarGz(out, root); err != nil {
		t.Fatalf("writeTarGz con 200 archivos: %v", err)
	}
	if info, err := os.Stat(out); err != nil || info.Size() == 0 {
		t.Errorf("el tar salió vacío o no existe: %v", err)
	}
}

func hash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func claves(m map[string]*tar.Header) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestExtractBinaryDelBundle verifica el camino de update: bajar el
// bundle y sacar el binario de adentro.
//
// El release viejo (v0.1.x) traia solo scripts Bash, asi que el extractor
// no puede asumir el layout: busca el ejecutable "boot-preview" en
// cualquier nivel.
func TestExtractBinaryDelBundle(t *testing.T) {
	dir, root := arbol(t)
	bundle := filepath.Join(dir, "b.tar.gz")
	if err := writeTarGz(bundle, root); err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}

	got, err := extractBinary(bundle, t.TempDir())
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "#!/bin/sh\n" {
		t.Errorf("extrajo otro archivo: %q", data)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("el binario extraido no es ejecutable: %o", info.Mode())
	}
}

// TestExtractBinarySinBinario falla con un mensaje claro.
//
// Es el caso del release v0.1.1: trae scripts, no un binario Go.
func TestExtractBinarySinBinario(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "viejo")
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(dir, "viejo.tar.gz")
	if err := writeTarGz(bundle, root); err != nil {
		t.Fatalf("writeTarGz: %v", err)
	}

	_, err := extractBinary(bundle, t.TempDir())
	if err == nil {
		t.Fatal("acepto un bundle sin binario")
	}
	if !strings.Contains(err.Error(), "no trae un binario") {
		t.Errorf("mensaje poco claro: %v", err)
	}
}
