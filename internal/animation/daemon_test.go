package animation

import (
	"os"
	"testing"
)

// el ppid se lee de /proc: el campo 2 (comm) puede tener espacios y
// paréntesis ("plymouthd --no-daemon"), y contar campos desde el
// principio rompe.
func TestPpidOfLeeElPadre(t *testing.T) {
	got := ppidOf(os.Getpid())
	want := os.Getppid()
	if got != want {
		t.Fatalf("ppidOf(%d) = %d, quiero %d", os.Getpid(), got, want)
	}
}

func TestPpidOfInexistente(t *testing.T) {
	// Un PID que no existe: /proc no lo tiene y no hay padre que leer.
	if got := ppidOf(999999); got != 0 {
		t.Fatalf("ppidOf(999999) = %d, quiero 0", got)
	}
}
