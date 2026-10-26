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

// daemonRunning dice si ya hay un daemon de Plymouth corriendo.
//
// Lo usa `animation shot`: si el daemon ya está (porque el usuario dejó
// la ventana abierta), el shot reutiliza esa ventana en vez de levantar
// un daemon nuevo. Si no, levanta el suyo.
func daemonRunning() bool {
	out, err := cli.RunCombined("sudo", "--preserve-env=DISPLAY,XAUTHORITY", "plymouth", "--ping")
	return err == nil && strings.TrimSpace(out) != ""
}

// StopPlymouth cierra el daemon de Plymouth y limpia lo que dejó.
//
// Lo usan 'boot-preview animation stop' y el wrapper de plymouth.
//
// El daemon corre dentro de un namespace de mounts, con un wrapper de
// sudo delante. Matar al plymouthd alcanza, pero el wrapper se queda
// esperando: se mata el grupo de procesos entero.
//
// También borra el tema de /run y la config temporal: con el daemon
// desacoplado, el comando que lo arrancó ya retornó y no limpió nada.
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

	// La limpieza de lo que dejó el daemon: el tema copiado a /run y la
	// config temporal montada sobre /etc/plymouth.
	cleanupPreviewFiles()
	return nil
}

// cleanupPreviewFiles borra el tema de /run y las configs temporales.
//
// El daemon desacoplado deja estas cosas: su comando ya retornó, asi que
// no hay nadie que las limpie.
func cleanupPreviewFiles() {
	// Las configs temporales: el nombre empieza con el prefijo que usa
	// el preview.
	//
	// Es /tmp y no os.TempDir(): el daemon las crea en /tmp (el script
	// corre como root y TMPDIR no lo tiene), así que ahí hay que
	// buscarlas.
	matches, _ := filepath.Glob("/tmp/boot-preview-animation-etc.*")
	for _, dir := range matches {
		_ = os.RemoveAll(dir)
	}

	// El tema en /run: solo el que copió el preview. Los que ya estaban
	// (los del sistema) no se tocan.
	entries, err := os.ReadDir("/run/plymouth/themes")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// El preview copia el tema tal cual: si su .plymouth dice que
		// fue generado desde un video, es del preview.
		ply := filepath.Join("/run/plymouth/themes", entry.Name(), entry.Name()+".plymouth")
		data, err := os.ReadFile(ply)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "Generado desde") {
			_, _ = cli.RunCombined("sudo", "rm", "-rf", filepath.Join("/run/plymouth/themes", entry.Name()))
		}
	}
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
func startPlymouthX11(theme string, paths cli.ToolPaths) (*plymouthProcess, error) {
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
	// En /tmp y no en os.TempDir(): el daemon la monta como root, y
	// TMPDIR puede no estar en su entorno.
	etcDir, err := os.MkdirTemp("/tmp", "boot-preview-animation-etc.*")
	if err != nil {
		return nil, err
	}
	conf := fmt.Sprintf("[Daemon]\nTheme=%s\nShowDelay=0.5\nDeviceTimeout=10\n", theme)
	if err := os.WriteFile(filepath.Join(etcDir, "plymouthd.conf"), []byte(conf), 0o644); err != nil {
		return nil, err
	}

	_, _ = cli.RunCombined("sudo", "plymouth", "--quit")
	time.Sleep(time.Second)

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

	time.Sleep(3 * time.Second)

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
