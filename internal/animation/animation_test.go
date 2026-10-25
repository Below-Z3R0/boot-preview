package animation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceLine(t *testing.T) {
	const original = "[Plymouth Theme]\nName=foo\n\n[script]\nImageDir=/usr/share/plymouth/themes/foo\nScriptFile=/usr/share/plymouth/themes/foo/script.script\n"

	tests := []struct {
		name   string
		key    string
		value  string
		expect string
	}{
		{
			name:   "reemplaza ImageDir",
			key:    "ImageDir",
			value:  "/run/plymouth/themes/foo",
			expect: "[Plymouth Theme]\nName=foo\n\n[script]\nImageDir=/run/plymouth/themes/foo\nScriptFile=/usr/share/plymouth/themes/foo/script.script\n",
		},
		{
			name:   "reemplaza ScriptFile",
			key:    "ScriptFile",
			value:  "/run/plymouth/themes/foo/script.script",
			expect: "[Plymouth Theme]\nName=foo\n\n[script]\nImageDir=/usr/share/plymouth/themes/foo\nScriptFile=/run/plymouth/themes/foo/script.script\n",
		},
		{
			name:   "una clave que no existe no cambia nada",
			key:    "ModuleName",
			value:  "script",
			expect: original,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := replaceLine(original, tc.key, tc.value)
			if got != tc.expect {
				t.Errorf("replaceLine()\n obtuve:\n%s\n quiero:\n%s", got, tc.expect)
			}
		})
	}
}

func TestReplaceLineNoPrefixConfusion(t *testing.T) {
	// "ImageDir" no debe pisar "ImageDirBackup".
	const original = "ImageDirBackup=/a\nImageDir=/b\n"
	got := replaceLine(original, "ImageDir", "/c")

	if got != "ImageDirBackup=/a\nImageDir=/c\n" {
		t.Errorf("replaceLine() no debe tocar prefijos: obtuve %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in     string
		expect string
	}{
		{":1", "':1'"},
		{"", "''"},
		{"sin espacios", "'sin espacios'"},
		{"con'comilla", `'con'\''comilla'`},
	}

	for _, tc := range tests {
		got := shellQuote(tc.in)
		if got != tc.expect {
			t.Errorf("shellQuote(%q) = %q, quiero %q", tc.in, got, tc.expect)
		}
	}
}

func TestSpread(t *testing.T) {
	list := []string{"a", "b", "c", "d", "e"}

	// Con más elementos que los pedidos: se distribuyen.
	got := spread(list, 3)
	if len(got) != 3 {
		t.Fatalf("spread() devolvio %d elementos, quiero 3", len(got))
	}
	if got[0] != "a" || got[2] != "e" {
		t.Errorf("spread() = %v, quiero que arranque en a y termine en e", got)
	}

	// Con menos elementos que los pedidos: se devuelven todos.
	got = spread(list, 10)
	if len(got) != len(list) {
		t.Errorf("spread() devolvio %d elementos, quiero %d", len(got), len(list))
	}
}

func TestEnvPrefix(t *testing.T) {
	t.Setenv("DISPLAY", ":1")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	os.Unsetenv("WAYLAND_DISPLAY")

	got := envPrefix()

	if got == "" {
		t.Fatal("envPrefix() quedo vacio")
	}
	if !contains(got, "DISPLAY=") {
		t.Errorf("envPrefix() = %q, quiero que incluya DISPLAY", got)
	}
	// WAYLAND_DISPLAY no debe estar: el renderer x11.so usa GTK, y si GTK
	// encuentra un servidor Wayland elige ese backend y la ventana se abre
	// en la sesion equivocada.
	if contains(got, "WAYLAND_DISPLAY") {
		t.Errorf("envPrefix() = %q, no debe incluir WAYLAND_DISPLAY", got)
	}
	// TERM siempre: sin el, GTK puede fallar al inicializar.
	if !contains(got, "TERM=") {
		t.Errorf("envPrefix() = %q, quiero que incluya TERM", got)
	}
}

func TestEnvPrefixAlwaysHasTerm(t *testing.T) {
	os.Unsetenv("TERM")
	t.Setenv("DISPLAY", ":1")

	got := envPrefix()
	if !contains(got, "TERM=") {
		t.Errorf("envPrefix() = %q, quiero TERM con un valor de reemplazo", got)
	}
}

func TestThemeModule(t *testing.T) {
	dir := t.TempDir()
	ply := filepath.Join(dir, "foo.plymouth")
	content := "[Plymouth Theme]\nName=foo\nModuleName=script\n\n[script]\nImageDir=/x\n"
	if err := os.WriteFile(ply, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := themeModule(dir); got != "script" {
		t.Errorf("themeModule() = %q, quiero %q", got, "script")
	}
}

func TestThemeModuleFallback(t *testing.T) {
	// Sin .plymouth: el modulo por defecto de Plymouth.
	if got := themeModule(t.TempDir()); got != "script" {
		t.Errorf("themeModule() = %q, quiero %q", got, "script")
	}
}

func TestOrNone(t *testing.T) {
	if got := orNone(""); got != "ninguna" {
		t.Errorf("orNone(\"\") = %q, quiero %q", got, "ninguna")
	}
	if got := orNone("algo"); got != "algo" {
		t.Errorf("orNone(\"algo\") = %q, quiero %q", got, "algo")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
