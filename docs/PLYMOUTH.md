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

```
plymouthd                 el daemon (dibuja en el DRM)
plymouth --show-splash    le dice "mostrá el splash"
plymouth --update=TEXTO   cambia el mensaje
plymouth --reload         recarga el tema (parcial)
plymouth --quit           lo cierra
```

`plymouth-set-default-theme` solo cambia el tema; `plymouth-kcm` (el de KDE)
es un selector, no un editor. Los PNG se leen **al arrancar** el daemon.

---

## Cómo se previsualiza

### `plymouth-preview`

```bash
plymouth-preview access               # una vez por sesión (ver abajo)
plymouth-preview                      # ventana con el splash
plymouth-preview x11 [segundos]       # idem, con límite de tiempo
plymouth-preview shot [archivo]       # captura el splash
plymouth-preview sources              # temas de Plymouth instalados
plymouth-preview preview <tema>       # tema SOLO del preview
plymouth-preview use <tema>           # tema del SISTEMA (-R)
plymouth-preview steam-source         # videos de Steam
plymouth-preview steam-convert <vid>  # convierte un video en tema
plymouth-preview steam-apply <vid>    # convierte y aplica al sistema
```

### Qué hay por debajo

**El Plymouth oficial, con el renderer X11.** No es un reproductor de
imágenes: es el mismo daemon, los mismos plugins y el mismo tema que usa la
máquina al arrancar.

```
plymouth-preview
  └─ plymouthd (root, del sistema)
       └─ renderers/x11.so              ← dibuja en una ventana GTK
            └─ plugins/splash/script.so  ← el tema (script o two-step)
                 └─ /run/plymouth/themes/<tema>/
```

**No hay VM, no hay initramfs, no hay QEMU.**

Plymouth prueba los renderers en este orden (`ply-renderer.c`):

```c
{ PLY_RENDERER_TYPE_X11,          "renderers/x11.so"          },
{ PLY_RENDERER_TYPE_DRM,          "renderers/drm.so"          },
{ PLY_RENDERER_TYPE_SIMPLEDRM,    "renderers/drm.so"          },
{ PLY_RENDERER_TYPE_FRAME_BUFFER, "renderers/frame-buffer.so" },
```

Con `PLY_RENDERER_TYPE_AUTO` (el default) prueba **X11 primero**. Si no hay
`DISPLAY`, cae a DRM: dibuja en la pantalla real, pero el compositor ya la
tiene tomada y no se ve nada útil.

El source del plugin (`plugins/renderers/x11/plugin.c`) usa GTK3:

```c
gdk_set_allowed_backends ("x11");
if (!gtk_init_check (0, NULL))
    return NULL;
```

### Requisito: root necesita acceso al servidor X

`plymouthd` **exige root** (`plymouthd must be run as root user`), y root
solo puede abrir ventanas si el servidor X le da acceso. En Wayland el
Xwayland normalmente tiene control de acceso y responde:

```
Authorization required, but no authorization protocol specified
```

El renderer `x11.so` no se puede crear (`renderer backend could not be
created`) y Plymouth cae a DRM → **pantalla negra, sin error visible**.

**La solución, una vez por sesión:**

```bash
plymouth-preview access
```

Que es:

```bash
xhost +SI:localuser:root
```

`xorg-xhost` está en el repo `extra`. El permiso dura hasta que cierres la
sesión gráfica: si abrís otra sesión, hay que repetirlo.

### Control en caliente desde otra terminal

Con la ventana abierta:

```bash
sudo plymouth --reload        # recarga el tema (tras editar un PNG)
sudo plymouth --show-splash   # vuelve a mostrar / relanza la animación
sudo plymouth --update=hola   # cambia el mensaje
sudo plymouth --quit          # cierra
```

Es el ciclo de desarrollo real: editás el PNG, `--reload`, y lo ves sin
reiniciar nada.

### Qué toca el comando (y qué NO)

| Archivo | Qué pasa |
|---|---|
| `/run/plymouth/themes/<tema>/` | copia del tema (Plymouth lo mira **antes** que `/usr/share`) |
| `/run/plymouth/themes/<tema>/<tema>.plymouth` | se reescribe `ImageDir` y `ScriptFile` para que apunten a `/run` |
| `/etc/plymouth/plymouthd.conf` | **no se toca**: se monta una config propia sobre `/etc/plymouth` en un namespace de mounts privado (`unshare -m`) |

El sistema queda intacto. Al salir, `/run/plymouth/themes/<tema>` se borra.

> **Por qué hace falta el namespace**: el `plymouthd.conf` del sistema dice
> `Theme=cachyos-bootanimation` y **gana** sobre `/run`. Montar la config
> por encima en un namespace privado es la única forma de usar otro tema sin
> tocar `/etc`.

### El plugin `script`, no `two-step`

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
105 frames a 1280×720 la diferencia de memoria es enorme.

> El intérprete del plugin `script` **no tiene `sleep`** ni bucles con
> espera: el avance de frame se hace en el callback de refresh, no en un
> `while`.

### Lo que NO funcionó (documentado para no repetirlo)

| Intento | Por qué falla |
|---|---|
| `ShowDelay=0` | Plymouth aborta: `Assertion 'seconds > 0.0' failed` |
| Dejar claves sin definir en el conf | quedan en **NaN** y Plymouth se rompe |
| Plymouth sin udev | los devices nunca se marcan "initialized" y cae al renderer de texto |
| `sudo plymouthd` sin `DISPLAY` | cae al renderer DRM y no se ve nada |
| `sudo` que no preserva `DISPLAY` | el renderer X11 no puede abrir la ventana |
| `env "A,B,C"` | `env` NO acepta lista: hay que pasar cada variable aparte |
| `xhost` sin permiso | `Authorization required` y el renderer no se crea |

### La vía anterior: VM de QEMU

Antes el preview usaba una **VM de QEMU con un initramfs hecho a mano**
(mkinitcpio + hook propio). Funcionaba, pero era lenta (~30 s), el splash se
veía de forma inconsistente por la race con la inicialización del DRM, y el
initramfs **no tenía rootfs**: cuando el init no podía montar el root
lanzaba la shell de emergencia y el splash desaparecía.

El renderer X11 hace lo mismo con el daemon real, en tu escritorio, en
segundos, y se puede capturar frame a frame.

---

## Usar la animación de Steam como tema

### Cuál es el video activo

```bash
plymouth-preview steam-source
```

```
▸ Animaciones de inicio de Steam
  Compradas en la tienda:
      Ghost Machine                    (25578860771)
    * Elden Ring Ranni                 (31391441102)  ← la activa

  Del cliente (Steam Big Picture / Deck):
      bigpicture_startup.webm
      deck_startup.webm
      oled_startup.webm
      ...
```

Los que trae el cliente son **videos**: VP9, 1920×1200, 60 fps, ~6 s, con
audio Opus. Plymouth no reproduce video, así que hay que **extraer los
frames** y armar un tema.

**Cómo se decide cuál está activo:**

Steam guarda la selección en `~/.steam/steam/config/config.vdf`:

```
"StartupMovie"
{
    "MovieID"     "31391441102"
    "LocalPath"   "/communityitemscache/startupmovies/31391441102_88576....webm"
}
```

> Ojo: `LocalPath` **no** incluye el segmento `config/`. La ruta real es
> `<steam>/config/communityitemscache/...`.

Si no hay selección, se usa la del cliente según la máquina:

| Máquina | Video |
|---|---|
| Desktop (Big Picture) | `bigpicture_startup.webm` |
| Steam Deck LCD (`Jupiter`) | `deck_startup.webm` |
| Steam Deck OLED (`Galileo`) | `oled_startup.webm` |

Se detecta con `/sys/devices/virtual/dmi/id/product_name`.

### Las compradas en la tienda de Steam

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

### Convertirlo en tema

```bash
plymouth-preview steam-convert                      # el que está activo
plymouth-preview steam-apply                        # el que está activo + lo aplica
plymouth-preview steam-convert ~/otro.webm          # un video puntual
plymouth-preview steam-apply deck_startup.webm      # nombre relativo a movies/
```

Ajustes por variable de entorno: `STEAM_FPS` (default 30) y `STEAM_RES`
(default `1920x1080`).

Qué hace:

1. `ffmpeg -vf fps=30,crop...,scale=1920x1080:flags=lanczos` → frames PNG.
   Se descarta el audio.
2. Copia los `throbber-*.png` del tema base para el spinner de progreso.
3. Genera el `script.script` con el plugin `script` (loop libre).
4. Genera el `.plymouth` con `ModuleName=script`.
5. Instala en `/usr/share/plymouth/themes/<nombre>`.

El nombre sale del **nombre real** del item: `steam-elden-ring-ranni`.

**Tamaño según los ajustes:**

| fps | res | frames | tamaño |
|---|---|---|---|
| 30 | 1920×1080 | 210 | **133 MB** |
| 15 | 1280×720 | 105 | **45 MB** |

Para trabajar cómodo:

```bash
STEAM_FPS=15 STEAM_RES=1280x720 plymouth-preview steam-convert
```

### Aplicar un tema al sistema

```bash
plymouth-preview use <nombre>    # plymouth-set-default-theme -R
```

**Ojo**: `-R` regenera el initramfs real (~30 s). Es lo que hace que el tema
se vea en el próximo arranque; sin `-R` no se aplica.

Para volver al de CachyOS: `plymouth-preview use cachyos-bootanimation`.

### Verlo antes de aplicarlo

```bash
plymouth-preview preview steam-elden-ring-ranni   # solo el preview
plymouth-preview                                  # mirarlo
plymouth-preview shot                             # captura
```

`preview` solo escribe `~/.config/plymouth-preview/theme`. El sistema no se
toca.

> **Nota**: `plymouth.theme=` y `plymouth.splash=` en el cmdline **no**
> cambian el tema (probado).

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
├── logo.png                  # el logo (watermark)
├── background.png            # el fondo
├── progress-*.png            # animación de progreso
├── throbber-*.png            # el spinner
└── entry.png                 # el texto
```

### Un tema `script` es

```
mi-tema/
├── mi-tema.plymouth          # ModuleName=script
├── script.script             # el script
└── animation-*.png           # los frames
```

---

## Herramientas de desarrollo

No hay IDE ni GUI oficial. Lo que existe:

| Herramienta | Qué hace |
|---|---|
| **renderer `x11.so`** | Plymouth ya lo trae (`/usr/lib/plymouth/renderers/`). Con `DISPLAY` seteado dibuja en una ventana X11: **sin VM** |
| `plymouth-x11` (Debian/Fedora) | el binario que usa ese renderer. **No está empaquetado en Arch**, pero el renderer sí |
| `plymouthd --debug --debug-file=...` | log de todo lo que hace el daemon |
| `plymouth --reload` | recarga el tema sin reiniciar el daemon (parcial) |
| `showplymouth.sh` (repo adi1090x) | script de 20 líneas: `plymouthd; --show-splash; sleep N; --quit` |
| **este repo** | `plymouth-preview` con `access`, `shot`, `preview`, `use` |

### Paquetes que hacen falta

| Paquete | Para qué |
|---|---|
| `plymouth` | El daemon y los plugins (ya instalado) |
| `xorg-xhost` | `xhost +SI:localuser:root` — el acceso de root al X |
| `imagemagick` | `import` para capturar la ventana |
| `xdotool` | Encontrar la ventana de Plymouth |
| `ffmpeg` | Convertir los videos de Steam en frames |

> `xorg-server-xvfb` sirve para un display virtual headless (CI, o si no
> querés abrir ninguna ventana). **No es necesario** para el uso normal:
> el Xwayland de tu sesión Wayland ya es un servidor X.

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
