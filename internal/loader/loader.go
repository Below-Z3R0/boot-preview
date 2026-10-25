// Package loader previsualiza el menú de Limine (el bootloader).
//
// Construye un ISO híbrido con tu config de Limine y lo abre en QEMU con
// KVM. El render es EXACTO: es el mismo bootloader, el mismo wallpaper y la
// misma paleta. No toca el disco real.
package loader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
	"github.com/Below-Z3R0/boot-preview/internal/wallpaper"
)

const (
	limineDir = "/usr/share/limine"
	workDir   = "/tmp/boot-preview-loader"

	// ovmfCode y ovmfVars son el firmware UEFI. Sin él, QEMU arranca con
	// SeaBIOS y el menú sale en modo texto (sin wallpaper).
	ovmfCode = "/usr/share/edk2/x64/OVMF_CODE.4m.fd"
	ovmfVars = "/usr/share/edk2/x64/OVMF_VARS.4m.fd"
)

// injectComment es el comentario que marca el bloque que inyecta
// boot-preview. Se borra antes de reinyectar, para que el apply sea
// idempotente.
const injectComment = "# ── inyectado por boot-preview loader ──"

// themeOptions son las opciones de theming que se llevan al Limine real.
var themeOptions = []string{
	"wallpaper", "wallpaper_style", "backdrop",
	"term_margin", "term_margin_gradient", "term_background", "term_foreground",
	"term_palette", "term_palette_bright", "term_font", "term_font_size",
	"term_font_scale", "term_font_spacing",
	"interface_resolution", "interface_rotation", "interface_branding",
	"interface_branding_colour", "interface_help_colour",
	"interface_help_colour_bright", "interface_help_hidden",
}

// ownComments son los comentarios que escribe boot-preview. Se borran
// antes de insertar el bloque nuevo, o se duplicarían.
var ownComments = []string{
	"# ── Limine theming (b3l0wz3r0-os) ──",
	"# Aplicado por boot-preview loader apply",
	"# y volvé a aplicar para cambiarlo",
}

// options son las opciones del comando.
type options struct {
	command    string // preview, shot, add, edit, apply, wallpapers
	real       bool
	wallpaper  string
	conf       string
	resolution string
	timeout    int
	shotPath   string
	addImage   string
	applyWhat  string // all, config, wallpaper

	// Las opciones de preparation del wallpaper (ver `loader add --help`).
	blur    string  // "0xRADIO"; "0x0" = sin desenfoque
	dim     float64 // velo negro uniforme (0 = nada, 1 = negro)
	overlay string  // velo de ese color en vez de negro
	format  string  // png o qoi
}

// Run ejecuta el comando loader.
func Run(args []string, paths cli.Paths) error {
	opts := options{
		command:    "preview",
		wallpaper:  filepath.Join(paths.LoaderConfig(), "wallpaper", "wallpaper.png"),
		conf:       filepath.Join(paths.LoaderConfig(), "limine.conf"),
		resolution: "1920x1080",
		timeout:    15,
		applyWhat:  "all",
		blur:       "0x0",
		format:     "png",
	}

	rest := args
	if len(args) > 0 {
		switch args[0] {
		case "real", "shot", "wall", "add", "edit", "apply", "wallpapers", "help", "-h", "--help":
			opts.command = args[0]
			rest = args[1:]
		default:
			// Un subcomando que no existe tiene que avisar, no abrir la
			// VM como si fuera el preview.
			return fmt.Errorf("opción desconocida: %s", args[0])
		}
	}

	switch opts.command {
	case "help", "-h", "--help":
		usage()
		return nil
	case "real":
		opts.command = "preview"
		opts.real = true
	case "shot":
		opts.command = "shot"
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			opts.shotPath = rest[0]
			rest = rest[1:]
		}
	case "wall":
		opts.command = "preview"
		if len(rest) == 0 {
			return fmt.Errorf("falta la imagen")
		}
		opts.wallpaper = rest[0]
		rest = rest[1:]
	case "add":
		opts.command = "add"
		if len(rest) == 0 {
			return fmt.Errorf("falta la imagen")
		}
		opts.addImage = rest[0]
		rest = rest[1:]
	case "apply":
		opts.command = "apply"
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			switch rest[0] {
			case "config", "wallpaper":
				opts.applyWhat = rest[0]
				rest = rest[1:]
			default:
				return fmt.Errorf("opción de apply desconocida: %s (valores: config, wallpaper)", rest[0])
			}
		}
	}

	// Las opciones comunes.
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "-c", "--config":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.conf = rest[i+1]
			i++
		case "-w", "--wallpaper":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.wallpaper = rest[i+1]
			i++
		case "--resolution", "--res":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.resolution = rest[i+1]
			i++
		case "--timeout":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.timeout, _ = strconv.Atoi(rest[i+1])
			i++
		case "--real":
			opts.real = true
		case "--blur":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.blur = rest[i+1]
			i++
		case "--dim":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.dim, _ = strconv.ParseFloat(rest[i+1], 64)
			i++
		case "--overlay":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.overlay = rest[i+1]
			i++
		case "--format":
			if i+1 >= len(rest) {
				return fmt.Errorf("falta el valor de %s", rest[i])
			}
			opts.format = rest[i+1]
			i++
		case "-h", "--help":
			usage()
			return nil
		default:
			return fmt.Errorf("opción desconocida: %s", rest[i])
		}
	}

	switch opts.command {
	case "add":
		return cmdAdd(opts, paths)
	case "edit":
		return cmdEdit(opts, paths)
	case "apply":
		return cmdApply(opts, paths)
	case "wallpapers":
		return cmdWallpapers(opts, paths)
	case "shot":
		return cmdShot(opts, paths)
	default:
		return cmdPreview(opts, paths)
	}
}

// ── add / edit ──────────────────────────────────────────────────────────────

// cmdAdd pone una imagen como wallpaper del preview.
func cmdAdd(opts options, paths cli.Paths) error {
	if _, err := os.Stat(opts.addImage); err != nil {
		return fmt.Errorf("no existe: %s", opts.addImage)
	}

	wallDir := filepath.Join(paths.LoaderConfig(), "wallpaper")
	if err := os.MkdirAll(wallDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(wallDir, "wallpaper.png")

	// Limine no puede difuminar: dibuja la imagen tal cual. Si el blur
	// está pedido, se aplica ANTES, al preparar el fichero.
	if opts.blur != "0x0" || opts.dim > 0 || opts.overlay != "" {
		cli.Step("Preparando el wallpaper")
		if err := wallpaper.Prepare(wallpaper.Options{
			Src:     opts.addImage,
			Out:     target,
			Blur:    opts.blur,
			Res:     opts.resolution,
			Format:  opts.format,
			Dim:     opts.dim,
			Overlay: opts.overlay,
		}); err != nil {
			return err
		}
	} else if err := cli.CopyFile(opts.addImage, target); err != nil {
		return err
	}

	cli.Ok("%s  ->  %s", opts.addImage, target)
	fmt.Println()
	cli.Say("Para verlo:  boot-preview loader")
	cli.Say("Para aplicarlo al Limine real:  boot-preview loader apply wallpaper")
	return nil
}

// cmdEdit abre la config del preview en un editor.
func cmdEdit(opts options, paths cli.Paths) error {
	editor := os.Getenv("VISUAL_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		for _, candidate := range []string{"code", "nvim", "micro", "nano", "vim", "helix", "zed"} {
			if cli.HaveCommand(candidate) {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no encuentro un editor (proba con EDITOR=...)")
	}

	conf := filepath.Join(paths.LoaderConfig(), "limine.conf")
	cli.Say("Abriendo %s con %s", conf, editor)

	cmd := exec.Command(editor, conf)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ── apply ───────────────────────────────────────────────────────────────────

// cmdApply aplica el tema al Limine real.
//
// El wallpaper va a /boot/splash.png (la ESP) y el bloque de theming a
// /boot/limine.conf, antes de la primera entrada.
//
// NO toca /etc/default/limine: ese es el origen, y Limine lo compila con
// limine-mkinitcpio. Reescribir el conf real directamente es lo que
// permite iterar sin recompilar el initramfs entero.
func cmdApply(opts options, paths cli.Paths) error {
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
	entries, err := os.ReadDir("/boot")
	if err != nil {
		return
	}

	var backups []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			backups = append(backups, filepath.Join("/boot", entry.Name()))
		}
	}
	if len(backups) <= keep {
		return
	}

	// ReadDir ya viene ordenado por nombre.
	for _, old := range backups[:len(backups)-keep] {
		_, _ = cli.RunCombined("sudo", "rm", "-f", old)
	}
}

// applyThemeBlock inyecta el bloque de theming en /boot/limine.conf.
func applyThemeBlock(paths cli.Paths) error {
	previewPath := filepath.Join(paths.LoaderConfig(), "limine.conf")

	if _, err := os.Stat("/boot/limine.conf"); err != nil {
		cli.Warn("No existe /boot/limine.conf")
		cli.Say("apply solo reescribe el bloque de theming; las entradas del")
		cli.Say("menú no se pueden reconstruir desde acá.")
		cli.Say("Restauralo desde un backup:")
		cli.Say("  sudo ls -1t /boot/limine.conf.bak-* /boot/limine.conf.old")
		cli.Say("  sudo cp /boot/limine.conf.bak-XXXX /boot/limine.conf")
		return fmt.Errorf("")
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
	for i, line := range block {
		if strings.HasPrefix(line, "wallpaper:") {
			block[i] = "wallpaper: boot():/splash.png"
		}
	}

	real, err := os.ReadFile("/boot/limine.conf")
	if err != nil {
		return fmt.Errorf("no se pudo leer /boot/limine.conf: %w", err)
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

// ── El preview ──────────────────────────────────────────────────────────────

// cmdPreview abre el menú en una ventana.
func cmdPreview(opts options, paths cli.Paths) error {
	if err := checkRequirements(opts); err != nil {
		return err
	}

	conf := opts.conf
	cleanup := func() {}
	if opts.real {
		// OJO: `sudo cat ... > archivo` y no `sudo cp ... archivo`. Con
		// cp el archivo queda con el modo de /boot/limine.conf
		// (-rwx------ root) y el usuario no lo puede leer en el paso
		// siguiente.
		tmp, err := os.CreateTemp("", "boot-preview-loader-real.*.conf")
		if err != nil {
			return err
		}
		tmp.Close()
		out, err := cli.RunCombined("sudo", "cat", "/boot/limine.conf")
		if err != nil {
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("no se pudo leer /boot/limine.conf (hace falta sudo): %s", out)
		}
		if err := os.WriteFile(tmp.Name(), []byte(out), 0o644); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
		conf = tmp.Name()
		cleanup = func() { _ = os.Remove(tmp.Name()) }
	}

	iso, err := buildISO(conf, opts)
	cleanup()
	if err != nil {
		return err
	}
	fmt.Printf("ISO listo: %s\n", iso)

	monitorSock := filepath.Join(workDir, "monitor.sock")
	qmpSock := filepath.Join(workDir, "qmp.sock")
	_ = os.Remove(monitorSock)
	_ = os.Remove(qmpSock)

	args := []string{
		"-enable-kvm", "-m", "512",
		"-cdrom", iso,
		"-monitor", "unix:" + monitorSock + ",server,nowait",
		"-qmp", "unix:" + qmpSock + ",server,nowait",
		"-boot", "d",
	}

	// UEFI (OVMF) — es lo que usa la máquina real, y el menú gráfico con
	// wallpaper solo funciona bien bajo UEFI. Sin esto, QEMU arranca con
	// SeaBIOS y Limine cae a modo texto.
	if _, err := os.Stat(ovmfCode); err == nil {
		if err := cli.CopyFile(ovmfVars, filepath.Join(workDir, "OVMF_VARS.fd")); err != nil {
			return err
		}
		args = []string{
			"-enable-kvm", "-m", "512",
			"-drive", "if=pflash,format=raw,readonly=on,file=" + ovmfCode,
			"-drive", "if=pflash,format=raw,file=" + filepath.Join(workDir, "OVMF_VARS.fd"),
			"-cdrom", iso,
			"-monitor", "unix:" + monitorSock + ",server,nowait",
			"-qmp", "unix:" + qmpSock + ",server,nowait",
			"-boot", "d",
		}
	} else {
		cli.Warn("sin OVMF, arranca con SeaBIOS y el menú puede salir en modo texto")
	}

	// El display es SDL y no GTK: la ventana es NATIVA del sistema, no un
	// GTK embebido. En Wayland funciona sin Xwayland.
	//
	// El backend SDL no trae Ctrl+Q (era del backend GTK), así que se mapea
	// con QMP: send-key inyecta ctrl+q en el QEMU y el quit de QMP lo
	// cierra.
	args = append(args, "-display", "sdl")

	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo arrancar QEMU: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Espera a que el monitor responda.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(monitorSock); err == nil {
			break
		}
		sleepMillis(200)
	}

	go mapCtrlQ(qmpSock)

	fmt.Println()
	fmt.Println("QEMU corriendo. Para salir: Ctrl+Q (o cerrá la ventana).")
	fmt.Printf("Monitor QEMU en: %s\n", monitorSock)

	_ = cmd.Wait()
	return nil
}

// checkRequirements verifica que estén las dependencias y los archivos.
func checkRequirements(opts options) error {
	for _, bin := range []string{"xorriso", "qemu-system-x86_64"} {
		if !cli.HaveCommand(bin) {
			return fmt.Errorf("falta: %s", bin)
		}
	}
	if _, err := os.Stat(opts.conf); err != nil {
		return fmt.Errorf("no existe la config: %s", opts.conf)
	}
	if _, err := os.Stat(opts.wallpaper); err != nil {
		return fmt.Errorf("no existe el wallpaper: %s", opts.wallpaper)
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		cli.Warn("sin /dev/kvm, la VM va a ir por emulación (lenta)")
	}
	return nil
}

// buildISO arma el árbol del ISO y lo construye.
func buildISO(conf string, opts options) (string, error) {
	if err := os.RemoveAll(workDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(workDir, "EFI", "BOOT"), 0o755); err != nil {
		return "", err
	}

	for _, file := range []string{"limine-uefi-cd.bin", "limine-bios-cd.bin", "limine-bios.sys"} {
		if err := cli.CopyFile(filepath.Join(limineDir, file), filepath.Join(workDir, file)); err != nil {
			return "", fmt.Errorf("falta %s en %s", file, limineDir)
		}
	}
	if err := cli.CopyFile(filepath.Join(limineDir, "BOOTX64.EFI"), filepath.Join(workDir, "EFI", "BOOT", "BOOTX64.EFI")); err != nil {
		return "", fmt.Errorf("falta BOOTX64.EFI en %s", limineDir)
	}

	// La config: se copia tal cual y se le inyecta el wallpaper y la
	// resolución.
	target := filepath.Join(workDir, "limine.conf")
	if err := cli.CopyFile(conf, target); err != nil {
		return "", err
	}
	if err := injectPreviewOptions(target, opts.resolution); err != nil {
		return "", err
	}

	if err := cli.CopyFile(opts.wallpaper, filepath.Join(workDir, "wallpaper.png")); err != nil {
		return "", err
	}

	// El ISO híbrido (UEFI + BIOS).
	iso := filepath.Join(workDir, "boot-preview-loader.iso")
	xorriso := exec.Command("xorriso", "-as", "mkisofs", "-R", "-r", "-J",
		"-b", "limine-bios-cd.bin",
		"-no-emul-boot", "-boot-load-size", "4", "-boot-info-table", "-hfsplus",
		"-apm-block-size", "2048", "--efi-boot", "limine-uefi-cd.bin",
		"-efi-boot-part", "--efi-boot-image", "--protective-msdos-label",
		".", "-o", iso)
	xorriso.Dir = workDir
	if out, err := xorriso.CombinedOutput(); err != nil {
		return "", fmt.Errorf("xorriso falló: %s", strings.TrimSpace(string(out)))
	}

	return iso, nil
}

// injectPreviewOptions mete las opciones del preview en el conf.
//
// OJO: en Limine las opciones GLOBALES tienen que ir antes de la primera
// entrada de menú (una línea que empieza con "/"). Si se inyectan al
// final, Limine las trata como opciones de esa entrada y las ignora.
func injectPreviewOptions(path, resolution string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Se borra el bloque inyectado en una corrida anterior (si lo hay),
	// para que la inyección sea idempotente: aplicarla dos veces tiene
	// que dejar el mismo archivo.
	lines := strings.Split(string(data), "\n")
	var kept []string
	skipping := false
	for _, line := range lines {
		if strings.TrimSpace(line) == injectComment {
			skipping = true
			continue
		}
		if skipping {
			// Todo lo que sigue al comentario hasta la primera línea con
			// contenido es parte del bloque.
			if strings.TrimSpace(line) == "" {
				continue
			}
			skipping = false
		}
		kept = append(kept, line)
	}
	text := strings.Join(kept, "\n")

	// Se borran las opciones de theming previas para que la config
	// inyectada gane.
	text = optionLineRe("wallpaper|wallpaper_style|interface_resolution").ReplaceAllString(text, "")
	// El timeout se fuerza a "no": si no, Limine autobootea la entrada
	// default (que en la config real apunta a un kernel con hash que no
	// está en el ISO) y se va de la pantalla antes de que puedas mirarla.
	text = optionLineRe("timeout").ReplaceAllString(text, "")

	inject := fmt.Sprintf(`%s
timeout: no
wallpaper: boot():/wallpaper.png
wallpaper_style: centered
interface_resolution: %s
`, injectComment, resolution)

	lines = strings.Split(text, "\n")
	insertAt := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "/") {
			insertAt = i
			break
		}
	}

	var out []string
	out = append(out, lines[:insertAt]...)
	out = append(out, strings.TrimRight(inject, "\n"))
	out = append(out, lines[insertAt:]...)

	result := strings.Join(out, "\n")
	// Las opciones borradas dejan lineas vacias: se colapsan para que el
	// archivo no crezca con cada inyeccion.
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return os.WriteFile(path, []byte(result+"\n"), 0o644)
}

// optionLineRe matchea una línea de opción completa.
func optionLineRe(names string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*(` + names + `)\s*:.*$\n?`)
}

// ── shot ────────────────────────────────────────────────────────────────────

// cmdShot captura el menú a un PNG.
func cmdShot(opts options, paths cli.Paths) error {
	if err := checkRequirements(opts); err != nil {
		return err
	}

	conf := opts.conf
	if opts.real {
		out, err := cli.RunCombined("sudo", "cat", "/boot/limine.conf")
		if err != nil {
			return fmt.Errorf("no se pudo leer /boot/limine.conf (hace falta sudo)")
		}
		tmp, err := os.CreateTemp("", "boot-preview-loader-real.*.conf")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		tmp.Close()
		if err := os.WriteFile(tmp.Name(), []byte(out), 0o644); err != nil {
			return err
		}
		conf = tmp.Name()
	}

	iso, err := buildISO(conf, opts)
	if err != nil {
		return err
	}

	monitorSock := filepath.Join(workDir, "monitor.sock")
	_ = os.Remove(monitorSock)

	// Sin GUI no hace falta ningún display: el screendump va por el monitor.
	args := []string{
		"-enable-kvm", "-m", "512",
		"-cdrom", iso,
		"-monitor", "unix:" + monitorSock + ",server,nowait",
		"-boot", "d",
		"-display", "none",
	}
	if _, err := os.Stat(ovmfCode); err == nil {
		if err := cli.CopyFile(ovmfVars, filepath.Join(workDir, "OVMF_VARS.fd")); err != nil {
			return err
		}
		args = []string{
			"-enable-kvm", "-m", "512",
			"-drive", "if=pflash,format=raw,readonly=on,file=" + ovmfCode,
			"-drive", "if=pflash,format=raw,file=" + filepath.Join(workDir, "OVMF_VARS.fd"),
			"-cdrom", iso,
			"-monitor", "unix:" + monitorSock + ",server,nowait",
			"-boot", "d",
			"-display", "none",
		}
	}

	cmd := exec.Command("qemu-system-x86_64", args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo arrancar QEMU: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Espera a que el monitor responda.
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(monitorSock); err == nil {
			break
		}
		sleepMillis(200)
	}

	// En vez de esperar un tiempo fijo, se le pregunta al monitor si
	// Limine ya dibujó el menú. Ahorra ~10 s de los 15 que costaba antes.
	//
	// La señal NO es "hay algo pintado": el logo de TianoCore (OVMF) ya
	// cuenta como pintado, y se terminaba capturando el logo en vez del
	// menú. La señal buena es el CAMBIO DE RESOLUCIÓN: OVMF arranca en
	// 1280x800 y Limine la cambia a la interface_resolution al tomar el
	// control.
	fmt.Printf("Esperando a que arranque el menu (max %ds)...\n", opts.timeout)

	raw := filepath.Join(workDir, "shot.ppm")
	_ = os.Remove(raw)

	deadline := nowSeconds() + int64(opts.timeout)
	settled := false

	for {
		_ = screendump(monitorSock, raw)

		if size, ok := imageSize(raw); ok && size == opts.resolution {
			// Ya es la resolución del menú: falta que termine de pintar
			// (Limine dibuja el fondo y después el texto).
			if settled {
				break
			}
			settled = true
			sleepMillis(500)
			continue
		}

		if nowSeconds() >= deadline {
			fmt.Println("  se agoto el tiempo de espera, capturando lo que haya")
			break
		}
		sleepMillis(300)
	}

	if _, err := os.Stat(raw); err != nil {
		return fmt.Errorf("no se pudo capturar el framebuffer")
	}

	// Sin nombre: va a <repo>/previews/loader/ con timestamp.
	out := opts.shotPath
	if out == "" {
		dir := paths.LoaderPreviews()
		if dir == "" {
			dir = workDir
		}
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, "loader-"+timestamp()+".png")
	} else if !filepath.IsAbs(out) && paths.Repo != "" {
		dir := paths.LoaderPreviews()
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, filepath.Base(out))
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}

	// vips es ~6x más rápido que ImageMagick para esto.
	if err := convertToPNG(raw, out); err != nil {
		return err
	}
	fmt.Printf("Captura: %s\n", out)
	return nil
}

// screendump le pide un volcado del framebuffer al monitor de QEMU.
func screendump(monitorSock, dest string) error {
	cmd := exec.Command("socat", "-", "UNIX-CONNECT:"+monitorSock)
	cmd.Stdin = strings.NewReader("screendump " + dest + "\n")
	return cmd.Run()
}

// convertToPNG convierte el PPM a PNG.
func convertToPNG(src, dest string) error {
	if cli.HaveCommand("vips") {
		if _, err := cli.RunCombined("vips", "copy", src, dest); err == nil {
			return nil
		}
	}
	if cli.HaveCommand("magick") {
		if _, err := cli.RunCombined("magick", src, dest); err == nil {
			return nil
		}
	}
	return cli.CopyFile(src, dest)
}

// imageSize devuelve el tamaño de una imagen como "WxH".
func imageSize(path string) (string, bool) {
	if cli.HaveCommand("vipsheader") {
		w, errW := cli.Run("vipsheader", "-f", "width", path)
		h, errH := cli.Run("vipsheader", "-f", "height", path)
		if errW == nil && errH == nil {
			return w + "x" + h, true
		}
	}
	if cli.HaveCommand("magick") {
		out, err := cli.Run("magick", "identify", "-format", "%wx%h", path)
		if err == nil {
			return out, true
		}
	}
	return "", false
}

// ── wallpapers ──────────────────────────────────────────────────────────────

// cmdWallpapers genera las dos versiones del wallpaper.
//
// Limine NO puede difuminar una imagen: la dibuja tal cual. Así que el
// blur se aplica ANTES, y lo que llega a Limine ya viene difuminado.
func cmdWallpapers(opts options, paths cli.Paths) error {
	wallDir := filepath.Join(paths.LoaderConfig(), "wallpaper")
	src := filepath.Join(wallDir, "rei.png")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("no hay wallpaper de ejemplo en %s", src)
	}

	cli.Step("La real (más detalle, sin blur)")
	real := filepath.Join(wallDir, "rei.png")
	if err := wallpaper.Prepare(wallpaper.Options{Src: src, Out: real, Blur: "0x0", Format: "png"}); err != nil {
		return err
	}

	cli.Step("La difuminada (ya viene con el blur aplicado)")
	blurred := filepath.Join(wallDir, "rei-blur.png")
	if err := wallpaper.Prepare(wallpaper.Options{Src: src, Out: blurred, Blur: "0x30", Format: "png"}); err != nil {
		return err
	}

	fmt.Println()
	cli.Say("  %s", real)
	cli.Say("  %s", blurred)
	fmt.Println()
	cli.Say("  La difuminada quedo como la activa (wallpaper.png):")
	cli.Say("      boot-preview loader                  → preview con ella")
	cli.Say("      boot-preview loader apply            → wallpaper + config al Limine real")
	fmt.Println()
	cli.Say("  Para volver a la nitida:")
	cli.Say("      cp %s %s", real, filepath.Join(wallDir, "wallpaper.png"))
	cli.Say("      boot-preview loader apply wallpaper")
	return nil
}

// ── Utilidades ──────────────────────────────────────────────────────────────

// copyFile copia un archivo.

// writeFile escribe un archivo, con sudo si hace falta.
func writeFile(path, content string, needsSudo bool) error {
	if !needsSudo {
		return os.WriteFile(path, []byte(content), 0o644)
	}
	tmp, err := os.CreateTemp("", "boot-preview-write.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		return err
	}
	tmp.Close()
	_, err = cli.RunCombined("sudo", "cp", tmp.Name(), path)
	return err
}

func usage() {
	fmt.Print(`boot-preview loader — ver el menú de Limine sin reiniciar

USO
  boot-preview loader [comando] [opciones]

VER
  (ninguno)        Preview del menú, con la config de prueba
  real             Preview con tu config real de /boot/limine.conf
  wall <imagen>    Preview con otro wallpaper (sin instalarlo)

CAPTURAR
  shot [archivo]   Captura el menú a un PNG. Sin nombre va a
                   <repo>/previews/loader/ con timestamp

EDITAR
  add <imagen>     Pone una imagen como wallpaper del preview
                   (con --blur/--dim/--overlay la prepara antes)
  edit             Abre la config del preview en tu editor

APLICAR AL LIMINE REAL
  apply             Wallpaper + config visual
  apply config      Solo la config visual
  apply wallpaper   Solo el wallpaper

WALLPAPERS
  wallpapers       Genera las dos versiones (real y difuminada)

OTROS
  help             Esta ayuda

INSTALACIÓN (de todo el tool)
  boot-preview setup

EJEMPLOS
  boot-preview loader
  boot-preview loader shot
  boot-preview loader add ~/Pictures/foto.png
  boot-preview loader edit
  boot-preview loader apply

OPCIONES AVANZADAS
  -c, --config <ruta>     Config de Limine a usar
  -w, --wallpaper <ruta>  Wallpaper a incluir en el ISO
      --resolution <WxH>  Resolución de la interfaz (default: 1920x1080)
      --timeout <seg>     Espera antes de capturar (default: 15)

PREPARAR EL WALLPAPER (para add)
  Limine no puede difuminar: el blur se aplica ANTES, a la imagen.
      --blur 0x30          Desenfoque gaussiano (0xRADIO). 0x0 = sin desenfoque
      --dim 0.4            Velo negro uniforme encima (0 = nada, 1 = negro)
      --overlay "#191724"  Velo de ese color en vez de negro
      --format qoi         QOI: lossless y se decodifica más rápido que PNG
`)
}
