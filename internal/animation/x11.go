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
)

// ── El preview en X11 ────────────────────────────────────────────────────────

// ── El preview en X11 ───────────────────────────────────────────────────────

// cmdX11 abre la ventana con el splash.
func cmdX11(args []string, paths cli.Paths) error {
	// El daemon queda en background: el comando devuelve el control y la
	// ventana sigue viva. Se cierra con 'boot-preview plymouth quit'.
	//
	// Un límite de tiempo es opcional: con él, el preview se cierra solo.
	seconds := 0
	detach := true

	for _, arg := range args {
		switch {
		case arg == "--foreground", arg == "-f":
			detach = false
		default:
			if v, err := strconv.Atoi(arg); err == nil {
				seconds = v
			}
		}
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

	// En background: el daemon se queda vivo después de que el comando
	// termine. En foreground, el comando espera a que termine.
	if !detach {
		defer proc.cleanup()
		if err := proc.start(false); err != nil {
			return err
		}

		cli.Say("")
		cli.Say("Ventana abierta con el tema '%s'.", theme)
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
			updateSplash("Iniciando " + strconv.Itoa(i+1))
		}
		return nil
	}

	// El daemon se desacopla: se cierra de su sesión para que no
	// dependa de la terminal que lo lanzó, y se anota el PID para poder
	// controlarlo después.
	if err := proc.detach(); err != nil {
		proc.cleanup()
		return err
	}
	if err := proc.start(true); err != nil {
		return err
	}

	fmt.Println()
	cli.Ok("Ventana abierta con el tema '%s'.", theme)
	cli.Say("")
	cli.Say("El daemon quedó en background. La ventana sigue viva aunque")
	cli.Say("cierres la terminal.")
	fmt.Println()
	cli.Say("Para controlarlo:")
	cli.Say("  boot-preview plymouth status      # ver el estado")
	cli.Say("  boot-preview plymouth reload      # recargar tras editar un PNG")
	cli.Say("  boot-preview plymouth update '...' # cambiar el mensaje")
	cli.Say("  boot-preview plymouth quit        # cerrar el daemon")
	fmt.Println()
	cli.Say("Para cerrarlo también:  boot-preview animation stop")
	return nil
}

// plymouthProcess es el plymouthd corriendo en segundo plano.
type plymouthProcess struct {
	cmd      *exec.Cmd
	etcDir   string
	runTheme string
	devNull  *os.File
	pidFile  string
}

// detach saca el daemon de la sesión de la terminal.
//
// Un proceso que muere con su terminal (SIGHUP) no sirve: el preview
// tiene que sobrevivir al cierre de la shell que lo lanzó.
//
// Se hace con setsid: el proceso se pone en su propia sesión, y con
// eso deja de recibir el SIGHUP de la terminal. El PID se anota para
// poder controlarlo después con 'boot-preview plymouth'.
func (p *plymouthProcess) detach() error {
	// El PID ya lo anota el script del daemon (ver startPlymouthX11).
	return nil
}

// pidFilePath es donde se anota el PID del daemon.
//
// Es /tmp y no os.TempDir(): si TMPDIR apunta a otro lado (por ejemplo
// el scratch de un proceso), el daemon escribiría el PID ahí y el
// comando que lo busca no lo encontraría.
func pidFilePath() string {
	return "/tmp/boot-preview-plymouthd.pid"
}

// findPlymouthd busca el PID del daemon de Plymouth.
//
// El daemon corre como root dentro de un namespace de mounts, asi que
// no se puede finds por el proceso padre.
func findPlymouthd() int {
	// El daemon corre como root: el pgrep necesita ser root también, si
	// no no lo ve.
	out, err := cli.RunCombined("sudo", "pgrep", "-f", "plymouthd --no-daemon")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(out, "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			return pid
		}
	}
	return 0
}

// StopPlymouth cierra el daemon de Plymouth.
//
// Lo usan 'boot-preview animation stop' y el wrapper de plymouth.
func StopPlymouth() error {
	// Si hay un PID anotado, se usa.
	if data, err := os.ReadFile(pidFilePath()); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 {
			// El grupo entero: el plymouthd y su wrapper de sudo.
			_ = syscall.Kill(-pid, syscall.SIGTERM)
			// Y el plymouthd solo, por si el grupo no existe.
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
		_ = os.Remove(pidFilePath())
	}

	// Y por las dudas, el comando propio de Plymouth.
	_, _ = cli.RunCombined("sudo", "--preserve-env=DISPLAY,XAUTHORITY", "plymouth", "--quit")
	return nil
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

	// El archivo del PID: el script lo escribe al arrancar.
	pidFile := pidFilePath()
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o755); err != nil {
		return nil, err
	}

	// El daemon, dentro del namespace de mounts.
	//
	// El script anota su propio PID antes del exec: con exec, el PID se
	// mantiene, así que el del plymouthd final es el mismo. Así el daemon
	// se puede matar después sin depender de pgrep.
	script := fmt.Sprintf(
		"mount --bind '%s' /etc/plymouth && echo $$ > '%s' && exec %s plymouthd --no-daemon --debug --debug-file='%s' --mode=boot",
		etcDir, pidFile, envPrefix(), x11Log)

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

	// NO se arranca acá: el SysProcAttr (setsid) tiene que estar puesto
	// ANTES del Start, y según el modo se pone o no. Lo hace start().
	proc := &plymouthProcess{cmd: cmd, etcDir: etcDir, runTheme: runTheme, devNull: devNull, pidFile: pidFile}
	return proc, nil
}

// start arranca el daemon y espera a que esté listo.
//
// Si detach es true, el proceso se pone en su propia sesión: no recibe
// el SIGHUP de la terminal y sigue vivo cuando el comando termina.
func (p *plymouthProcess) start(detach bool) error {
	if detach {
		p.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("no se pudo arrancar plymouthd: %w", err)
	}
	// NO se cierra el descriptor: el daemon lo tiene abierto y Plymouth
	// se apaga en cuanto su stdout se cierra.

	sleep(3 * time.Second)

	if !p.alive() {
		p.cleanup()
		cli.Warn("plymouthd terminó. Log:")
		printLogTail(x11Log)
		return fmt.Errorf("")
	}

	// Mostrar el splash.
	//
	// NO se usa RunCombined: ese captura la salida por un pipe, y Plymouth
	// se comporta distinto si su stdout no es una tty (deja de dibujar).
	// Con la salida a /dev/null se comporta como con el shell.
	showSplash()
	return nil
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
	// sesión Wayland (donde no la ve el 'import' de X) y el preview
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
