// Package animation previsualiza el splash de Plymouth.
//
// Usa el Plymouth oficial del sistema (plymouthd + sus plugins) con el
// renderer X11: dibuja en una ventana GTK sobre tu escritorio. No es un
// reproductor de imágenes: es el mismo daemon, los mismos plugins y el
// mismo tema que usa la máquina al arrancar. Sin VM, sin initramfs.
package animation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
	"github.com/Below-Z3R0/boot-preview/internal/steam"
)

const (
	themesDir = "/usr/share/plymouth/themes"
	sysConf   = "/etc/plymouth/plymouthd.conf"
	x11Log    = "/tmp/plymouth-x11.log"

	// refreshRate es el FRAMES_PER_SECOND del plugin "script" de Plymouth.
	// El script se refresca a esa frecuencia, y de ahí sale el timing.
	refreshRate = 50
)

// Run ejecuta el comando animation.
func Run(args []string, paths cli.Paths) error {
	cmd := "x11"
	rest := args

	if len(args) > 0 {
		switch args[0] {
		case "x11", "shot", "access", "sources", "preview", "use",
			"steam-source", "steam-convert", "steam-apply", "help", "-h", "--help":
			cmd = args[0]
			rest = args[1:]
		default:
			// Un subcomando que no existe tiene que avisar, no abrir la
			// ventana como si fuera el preview.
			return fmt.Errorf("opción desconocida: %s", args[0])
		}
	}

	switch cmd {
	case "x11":
		return cmdX11(rest, paths)
	case "shot":
		return cmdShot(rest, paths)
	case "access":
		return cmdAccess()
	case "sources":
		return cmdSources(paths)
	case "preview":
		return cmdPreview(rest, paths)
	case "use":
		return cmdUse(rest, paths)
	case "steam-source":
		return cmdSteamSource()
	case "steam-convert":
		return cmdSteamConvert(rest, paths, false)
	case "steam-apply":
		return cmdSteamConvert(rest, paths, true)
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		// Sin subcomando: la ventana con el splash.
		return cmdX11(rest, paths)
	}
}

// ── El tema ─────────────────────────────────────────────────────────────────

// systemTheme es el tema que Plymouth usa ahora.
func systemTheme() string {
	data, err := os.ReadFile(sysConf)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Theme") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "Theme"))
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(rest, "="))
	}
	return ""
}

// previewTheme es el tema que usa el preview (o el del sistema, si el
// usuario no eligió otro).
func previewTheme(paths cli.Paths) string {
	data, err := os.ReadFile(filepath.Join(paths.AnimationConfig(), "theme"))
	if err != nil {
		return systemTheme()
	}
	return strings.TrimSpace(string(data))
}

// ── sources ────────────────────────────────────────────────────────────────

// cmdSources lista los temas de Plymouth instalados.
func cmdSources(paths cli.Paths) error {
	actual := systemTheme()
	preview := previewTheme(paths)

	cli.Step("Temas de Plymouth instalados")
	cli.Say("directorio: %s/", themesDir)
	fmt.Println()

	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return fmt.Errorf("no se pudo leer %s: %w", themesDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		module := themeModule(filepath.Join(themesDir, name))

		var marca string
		if name == actual {
			marca += " " + cli.Green("[sistema]")
		}
		if name == preview {
			marca += " " + cli.Green("[preview]")
		}
		if module == "" {
			module = "?"
		}
		fmt.Printf("    %-30s %-12s%s\n", name, module, marca)
	}

	fmt.Println()
	cli.Say("sistema: %s", actual)
	cli.Say("preview: %s", preview)
	return nil
}

// themeModule lee el ModuleName del .plymouth de un tema.
// themeModule devuelve el módulo de Plymouth de un tema.
//
// Si no lo dice el .plymouth, se asume "script": es el único que sabe
// interpretar una secuencia de PNGs, y es el que usa Plymouth por
// defecto cuando el .plymouth no especifica nada.
func themeModule(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "script"
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".plymouth") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line != "ModuleName" && !strings.HasPrefix(line, "ModuleName=") {
				continue
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, "ModuleName"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "="))
			if rest != "" {
				return rest
			}
		}
	}
	return "script"
}

// ── preview / use ───────────────────────────────────────────────────────────

// cmdPreview cambia el tema SOLO del preview.
func cmdPreview(args []string, paths cli.Paths) error {
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
func cmdUse(args []string, paths cli.Paths) error {
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

// ── access ──────────────────────────────────────────────────────────────────

// cmdAccess da acceso de root al servidor X.
//
// plymouthd exige root, y root solo puede abrir ventanas si el servidor X
// le da acceso. Sin esto el splash no aparece y no hay error.
func cmdAccess() error {
	cli.Step("Dando acceso de root al servidor X")

	display := os.Getenv("DISPLAY")
	if display == "" {
		cli.Warn("No hay DISPLAY. X11 necesita un servidor gráfico.")
		cli.Say("En Wayland usually usás el Xwayland: export DISPLAY=:1")
		return fmt.Errorf("")
	}

	if x11RootAccess() {
		cli.Ok("root ya puede abrir ventanas en %s", display)
		return nil
	}

	if !cli.HaveCommand("xhost") {
		cli.Warn("Falta xhost (paquete xorg-xhost).")
		cli.Say("Instalalo con: sudo pacman -S xorg-xhost")
		return fmt.Errorf("")
	}

	cli.Say("Ejecutando: xhost +SI:localuser:root")
	if out, err := cli.RunCombined("xhost", "+SI:localuser:root"); err != nil {
		cli.Warn("No se pudo dar el acceso: %s", out)
		cli.Say("Probá manualmente:  DISPLAY=%s xhost +SI:localuser:root", display)
		return fmt.Errorf("")
	}

	cli.Ok("root puede abrir ventanas en %s", display)
	cli.Say("El permiso dura hasta que cierres la sesión gráfica.")
	return nil
}

// x11RootAccess dice si root puede conectarse al servidor X.
func x11RootAccess() bool {
	cmd := exec.Command("sudo", "xdotool", "getdisplaygeometry")
	cmd.Env = append(os.Environ(), "DISPLAY="+os.Getenv("DISPLAY"), "XAUTHORITY="+os.Getenv("XAUTHORITY"))
	return cmd.Run() == nil
}

// ── El preview en X11 ───────────────────────────────────────────────────────

// cmdX11 abre la ventana con el splash.
func cmdX11(args []string, paths cli.Paths) error {
	seconds := 0
	if len(args) > 0 {
		seconds, _ = strconv.Atoi(args[0])
	}

	theme := previewTheme(paths)
	if _, err := os.Stat(filepath.Join(themesDir, theme)); err != nil {
		return fmt.Errorf("no existe el tema: %s", theme)
	}

	if os.Getenv("DISPLAY") == "" {
		cli.Warn("No hay DISPLAY. X11 necesita un servidor gráfico.")
		cli.Say("En Wayland: export DISPLAY=:1 (Xwayland)")
		return fmt.Errorf("")
	}

	cli.Step("Plymouth en X11 con el tema '%s'", theme)

	if !x11RootAccess() {
		cli.Warn("root no puede abrir una ventana en %s (control de acceso de X).", os.Getenv("DISPLAY"))
		fmt.Println()
		cli.Say("Solución (una vez por sesión):")
		cli.Say("  boot-preview animation access")
		fmt.Println()
		cli.Say("Eso le da a root acceso al servidor X. Plymouth dibuja en una")
		cli.Say("ventana GTK sobre tu escritorio, con el tema real.")
		return fmt.Errorf("")
	}

	proc, err := startPlymouthX11(theme, paths)
	if err != nil {
		return err
	}
	defer proc.cleanup()

	cli.Say("")
	cli.Say("Ventana abierta con el tema '%s'.", theme)
	cli.Say("Desde OTRA terminal:")
	cli.Say("  sudo plymouth --reload        # recargar tras editar un PNG")
	cli.Say("  sudo plymouth --show-splash   # relanzar la animación")
	cli.Say("  sudo plymouth --quit          # cerrar")
	cli.Say("")
	cli.Say("Ctrl+C para cerrar.")

	if seconds == 0 {
		proc.wait()
		return nil
	}

	for i := 0; i < seconds; i++ {
		if !proc.alive() {
			break
		}
		sleep(time.Second)
		// La salida va a /dev/null y no a un pipe: Plymouth se comporta
		// distinto si su stdout no es una terminal (deja de dibujar).
		updateSplash("Iniciando " + strconv.Itoa(i+1))
	}
	return nil
}

// plymouthProcess es el plymouthd corriendo en segundo plano.
type plymouthProcess struct {
	cmd      *exec.Cmd
	etcDir   string
	runTheme string
	devNull  *os.File
}

// alive dice si el proceso sigue corriendo.
//
// NO se usa Process.Signal(nil): en Go devuelve "unsupported signal type"
// (solo se puede enviar una señal real), así que siempre daría false.
// syscall.Kill(pid, 0) es la forma POSIX de consultar si el proceso existe:
// no le manda nada, solo pregunta.
func (p *plymouthProcess) alive() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	return syscall.Kill(p.cmd.Process.Pid, 0) == nil
}

// wait espera a que termine.
func (p *plymouthProcess) wait() {
	if p.cmd != nil {
		_ = p.cmd.Wait()
	}
}

// cleanup cierra el daemon y borra lo que se puso.
func (p *plymouthProcess) cleanup() {
	_, _ = cli.RunCombined("sudo", "plymouth", "--quit")
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_, _ = cli.RunCombined("sudo", "rm", "-rf", p.runTheme)
	}
	if p.etcDir != "" {
		_ = os.RemoveAll(p.etcDir)
	}
	// El descriptor de /dev/null se cierra recién acá: si se cerrara antes,
	// Plymouth vería su stdout cerrado y se apagaría.
	if p.devNull != nil {
		_ = p.devNull.Close()
	}
}

// startPlymouthX11 monta el tema y arranca el daemon.
//
// El tema va a /run/plymouth/themes, que Plymouth mira ANTES que /usr/share.
// Y la config se monta sobre /etc/plymouth en un namespace de mounts
// privado: el sistema NO se toca.
func startPlymouthX11(theme string, paths cli.Paths) (*plymouthProcess, error) {
	runTheme := filepath.Join("/run/plymouth/themes", theme)

	// El tema a /run.
	if _, err := cli.RunCombined("sudo", "mkdir", "-p", "/run/plymouth/themes"); err != nil {
		return nil, fmt.Errorf("no se pudo crear /run/plymouth/themes")
	}
	_, _ = cli.RunCombined("sudo", "rm", "-rf", runTheme)
	if out, err := cli.RunCombined("sudo", "cp", "-r", filepath.Join(themesDir, theme), "/run/plymouth/themes/"); err != nil {
		return nil, fmt.Errorf("no se pudo copiar el tema: %s", out)
	}

	// El .plymouth del tema en /run debe apuntar a SI MISMO (ImageDir y
	// ScriptFile), si no Plymouth lee los frames del directorio equivocado.
	plyFile := filepath.Join(runTheme, theme+".plymouth")
	if _, err := os.Stat(plyFile); err == nil {
		data, err := os.ReadFile(plyFile)
		if err == nil {
			content := string(data)
			content = replaceLine(content, "ImageDir", runTheme)
			content = replaceLine(content, "ScriptFile", filepath.Join(runTheme, "script.script"))
			tmp := plyFile + ".tmp"
			if err := os.WriteFile(tmp, []byte(content), 0o644); err == nil {
				_, _ = cli.RunCombined("sudo", "mv", tmp, plyFile)
			}
		}
	}

	// La config propia, montada sobre /etc/plymouth en un namespace
	// privado: el sistema NO se toca.
	etcDir, err := os.MkdirTemp("", "boot-preview-animation-etc.*")
	if err != nil {
		return nil, err
	}
	conf := fmt.Sprintf("[Daemon]\nTheme=%s\nShowDelay=0.5\nDeviceTimeout=10\n", theme)
	if err := os.WriteFile(filepath.Join(etcDir, "plymouthd.conf"), []byte(conf), 0o644); err != nil {
		return nil, err
	}

	_, _ = cli.RunCombined("sudo", "plymouth", "--quit")
	sleep(time.Second)

	// El daemon, dentro del namespace de mounts.
	script := fmt.Sprintf(
		"mount --bind '%s' /etc/plymouth && exec %s plymouthd --no-daemon --debug --debug-file='%s' --mode=boot",
		etcDir, envPrefix(), x11Log)

	cmd := exec.Command("sudo", "unshare", "-m", "--propagation", "private", "bash", "-c", script)
	// plymouthd con --debug vuelca todo a stdout: se descarta porque el
	// log verbose ya va al archivo.
	//
	// La salida va a /devNull y NO se deja heredada: si el daemon
	// escribiera en un pipe se bloquearía al llenarse (nadie lo lee) y
	// Plymouth deja de dibujar. Con un archivo o /dev/null no hay eso.
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir %s: %w", os.DevNull, err)
	}
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	if err := cmd.Start(); err != nil {
		devNull.Close()
		return nil, fmt.Errorf("no se pudo arrancar plymouthd: %w", err)
	}
	// NO se cierra el descriptor aquí: el daemon lo tiene abierto y Plymouth
	// se apaga en cuanto su stdout se cierra. Se cierra cuando el proceso
	// termina (lo hace cleanup, a través de Wait).

	proc := &plymouthProcess{cmd: cmd, etcDir: etcDir, runTheme: runTheme, devNull: devNull}
	sleep(3 * time.Second)

	if !proc.alive() {
		proc.cleanup()
		cli.Warn("plymouthd terminó. Log:")
		printLogTail(x11Log)
		return nil, fmt.Errorf("")
	}

	// Mostrar el splash.
	//
	// NO se usa RunCombined: ese captura la salida por un pipe, y Plymouth
	// se comporta distinto si su stdout no es una tty (deja de dibujar).
	// Con la salida a /dev/null se comporta como con el shell.
	showSplash()

	return proc, nil
}

// showSplash le pide al daemon que muestre el splash.
//
// La salida va a /dev/null (no a un pipe): Plymouth se comporta distinto
// si su stdout no es una terminal.
func showSplash() {
	runPlymouth("--show-splash")
}

// updateSplash le manda un mensaje al splash en curso.
//
// Plymouth redibuja el splash cuando recibe un update. Sin esto el
// primer frame puede no llegar a pintarse.
func updateSplash(message string) {
	runPlymouth("--update=" + message)
}

// runPlymouth ejecuta un subcomando de plymouth como root.
//
// La salida va a /dev/null y NO por un pipe: Plymouth se comporta distinto
// si su stdout no es una terminal (deja de dibujar). Además, un pipe sin
// lector se llena y bloquea el daemon.
func runPlymouth(args ...string) {
	cmd := exec.Command("sudo", append([]string{"--preserve-env=DISPLAY,XAUTHORITY", "plymouth"}, args...)...)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
		defer devNull.Close()
	}
	_ = cmd.Run()
}

// envPrefix arma el prefijo de entorno para el daemon.
//
// sudo NO preserva DISPLAY, y sin él el renderer x11.so no puede abrir la
// ventana GTK.
//
// DISPLAY se pasa SIEMPRE, aunque el proceso no lo tenga: el
// daemon corre como root y su entorno viene del shell que lo
// lanzó.
func envPrefix() string {
	var parts []string
	// Se pasan SOLO estas variables. WAYLAND_DISPLAY no va a propósito:
	// el renderer x11.so usa GTK, y si GTK encuentra un servidor Wayland
	// elige ese backend en vez del X11. La ventana se abriría en la
	// sesión Wayland (donde no la ve el `import` de X) y el preview
	// parecería no hacer nada.
	//
	// El daemon corre como root: su entorno viene del shell que lo
	// lanzó, así que hay que pasarle el DISPLAY a mano.
	for _, name := range []string{"DISPLAY", "XAUTHORITY", "XDG_RUNTIME_DIR"} {
		if value := os.Getenv(name); value != "" {
			parts = append(parts, name+"="+shellQuote(value))
		}
	}
	// TERM siempre: sin él Plymouth/GTK puede fallar al inicializar.
	term := os.Getenv("TERM")
	if term == "" {
		term = "unknown"
	}
	parts = append(parts, "TERM="+shellQuote(term))
	return "env " + strings.Join(parts, " ")
}

// shellQuote entrecomilla un valor para el shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// replaceLine cambia el valor de una clave del .plymouth.
func replaceLine(content, key, value string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Solo la clave exacta: "ImageDir" no debe pisar "ImageDirBackup".
		if line != key && !strings.HasPrefix(line, key+"=") {
			continue
		}
		lines[i] = key + "=" + value
		break
	}
	return strings.Join(lines, "\n")
}

// printLogTail muestra las últimas líneas del log de plymouthd.
func printLogTail(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	for _, line := range lines {
		cli.Say("    %s", line)
	}
}

// ── shot ────────────────────────────────────────────────────────────────────

// cmdShot captura el splash a un PNG.
func cmdShot(args []string, paths cli.Paths) error {
	out := ""
	if len(args) > 0 {
		out = args[0]
	}

	theme := previewTheme(paths)
	if _, err := os.Stat(filepath.Join(themesDir, theme)); err != nil {
		return fmt.Errorf("no existe el tema: %s", theme)
	}

	if os.Getenv("DISPLAY") == "" {
		cli.Warn("No hay DISPLAY. X11 necesita un servidor gráfico.")
		return fmt.Errorf("")
	}
	if !cli.HaveCommand("import") {
		cli.Warn("Falta ImageMagick (import).")
		return fmt.Errorf("")
	}
	if !x11RootAccess() {
		cli.Warn("root no puede abrir una ventana en %s.", os.Getenv("DISPLAY"))
		cli.Say("Solución: boot-preview animation access")
		return fmt.Errorf("")
	}

	proc, err := startPlymouthX11(theme, paths)
	if err != nil {
		return err
	}
	defer proc.cleanup()

	// La ventana de Plymouth se llama "plymouthd".
	wid := findPlymouthWindow()
	if wid == "" {
		cli.Warn("No encontré la ventana de Plymouth.")
		printLogTail(x11Log)
		return fmt.Errorf("")
	}

	cli.Step("Capturando (ventana %s)", wid)

	tmpDir, err := os.MkdirTemp("", "plymouth-shot.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	bestSize := 0
	bestFrame := ""
	var candidates []string

	// 8 muestras cada 500 ms: 4 s de animación.
	for i := 1; i <= 8; i++ {
		sleep(500 * time.Millisecond)
		png := filepath.Join(tmpDir, fmt.Sprintf("shot-%d.png", i))
		if err := captureWindow(wid, png); err != nil {
			continue
		}
		info, err := os.Stat(png)
		if err != nil || info.Size() == 0 {
			continue
		}
		// Un frame negro pesa ~2 KB; uno con contenido, 200 KB o más.
		size := int(info.Size())
		if size > 20000 {
			candidates = append(candidates, png)
		}
		if size > bestSize {
			bestSize = size
			bestFrame = png
		}
	}

	if bestFrame == "" || bestSize < 20000 {
		return fmt.Errorf("no se pudo capturar el splash (todas las frames salieron vacías)")
	}

	if out == "" {
		dir := paths.AnimationPreviews()
		if dir == "" {
			dir = paths.AnimationConfig()
		}
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, "animation-"+timeStamp()+".png")
	} else if !filepath.IsAbs(out) && paths.Repo != "" {
		dir := paths.AnimationPreviews()
		_ = os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, filepath.Base(out))
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := cli.CopyFile(bestFrame, out); err != nil {
		return err
	}

	fmt.Printf("Captura: %s  (%s)\n", out, cli.FileSize(out))

	// Contact sheet con 4 frames DISTRIBUIDOS en el tiempo: así se ve cómo
	// evoluciona la animación, no solo el frame más pesado.
	if len(candidates) >= 4 && cli.HaveCommand("magick") {
		sheet := strings.TrimSuffix(out, ".png") + "-sheet.png"
		pick := spread(candidates, 4)
		args := append(append([]string{}, pick...), "-resize", "640x360", "+append", "-append", sheet)
		if _, err := cli.RunCombined("magick", args...); err == nil {
			fmt.Printf("Sheet:   %s  (%s)\n", sheet, cli.FileSize(sheet))
		}
	}

	return nil
}

// findPlymouthWindow busca la ventana de Plymouth por su nombre.
func findPlymouthWindow() string {
	// DISPLAY se pasa explícitamente: xdotool no lo busca por su cuenta y
	// si el proceso que lo invoca no lo tiene en su entorno, busca en :0.
	cmd := exec.Command("xdotool", "search", "--name", "^plymouthd$")
	cmd.Env = append(os.Environ(), "DISPLAY="+os.Getenv("DISPLAY"))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if id := strings.TrimSpace(line); id != "" {
			return id
		}
	}
	return ""
}

// captureWindow captura una ventana X11 a un PNG.
func captureWindow(wid, dest string) error {
	cmd := exec.Command("import", "-window", wid, dest)
	cmd.Env = append(os.Environ(), "DISPLAY="+os.Getenv("DISPLAY"))
	return cmd.Run()
}

// spread toma n elementos distribuidos a lo largo de la lista.
func spread(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	var picked []string
	for i := 0; i < n; i++ {
		pos := i * (len(list) - 1) / (n - 1)
		picked = append(picked, list[pos])
	}
	return picked
}

// ── Steam ───────────────────────────────────────────────────────────────────

// cmdSteamSource lista las animaciones de Steam.
func cmdSteamSource() error {
	home := cli.Home()
	movies := steam.Movies(home)
	active := steam.ActiveVideo(home)

	cli.Step("Animaciones de inicio de Steam")

	if len(movies) > 0 {
		cli.Say("Compradas en la tienda:")
		for _, m := range movies {
			if !m.Purchased {
				continue
			}
			if m.Active {
				fmt.Printf("    %s %-32s (%s)  ← la activa\n", cli.Green("*"), m.Name, m.ID)
			} else {
				fmt.Printf("      %-32s (%s)\n", m.Name, m.ID)
			}
		}
		fmt.Println()
	}

	cli.Say("Del cliente (Steam Big Picture / Deck):")
	for _, m := range movies {
		if m.Purchased {
			continue
		}
		if m.Active {
			fmt.Printf("    %s %-44s ← la activa\n", cli.Green("*"), m.Name)
		} else {
			fmt.Printf("      %-44s\n", m.Name)
		}
	}
	fmt.Println()

	cli.Say("Activa: %s", orNone(active))
	cli.Say("Steam las guarda en config/communityitemscache/startupmovies/")
	return nil
}

// orNone devuelve "ninguna" si el string está vacío.
func orNone(s string) string {
	if s == "" {
		return "ninguna"
	}
	return s
}

func usage() {
	fmt.Print(`boot-preview animation — ver y convertir splashes de Plymouth sin reiniciar

Usa el Plymouth OFICIAL del sistema (plymouthd + sus plugins) con el
renderer X11: dibuja en una ventana GTK sobre tu escritorio. No es un
reproductor de imágenes: es el mismo daemon, los mismos plugins y el mismo
tema que usa la máquina al arrancar. Sin VM, sin initramfs.

USO
  boot-preview animation [comando] [opciones]

VER
  (ninguno)              Ventana con el splash
  x11 [segundos]         Idem, con límite de tiempo (0 = hasta Ctrl+C)

CAPTURAR
  shot [archivo]         Captura el splash a un PNG + contact sheet.
                         Sin nombre va a <repo>/previews/animation/

ACCESO
  access                 Da acceso de root al servidor X (una vez por
                         sesión). Sin esto el splash no aparece y no hay
                         error: plymouthd exige root y root necesita
                         permiso de X.

TEMAS
  sources                Los temas de Plymouth instalados
  preview <tema>         Cambia el tema SOLO del preview
  use <tema>             Aplica el tema al SISTEMA (-R, regenera el
                         initramfs)

STEAM
  steam-source           Los videos de startup (cuál está activo)
  steam-convert <vid>   Convierte un video en tema de Plymouth
  steam-apply <vid>     Convierte y lo aplica al sistema

OTROS
  help                   Esta ayuda

EJEMPLOS
  boot-preview animation
  boot-preview animation access
  boot-preview animation shot
  boot-preview animation sources
  boot-preview animation steam-convert

AJUSTES DE LA CONVERSIÓN
  STEAM_FPS=15 STEAM_RES=1280x720 boot-preview animation steam-convert
                         15 fps a 720p: 105 frames, 45 MB. A 30 fps y
                         1080p son 210 frames y 133 MB.
`)
}
