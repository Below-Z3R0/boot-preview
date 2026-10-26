package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// ── apply ───────────────────────────────────────────────────────────────────

// cmdApply aplica el tema al Limine real.
//
// El wallpaper va a /boot/splash.png (la ESP) y el bloque de theming a
// /boot/limine.conf, antes de la primera entrada.
//
// NO toca /etc/default/limine: ese es el origen, y Limine lo compila con
// limine-mkinitcpio. Reescribir el conf real directamente es lo que
// permite iterar sin recompilar el initramfs entero.
func cmdApply(opts options, paths cli.ToolPaths) error {
	needWallpaper := opts.applyWhat == "all" || opts.applyWhat == "wallpaper"
	needConfig := opts.applyWhat == "all" || opts.applyWhat == "config"

	cli.Step("Aplicando al Limine real")
	fmt.Println()

	if needWallpaper {
		src := filepath.Join(paths.LoaderConfig(), "wallpaper", "wallpaper.png")
		if _, err := os.Stat(src); err != nil {
			src = filepath.Join(paths.LoaderConfig(), "wallpaper", "rei.png")
		}
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("no hay wallpaper que aplicar")
		}
		// OJO: /boot es la ESP en vfat (fmask=0077), así que los permisos
		// los fija el mount y chmod no hace nada ahí. No hace falta
		// normalizar el modo: Limine lee la ESP directo.
		if out, err := cli.RunCombined("sudo", "cp", src, "/boot/splash.png"); err != nil {
			return fmt.Errorf("no se pudo copiar el wallpaper: %s", out)
		}
		cli.Ok("wallpaper  ->  /boot/splash.png")
	}

	if needConfig {
		// Red de seguridad: el conf real tiene las entradas del menú. Si
		// algo sale mal al reescribirlo, se puede volver con este backup.
		backup := "/boot/limine.conf.bak-" + timestamp()
		if _, err := cli.RunCombined("sudo", "cp", "/boot/limine.conf", backup); err == nil {
			cli.Ok("backup      ->  %s", backup)
		}
		// /boot es la ESP: no conviene llenarla de backups. Se conservan
		// los 5 más recientes.
		pruneBackups("/boot/limine.conf.bak-", 5)

		if err := applyThemeBlock(paths); err != nil {
			return err
		}
	}

	fmt.Println()
	cli.Say("Para verlo antes de reiniciar:")
	cli.Say("    boot-preview loader real")
	fmt.Println()
	cli.Say("Para que tome efecto:  sudo reboot")
	return nil
}

// pruneBackups deja solo los keep backups más recientes.
//
// Los backups se nombran con un timestamp, así que el orden alfabético
// es también el orden temporal.
func pruneBackups(prefix string, keep int) {
	// /boot es la ESP en vfat con dmask=0077: os.ReadDir devuelve
	// EACCES y los backups se acumulaban sin tope. Se lista con sudo.
	out, err := cli.RunCombined("sudo", "ls", "-1", "/boot")
	if err != nil {
		return
	}

	var backups []string
	for _, name := range strings.Split(out, "\n") {
		name = strings.TrimSpace(name)
		if strings.HasPrefix(name, prefix) {
			backups = append(backups, filepath.Join("/boot", name))
		}
	}
	if len(backups) <= keep {
		return
	}

	// ls -1 ya viene ordenado por nombre, y el nombre lleva timestamp.
	for _, old := range backups[:len(backups)-keep] {
		_, _ = cli.RunCombined("sudo", "rm", "-f", old)
	}
}

// bootFileExists dice si existe un archivo de /boot.
//
// No alcanza con os.Stat: /boot es de root y vfat con dmask=0077, asi
// que el usuario recibe EACCES en vez de ENOENT, y el codigo confundia
// "no puedo leerlo" con "no existe".
func bootFileExists(path string) bool {
	_, err := cli.RunCombined("sudo", "test", "-e", path)
	return err == nil
}

// readBootFile lee un archivo de /boot con sudo.
func readBootFile(path string) ([]byte, error) {
	out, err := cli.RunCombined("sudo", "cat", path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %s", path, strings.TrimSpace(out))
	}
	return []byte(out), nil
}

// applyThemeBlock inyecta el bloque de theming en /boot/limine.conf.
func applyThemeBlock(paths cli.ToolPaths) error {
	previewPath := filepath.Join(paths.LoaderConfig(), "limine.conf")

	if !bootFileExists("/boot/limine.conf") {
		cli.Warn("No existe /boot/limine.conf")
		cli.Say("apply solo reescribe el bloque de theming; las entradas del")
		cli.Say("menú no se pueden reconstruir desde acá.")
		cli.Say("Restauralo desde un backup:")
		cli.Say("  sudo ls -1t /boot/limine.conf.bak-* /boot/limine.conf.old")
		cli.Say("  sudo cp /boot/limine.conf.bak-XXXX /boot/limine.conf")
		return fmt.Errorf("no existe /boot/limine.conf")
	}

	preview, err := os.ReadFile(previewPath)
	if err != nil {
		return fmt.Errorf("no se pudo leer la config del preview: %w", err)
	}

	// El bloque de theming sale de la config del PREVIEW (la que edita el
	// usuario con `boot-preview loader edit`).
	block := extractThemeBlock(string(preview))
	if len(block) == 0 {
		return fmt.Errorf("la config del preview no tiene opciones de theming")
	}

	// El wallpaper del preview apunta al ISO (boot():/wallpaper.png); en el
	// Limine real va a /boot/splash.png.
	rewriteWallpaperForReal(block)

	real, err := readBootFile("/boot/limine.conf")
	if err != nil {
		return err
	}

	// Se quitan las opciones de theming del conf real (y el bloque
	// anterior, si lo hubiera).
	//
	// La cabecera y los comentarios del bloque se borran SIEMPRE, aunque
	// hayan quedado huérfanos (por ejemplo si una actualización borró las
	// opciones y dejó los comentarios): si no, al reinsertar el bloque se
	// duplican.
	lines := strings.Split(string(real), "\n")
	var kept []string
	skipping := false
	for _, line := range lines {
		if isThemeOption(line) {
			continue
		}
		if isOwnComment(line) {
			skipping = true
			continue
		}
		if skipping {
			if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
				continue
			}
			skipping = false
		}
		kept = append(kept, line)
	}

	// Y se inserta el bloque nuevo antes de la primera entrada.
	insertAt := len(kept)
	for i, line := range kept {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "/") {
			insertAt = i
			break
		}
	}

	header := []string{
		"# ── Limine theming (b3l0wz3r0-os) ──",
		"# Aplicado por boot-preview loader apply. Editá la config del preview",
		"# y volvé a aplicar para cambiarlo.",
	}

	var out []string
	out = append(out, kept[:insertAt]...)
	out = append(out, header...)
	out = append(out, block...)
	out = append(out, kept[insertAt:]...)

	if err := writeFile("/boot/limine.conf", strings.Join(out, "\n")+"\n", true); err != nil {
		return err
	}

	cli.Ok("config visual  ->  /boot/limine.conf (%d opciones)", len(block))
	return nil
}

// optionRe matchea una opción del conf ("clave: valor").
var optionRe = regexp.MustCompile(`^\s*([a-z_0-9]+)\s*:`)

// extractThemeBlock saca las opciones de theming de un conf.
func extractThemeBlock(content string) []string {
	var block []string
	for _, line := range strings.Split(content, "\n") {
		match := optionRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for _, option := range themeOptions {
			if match[1] == option {
				block = append(block, line)
				break
			}
		}
	}
	return block
}

// isThemeOption dice si una línea es una opción de theming.
func isThemeOption(line string) bool {
	match := optionRe.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	for _, option := range themeOptions {
		if match[1] == option {
			return true
		}
	}
	return false
}

// isOwnComment dice si la línea es un comentario que escribió boot-preview.
func isOwnComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, comment := range ownComments {
		if strings.HasPrefix(trimmed, comment) {
			return true
		}
	}
	return false
}

// rewriteWallpaperForReal cambia el wallpaper del preview (el del ISO)
// por el de la ESP.
//
// Se matchea con la misma regex que usa la extracción: la línea puede
// venir indentada o con espacios antes de los dos puntos
// ("  wallpaper : boot():/wallpaper.png").
func rewriteWallpaperForReal(block []string) {
	for i, line := range block {
		match := optionRe.FindStringSubmatch(line)
		if match != nil && match[1] == "wallpaper" {
			block[i] = "wallpaper: boot():/splash.png"
		}
	}
}
