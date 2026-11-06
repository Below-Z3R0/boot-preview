package animation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// El tema base de CachyOS no tiene un logo.png: el logo es la secuencia
// animation-NN.png del plugin two-step. Tocar ese tema se pierde en el
// próximo `pacman -Syu`.
//
// Por eso el logo del usuario vive en un tema propio, b3l0w-splash,
// que hereda el throbber y las fuentes del base pero usa una sola
// imagen como frame.
const (
	temaBase = "cachyos-bootanimation"
	temaLogo = "b3l0w-splash"
)

// firmaPNG son los primeros bytes de cualquier PNG.
var firmaPNG = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// cmdLogo cambia la imagen del splash de Plymouth.
func cmdLogo(args []string, paths cli.ToolPaths) error {
	reset := false
	imagen := ""

	for _, arg := range args {
		switch arg {
		case "--reset", "-r":
			reset = true
		case "-h", "--help":
			logoHelp()
			return nil
		default:
			if !strings.HasPrefix(arg, "-") {
				imagen = arg
			}
		}
	}

	switch {
	case reset:
		return logoReset()
	case imagen == "":
		return logoStatus()
	default:
		return logoApply(imagen)
	}
}

// logoStatus muestra qué está usando el sistema ahora.
func logoStatus() error {
	tema := systemTheme()
	cli.Say("Tema activo: %s", tema)
	if tema == temaLogo {
		info, err := os.Stat(filepath.Join(themesDir, temaLogo, "animation-00.png"))
		if err == nil {
			cli.Ok("Logo propio: %d KB", info.Size()/1024)
		}
	} else {
		cli.Say("El logo es el del tema '%s'.", tema)
	}
	fmt.Println()
	cli.Say("Cambiarlo:  boot-preview animation logo <imagen.png>")
	cli.Say("Volver al base: boot-preview animation logo --reset")
	return nil
}

// logoApply arma el tema propio con la imagen elegida.
func logoApply(imagen string) error {
	abs, err := filepath.Abs(imagen)
	if err != nil {
		return err
	}
	if err := validarPNG(abs); err != nil {
		return err
	}

	base := filepath.Join(themesDir, temaBase)
	if _, err := os.Stat(base); err != nil {
		return fmt.Errorf("no existe el tema base %s: %w", temaBase, err)
	}

	tmp, err := os.MkdirTemp("", "b3l0w-splash-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Heredar throbber, fuentes e iconos del tema base.
	if out, err := cli.RunCombined("cp", "-r", base+"/.", tmp); err != nil {
		return fmt.Errorf("no se pudo copiar el tema base: %s", out)
	}

	// Un solo frame: la imagen del usuario. Los demás se borran para
	// que two-step no anime una secuencia vieja.
	frames, _ := filepath.Glob(filepath.Join(tmp, "animation-*.png"))
	for _, frame := range frames {
		if err := os.Remove(frame); err != nil {
			return err
		}
	}
	if err := copiar(abs, filepath.Join(tmp, "animation-00.png")); err != nil {
		return err
	}

	// El .plymouth propio: mismo plugin, ImageDir del tema nuevo.
	plymouth := `[Plymouth Theme]
Name=b3l0w-splash
Description=Logo propio de b3l0w
ModuleName=two-step

[two-step]
Font=Inter 12
TitleFont=Inter Bold 32
ImageDir=/usr/share/plymouth/themes/b3l0w-splash
DialogHorizontalAlignment=.5
DialogVerticalAlignment=.5
TitleHorizontalAlignment=.5
TitleVerticalAlignment=.382
HorizontalAlignment=.5
VerticalAlignment=.5
Transition=none
TransitionDuration=0.0
BackgroundStartColor=0x000000
BackgroundEndColor=0x000000
ProgressBarBackgroundColor=0x606060
ProgressBarForegroundColor=0xffffff
DialogClearsFirmwareBackground=false
MessageBelowAnimation=true

[boot-up]
UseEndAnimation=true
UseFirmwareBackground=false

[shutdown]
UseEndAnimation=false
UseFirmwareBackground=false

[reboot]
UseEndAnimation=false
UseFirmwareBackground=false
`
	if err := os.WriteFile(filepath.Join(tmp, temaLogo+".plymouth"), []byte(plymouth), 0o644); err != nil {
		return err
	}

	cli.Step("Instalando '%s'", temaLogo)
	destino := filepath.Join(themesDir, temaLogo)
	if out, err := cli.RunCombined("sudo", "rm", "-rf", destino); err != nil {
		return fmt.Errorf("no se pudo limpiar el tema anterior: %s", out)
	}
	if out, err := cli.RunCombined("sudo", "cp", "-r", tmp, destino); err != nil {
		return fmt.Errorf("no se pudo copiar el tema: %s", out)
	}

	cli.Warn("Regenerando el initramfs (~30 s): el splash del próximo boot cambia.")
	if out, err := cli.RunCombined("sudo", "plymouth-set-default-theme", "-R", temaLogo); err != nil {
		return fmt.Errorf("no se pudo aplicar: %s", out)
	}

	cli.Ok("Logo aplicado. Tema activo: %s", systemTheme())
	cli.Say("Para verlo sin reiniciar: boot-preview animation")
	return nil
}

// logoReset vuelve al tema de CachyOS.
func logoReset() error {
	destino := filepath.Join(themesDir, temaLogo)
	if _, err := os.Stat(destino); err == nil {
		if out, err := cli.RunCombined("sudo", "rm", "-rf", destino); err != nil {
			return fmt.Errorf("no se pudo borrar %s: %s", destino, out)
		}
	}
	cli.Warn("Regenerando el initramfs (~30 s).")
	if out, err := cli.RunCombined("sudo", "plymouth-set-default-theme", "-R", temaBase); err != nil {
		return fmt.Errorf("no se pudo volver al tema base: %s", out)
	}
	cli.Ok("Tema activo: %s", systemTheme())
	return nil
}

// validarPNG rechaza lo que Plymouth no va a poder dibujar.
func validarPNG(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("no puedo abrir %s: %w", path, err)
	}
	defer f.Close()

	head := make([]byte, len(firmaPNG))
	if _, err := f.Read(head); err != nil {
		return fmt.Errorf("%s no es un PNG válido", path)
	}
	if !bytes.Equal(head, firmaPNG) {
		return fmt.Errorf("%s no es un PNG (Plymouth solo dibuja PNG)", path)
	}
	return nil
}

func copiar(origen, destino string) error {
	data, err := os.ReadFile(origen)
	if err != nil {
		return err
	}
	return os.WriteFile(destino, data, 0o644)
}

func logoHelp() {
	fmt.Print(`boot-preview animation logo — la imagen del splash de Plymouth

USO
  boot-preview animation logo                 qué logo está activo
  boot-preview animation logo <imagen.png>    lo cambia
  boot-preview animation logo --reset         vuelve al tema de CachyOS

QUÉ HACE
  Crea el tema b3l0w-splash heredando throbber y fuentes del tema
  base, y usa tu imagen como único frame. No toca el tema de CachyOS,
  así que un pacman -Syu no borra tu logo.

  Plymouth dibuja solo PNG. Tamaño recomendado: 1920x1080 con fondo
  transparente o negro.

  Aplicarlo regenera el initramfs (~30 s) y pide sudo.
`)
}
