package pack

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── Hooks ───────────────────────────────────────────────────────────────────

// watchedPackages son los paquetes que, si se actualizan, conviene
// revalidar: el preview usa el software del sistema.
var watchedPackages = []string{
	"plymouth", "limine", "qemu-desktop", "libisoburn",
	"imagemagick", "socat", "ffmpeg", "xdotool", "xorg-xhost",
}

// hookDef es el archivo .hook que pacman lee.
//
// El Exec llama al binario con "hooks notify": los targets que dispararon
// el hook llegan por stdin, uno por línea (por el NeedsTargets).
const hookDef = `[Trigger]
Operation = Upgrade
Type = Package
Target = plymouth
Target = limine
Target = qemu-desktop
Target = libisoburn
Target = imagemagick
Target = socat
Target = ffmpeg
Target = xdotool
Target = xorg-xhost

[Action]
Description = Avisar que boot-preview puede necesitar re-verificación
When = PostTransaction
NeedsTargets
Exec = /usr/local/bin/boot-preview hooks notify
`

// Hooks instala o quita el hook de pacman.
func hooks(args []string, paths cli.ToolPaths) error {
	action := "install"
	if len(args) > 0 {
		action = args[0]
	}

	const (
		hookDir  = "/etc/pacman.d/hooks"
		hookFile = "/etc/pacman.d/hooks/boot-preview.hook"
	)

	switch action {
	case "install":
		cli.Step("Instalando el hook de pacman")

		// /etc es de root: se escribe con sudo.
		tmp, err := os.CreateTemp("", "boot-preview-hook.*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if _, err := tmp.WriteString(hookDef); err != nil {
			tmp.Close()
			return err
		}
		tmp.Close()

		if out, err := cli.RunCombined("sudo", "install", "-m", "644", tmp.Name(), hookFile); err != nil {
			return fmt.Errorf("no se pudo instalar el hook: %s", out)
		}
		cli.Ok("%s", hookFile)

		fmt.Println()
		cli.Say("A partir de ahora, cuando se actualice Plymouth, Limine o una")
		cli.Say("dependencia, pacman va a avisar que conviene revalidar.")
		return nil

	case "uninstall", "remove":
		cli.Step("Quitando el hook de pacman")
		if out, err := cli.RunCombined("sudo", "rm", "-f", hookFile); err != nil {
			cli.Warn("no se pudo borrar %s: %s", hookFile, out)
			return nil
		}
		cli.Ok("%s eliminado", hookFile)
		return nil

	case "status":
		cli.Step("Estado del hook de pacman")
		if _, err := os.Stat(hookFile); err != nil {
			cli.Say("no instalado (boot-preview hooks install)")
			return nil
		}
		cli.Ok("%s", hookFile)
		cli.Say("disparadores:")
		for _, line := range strings.Split(hookDef, "\n") {
			if strings.HasPrefix(line, "Target") {
				cli.Say("    %s", line)
			}
		}
		return nil

	case "notify":
		return notify()

	default:
		return fmt.Errorf("uso: boot-preview hooks [install|uninstall|status|notify]")
	}
}

// notify avisa que algo que boot-preview usa se actualizó.
//
// Lo llama el hook de pacman: los targets que lo dispararon llegan por
// stdin, uno por línea.
//
// Si boot-preview no está instalado, no dice nada: el hook corre como
// root y no tiene por qué molestar al que no lo usa.
func notify() error {
	configDir := filepath.Join(configHome(), "boot-preview")
	if _, err := os.Stat(configDir); err != nil {
		return nil
	}

	// Los targets que nos interesan.
	hit := map[string]bool{}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		target := strings.TrimSpace(scanner.Text())
		if target == "" {
			continue
		}
		for _, watched := range watchedPackages {
			if target == watched {
				hit[target] = true
			}
		}
	}
	if len(hit) == 0 {
		return nil
	}

	names := make([]string, 0, len(hit))
	for name := range hit {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("\n\033[38;2;235;194;186m▸\033[0m \033[1mboot-preview\033[0m: se actualizó %s\n", strings.Join(names, ", "))
	fmt.Printf("  \033[38;2;125;207;216m→\033[0m Corré \033[1mboot-preview verify\033[0m para revalidar los previews.\n\n")
	return nil
}

// configHome devuelve la carpeta de config del usuario.
func configHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}
