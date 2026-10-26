package cli

import (
	"strings"
	"testing"
	"time"
)

// Un comando que no termina tiene que morir en el deadline: si no,
// el que lo llama hereda la espera (el `plymouth --quit` colgado 30 s).
func TestRunTimeoutCorta(t *testing.T) {
	start := time.Now()
	_, err := RunTimeout(200*time.Millisecond, "sleep", "5")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("esperaba error por timeout")
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("tardó %v: el deadline no se aplicó", elapsed)
	}
}

func TestRunTimeoutDevuelveSalida(t *testing.T) {
	out, err := RunTimeout(2*time.Second, "echo", "hola")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if strings.TrimSpace(out) != "hola" {
		t.Fatalf("salida = %q, quiero %q", out, "hola")
	}
}

// Un comando puede contestar solo con el código de salida (plymouth
// --ping no imprime nada). Eso es éxito, no un fallo: el error es la
// señal, no la ausencia de texto.
func TestRunTimeoutExitCeroSinSalida(t *testing.T) {
	out, err := RunTimeout(2*time.Second, "true")
	if err != nil {
		t.Fatalf("exit 0 sin salida dio error: %v", err)
	}
	if out != "" {
		t.Fatalf("salida = %q, quiero vacía", out)
	}
}
