package animation

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── preview / use ────────────────────────────────────────────────────────────

// ── preview / use ───────────────────────────────────────────────────────────

// cmdPreview cambia el tema SOLO del preview.
func cmdPreview(args []string, paths cli.ToolPaths) error {
	if len(args) == 0 || args[0] == "" {
		cli.Say("Falta el nombre del tema")
		return cmdSources(paths)
	}
	name := args[0]
	if _, err := os.Stat(filepath.Join(themesDir, name)); err != nil {
		cli.Say("No existe el tema: %s", name)
		return cmdSources(paths)
	}

	if err := os.MkdirAll(paths.AnimationConfig(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(paths.AnimationConfig(), "theme"), []byte(name+"\n"), 0o644); err != nil {
		return err
	}

	cli.Ok("El preview va a usar '%s'", name)
	cli.Say("El sistema sigue con '%s'.", systemTheme())
	cli.Say("Para verlo: boot-preview animation")
	return nil
}

// cmdUse aplica el tema al SISTEMA (regenera el initramfs).
func cmdUse(args []string, paths cli.ToolPaths) error {
	if len(args) == 0 || args[0] == "" {
		cli.Say("Falta el nombre del tema")
		return cmdSources(paths)
	}
	name := args[0]
	if _, err := os.Stat(filepath.Join(themesDir, name)); err != nil {
		cli.Say("No existe el tema: %s", name)
		return cmdSources(paths)
	}

	cli.Step("Aplicando '%s' al SISTEMA", name)
	cli.Warn("Esto regenera el initramfs real (~30 s) y cambia el splash del próximo boot.")

	if out, err := cli.RunCombined("sudo", "plymouth-set-default-theme", "-R", name); err != nil {
		return fmt.Errorf("no se pudo aplicar: %s", out)
	}

	cli.Ok("Tema activo: %s", systemTheme())
	cli.Say("Para volver al de CachyOS: boot-preview animation use cachyos-bootanimation")
	return nil
}
