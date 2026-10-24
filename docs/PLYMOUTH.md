# Plymouth: el splash de arranque

Lo que se ve **después** de elegir CachyOS en el menú de Limine.

> **Vicinae no es esto.** Vicinae es un launcher de escritorio (command
> palette tipo Raycast). La animación de arranque es Plymouth.

---

## Qué es

Plymouth es el splash gráfico de Linux: dibuja en el framebuffer/DRM
mientras el sistema arranca, y lo va actualizando con mensajes de progreso.

### Lo que tenés instalado

| | |
|---|---|
| **Tema activo** | `cachyos-bootanimation` (`/etc/plymouth/plymouthd.conf`) |
| **Plugin** | `two-step` — logo arriba, throbber abajo |
| **Frames** | **88** de animación + **41** del throbber |
| **Formato** | PNG (**no** es un video) |
| **Hooks en el initramfs** | `HOOKS=(... plymouth ...)` |

### No hay editor ni hot reload

Plymouth **no** tiene una GUI de edición ni recarga en vivo:

```
plymouthd                 el daemon (dibuja en el DRM)
plymouth --show-splash    le dice "mostrá el splash"
plymouth --update=TEXTO   cambia el mensaje
plymouth --reload         recarga el tema (parcial)
plymouth --quit           lo cierra
```

`plymouth-set-default-theme` solo cambia el tema; `plymouth-kcm` (el de KDE)
es un selector, no un editor. Los PNG se leen **al arrancar** el daemon: para
ver un cambio hay que reiniciarlo.

---

## Cómo se previsualiza

### `plymouth-preview`

```bash
plymouth-preview                     # ventana con el splash
plymouth-preview shot [archivo]      # captura el splash
plymouth-preview sources             # temas de Plymouth instalados
plymouth-preview preview <tema>      # tema SOLO del preview
plymouth-preview use <tema>          # tema del SISTEMA (-R)
plymouth-preview steam-source        # videos de Steam
plymouth-preview steam-convert <vid> # convierte un video en tema
plymouth-preview steam-apply <vid>   # convierte y aplica al sistema
plymouth-preview build               # regenera el initramfs
```

### Qué hay por debajo

**El Plymouth oficial dentro de una VM.** No es un reproductor de imágenes:

| Pieza | Qué es |
|---|---|
| `plymouthd` | el binario del sistema (`/usr/sbin/plymouthd`) |
| Plugins | los mismos de `/usr/lib/plymouth/` (`two-step`, `script`, ...) |
| Renderer | `renderers/drm.so`, dibujando sobre el DRM de la VM |
| VM | QEMU + KVM, `-kernel` + `-initrd` (sin ISO) |
| Initramfs | generado con `mkinitcpio`, el mismo que usa la máquina |

O sea: es **el mismo software y el mismo render** que ves al arrancar; la VM
solo reemplaza al hardware.

### Cómo funciona

1. Genera un initramfs con **mkinitcpio**:
   ```
   MODULES=(bochs)      # driver de la VGA de QEMU (-vga std)
   HOOKS=(base udev plymouth modconf block filesystems)
   ```
   El hook `plymouth` de mkinitcpio monta `/dev/pts` y lanza
   `plymouthd --mode=boot --attach-to-session`.
2. Arranca QEMU con `-kernel` + `-initrd` (sin ISO).
3. `shot` muestrea el framebuffer cada 400 ms durante 24 s y se queda con el
   frame de más contenido (un frame negro pesa ~1.5 KB; el splash, 20-25 KB).

El initramfs se cachea en `~/.cache/plymouth-preview/` (~44 MB).

### Por qué no se queda en pantalla

El initramfs **no tiene rootfs**. Cuando el init de mkinitcpio no puede montar
el root, lanza la shell de emergencia y el splash desaparece.

Hay una race con la inicialización del DRM de QEMU: el splash puede verse
desde ~1 s hasta ~15 s, de forma inconsistente. Por eso `shot` muestrea en
vez de capturar en un instante fijo.

Para que se quede de forma estable haría falta un **rootfs real** (kernel +
root + init), que es lo que hace una ISO de verdad. Es el siguiente paso si
se quiere un preview interactivo.

### Lo que NO funcionó (documentado para no repetirlo)

| Intento | Por qué falla |
|---|---|
| Initramfs a mano con busybox | Plymouth necesita udev (`device is not initialized`) y los renderers en `renderers/` |
| Hook en `run_hook` que bloquea | El splash no se dibuja (el `--attach-to-session` necesita el flujo normal) |
| Hook en `run_latehook` | El init llama a `mount_handler` **antes** de los latehooks y sale sin ejecutarlos |
| Hook en `run_emergencyhook` | Bloquea la shell, pero el splash ya se perdió |
| `ShowDelay=0` | `Assertion 'seconds > 0.0' failed` en el event loop |
| `-cpu qemu64` (default) | `Illegal instruction`: las libs de Arch piden AVX2. Hace falta `-cpu host` |

---

## Usar la animación de Steam como tema

Steam trae la animación de entrada a Big Picture en
`~/.local/share/Steam/steamui/movies/bigpicture_startup.webm`.

**Es un video, no una imagen**: VP9, 1920x1200, 60 fps, 6 s, con audio Opus.
Plymouth no reproduce video, así que hay que **extraer los frames** y armar
un tema `two-step` con ellos.

### Cuál es el video activo

```bash
plymouth-preview steam-source
```

Muestra todos los videos de startup que tiene Steam y marca el activo:

```
* bigpicture_startup.webm  (1920,1200,60/1 6.000000) ← el activo
  deck_startup.webm        (450,450,60/1 4.008000)
  oled_startup.webm        (450,450,60/1 4.000000)
  ...
```

**Cómo se decide cuál es el activo:**

| Máquina | Video |
|---|---|
| Desktop (Big Picture) | `bigpicture_startup.webm` |
| Steam Deck LCD (`Jupiter`) | `deck_startup.webm` |
| Steam Deck OLED (`Galileo`) | `oled_startup.webm` |

Se detecta con `/sys/devices/virtual/dmi/id/product_name`. En desktop **no
hay forma de elegirlo desde Steam**: es el archivo que trae el cliente, no
se puede cambiar por configuración. `bigpicture_startup.webm` en este
sistema es de **noviembre 2022** (el empaquetado original).

### Convertirlo en tema

```bash
plymouth-preview steam-convert                      # el video activo
plymouth-preview steam-apply                        # el video activo + lo aplica
plymouth-preview steam-convert ~/otro.webm          # un video puntual
plymouth-preview steam-apply deck_startup.webm      # nombre relativo a movies/
```

Ajustes por variable de entorno: `STEAM_FPS` (default 30) y `STEAM_RES`
(default `1920x1080`).

Qué hace:

1. `ffmpeg -vf fps=30,crop...,scale=1920x1080:flags=lanczos` → 180 frames PNG
   (6 s × 30 fps). Se descarta el audio.
2. Copia los `throbber-*.png` del tema base para el spinner de progreso.
3. Genera el `.plymouth` con `ModuleName=two-step`.
4. Instala en `/usr/share/plymouth/themes/<nombre>` (~18 MB).

El nombre por defecto es `steam-<nombre-del-video>`, p. ej.
`steam-bigpicture-startup`.

### Aplicar un tema al sistema

```bash
plymouth-preview use <nombre>    # plymouth-set-default-theme -R
```

**Ojo**: `-R` regenera el initramfs real (~30 s). Es lo que hace que el tema
se vea en el próximo arranque; sin `-R` no se aplica.

Para volver al de CachyOS: `plymouth-preview use cachyos-bootanimation`.

### Limitación del muestreo

`shot` se queda con el frame de más contenido, que suele ser el **final** de
la animación (el último frame tiene más píxeles no-negros). Como el arranque
del initramfs es lento e inconsistente, el muestreo a veces solo alcanza a
ver el final.

Para ver la animación completa: `plymouth-preview` abre la ventana de QEMU y
la animación se reproduce una vez (6 s). El contact sheet (`<captura>-sheet.png`)
muestra los 4 frames más pesados, útiles cuando el muestreo sí agarra el
principio.

### Verlo antes de aplicarlo

```bash
plymouth-preview theme steam-bigpicture-startup   # solo el preview
plymouth-preview shot                             # captura
```

`theme` cambia el tema **del preview**, no del sistema. El preview inyecta el
nombre en el initramfs (`/preview-theme`) y un hook lo escribe en
`/etc/plymouth/plymouthd.conf` antes de que arranque el daemon.

> **Nota**: `plymouth.theme=` y `plymouth.splash=` en el cmdline **no**
> cambian el tema (probado). La única forma es el conf dentro del initramfs.

---

## Qué se puede personalizar

### Plugins disponibles

| Plugin | Qué hace |
|---|---|
| `two-step` | logo + spinner (el actual) |
| `script` | **el potente**: animaciones con sprites, funciones, transiciones |
| `fade-throbber` | fade entre frames |
| `space-flares` | efecto espacial |
| `tribar` | barra de progreso de 3 colores |
| `details` | texto de arranque tipo consola |
| `text` | solo texto |
| `label-freetype` / `label-pango` | tipografía de los labels |

**No hay video, ni MP4, ni shaders.** Es PNG + scripting.

### Un tema `two-step` es

```
mi-tema/
├── mi-tema.plymouth          # la config (ModuleName=two-step)
├── animation-00.png ...       # los frames del logo
└── throbber-00.png ...        # los frames del spinner
```

### Aplicar un tema al sistema

```bash
sudo plymouth-set-default-theme -R <nombre>
```

El `-R` regenera el initramfs. **Sin `-R` no se aplica** en el próximo boot.

---

## Las animaciones compradas en la tienda de Steam

Las que se compran con puntos de Steam **no** están en `steamui/movies/`.
Están en:

```
~/.local/share/Steam/config/communityitemscache/startupmovies/
    31391441102_88576f998883aafe4ad3821ac8c0cfa4f11d7c84.webm   ← Elden Ring Ranni
    25578860771_4dfeecc35e628ed9160fb19b9aa9a04bd291f2db.webm   ← Ghost Machine
```

El nombre del archivo es `<item_id>_<hash>.webm`. **El nombre real está en
otro lado**:

```bash
~/.steam/steam/userdata/<steamid>/config/localconfig.vdf
```

con la clave `"GetStartupMovies"`, que es un JSON con `name`, `appid`,
`movie_webm_local_path`, etc.

**Cuál está activa** se guarda en:

```bash
~/.steam/steam/config/config.vdf
```

```
"StartupMovie"
{
    "MovieID"     "31391441102"
    "LocalPath"   "/communityitemscache/startupmovies/31391441102_88576....webm"
}
```

> Ojo: `LocalPath` **no** incluye el segmento `config/`. La ruta real es
> `<steam>/config/communityitemscache/...`.

`plymouth-preview steam-source` hace todo ese parseo y muestra:

```
▸ Animaciones de inicio de Steam
  Compradas en la tienda:
    Ghost Machine                    (25578860771)
  * Elden Ring Ranni                 (31391441102)  ← la activa

  Del cliente (Steam Big Picture / Deck):
    bigpicture_startup.webm
    deck_startup.webm
    ...
```

## Comandos dentro de la VM

```bash
plymouth-preview shell
```

Abre la consola serie de la VM en la terminal (busybox). Ahí:

| Comando | Qué hace |
|---|---|
| `plymouth --show-splash` | relanzar la animación |
| `plymouth --update=hola` | cambiar el mensaje |
| `plymouth --reload` | recargar el tema |
| `plymouth --quit` | cerrar el splash |
| `ls /run/plymouth` | estado del daemon |
| `reboot -f` | apagar la VM |

Salir de QEMU: **Ctrl+A**, después **X**.

## Herramientas de desarrollo

No hay IDE ni GUI oficial. Lo que existe:

| Herramienta | Qué hace |
|---|---|
| **renderer `x11.so`** | Plymouth ya lo trae (`/usr/lib/plymouth/renderers/`). Con `DISPLAY` seteado dibuja en una ventana X11: **sin VM** |
| `plymouth-x11` (Debian/Fedora) | el binario que usa ese renderer. **No está empaquetado en Arch**, pero el renderer sí |
| `plymouthd --debug --debug-file=...` | log de todo lo que hace el daemon |
| `plymouth --reload` | recarga el tema sin reiniciar el daemon (parcial) |
| `showplymouth.sh` (repo adi1090x) | script de 20 líneas: `plymouthd; --show-splash; sleep N; --quit` |
| **este repo** | `plymouth-preview` + `shell` + `x11` |

### Modo X11 (sin VM) — la forma cómoda de desarrollar

```bash
plymouth-preview x11 [segundos]
```

**Abre una ventana GTK fullscreen con el splash, sobre tu escritorio.** Sin
VM, sin initramfs, sin QEMU. Es el plugin `x11.so` que ya trae Plymouth
(Arch lo compila en el paquete `plymouth`: no hay nada que instalar ni
compilar).

Plymouth elige renderer en este orden (`ply-renderer.c`):

```c
{ PLY_RENDERER_TYPE_X11,          "renderers/x11.so"          },
{ PLY_RENDERER_TYPE_DRM,          "renderers/drm.so"          },
{ PLY_RENDERER_TYPE_SIMPLEDRM,    "renderers/drm.so"          },
{ PLY_RENDERER_TYPE_FRAME_BUFFER, "renderers/frame-buffer.so" },
```

Con `PLY_RENDERER_TYPE_AUTO` (el default) prueba **X11 primero**. El source
del plugin (`plugins/renderers/x11/plugin.c`) usa GTK3:

```c
gdk_set_allowed_backends ("x11");
if (!gtk_init_check (0, NULL))
    return NULL;
```

#### Requisito: root necesita acceso al servidor X

`plymouthd` **exige root** (`plymouthd must be run as root user`), y root
solo puede abrir una ventana si el servidor X le da acceso. En Wayland el
Xwayland normalmente tiene control de acceso:

```
Authorization required, but no authorization protocol specified
```

El renderer `x11.so` no se puede crear (`renderer backend could not be
created`) y Plymouth cae a DRM, que el compositor ya tiene tomado → pantalla
en negro, sin error visible.

**Solución (una vez por sesión):**

```bash
sudo xhost +SI:localuser:root
```

`xorg-xhost` está en el repo `extra`. Sin eso el comando avisa y sale.

#### Control en caliente desde otra terminal

Con la ventana abierta:

```bash
sudo plymouth --reload        # recarga el tema (tras editar un PNG)
sudo plymouth --show-splash   # vuelve a mostrar / relanza la animación
sudo plymouth --update=hola   # cambia el mensaje
sudo plymouth --quit          # cierra
```

Es el ciclo de desarrollo real: editás el PNG, `--reload`, y lo ves sin
reiniciar nada.

#### Qué toca el comando (y qué NO)

| Archivo | Qué pasa |
|---|---|
| `/run/plymouth/themes/<tema>/` | copia del tema (Plymouth lo mira antes que `/usr/share`) |
| `/run/plymouth/themes/<tema>/<tema>.plymouth` | se reescribe `ImageDir` y `ScriptFile` para que apunten a `/run` |
| `/etc/plymouth/plymouthd.conf` | **no se toca**: se monta una config propia sobre `/etc/plymouth` en un namespace de mounts privado (`unshare -m`) |

El sistema queda intacto. Al salir, `/run/plymouth/themes/<tema>` se borra.

#### El plugin `script`, no `two-step`

Los temas generados desde un video de Steam usan el plugin **`script`**, no
`two-step`:

- `two-step` ata la animación al **final del boot** (`UseEndAnimation`): sin
  un boot real nunca arranca. Solo se ve el throbber.
- `script` es un **loop libre**. Plymouth llama a la función de refresh cada
  `1/refresh_rate` segundos y ahí se avanza un frame:

```javascript
frames = [ "animation-000.png", "animation-001.png", ... ];

sprite = Sprite(Image("animation-000.png"));
sprite.SetZ(0);
sprite.SetOpacity(1);

frame = 0;

fun Refresh ()
{
  frame = (frame + 1) % 105;
  sprite.SetImage(Image(frames[frame]));
}

Plymouth.SetRefreshFunction(Refresh);
```

**Un solo sprite** al que se le cambia la imagen, no un sprite por frame: con
105 frames a 1280x720 la diferencia de memoria es enorme.

> El intérprete del plugin `script` **no tiene `sleep`** ni bucles con
> espera: el avance de frame se hace en el callback de refresh, no en un
> `while`.

## Por qué la VM tarda

| Causa | Peso |
|---|---|
| initramfs con el tema completo | **45-133 MB** (105-210 PNG) |
| `mkinitcpio` al regenerar | ~6 s |
| Arranque de QEMU + kernel | ~8 s |
| Muestreo del framebuffer | 24 s (máx) |

**Cómo acelerarlo**:

```bash
STEAM_FPS=15 STEAM_RES=1280x720 plymouth-preview steam-convert
```

| fps | res | frames | tamaño |
|---|---|---|---|
| 30 | 1920x1080 | 210 | **133 MB** |
| 15 | 1280x720 | 105 | **45 MB** |

El tema se cachea en `~/.cache/plymouth-preview/<tema>-<kernel>-<hash>.img`,
así que solo se regenera la primera vez o cuando cambia el tema.

---

## Dónde conseguir temas

| Fuente | Qué hay |
|---|---|
| **gnome-look.org** (`cat=108`) | cientos de temas de la comunidad |
| **github.com/adi1090x/plymouth-themes** | 80+ portados de bootanimations de Android |
| **AUR** | `holo-plymouth-themes`, `breeze-plymouth` |
| **KDE System Settings** | `plymouth-kcm` ya instalado |

### Temas ya instalados en este sistema

```
bgrt                  cachyos            cachyos-bootanimation
details               fade-in            glow
script                solar              spinfinity
spinner               text               tribar
```
