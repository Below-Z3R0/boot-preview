# boot-preview

Ver el **arranque de Linux** sin reiniciar la máquina.

Son dos pantallas distintas, en dos momentos distintos:

| | Qué es | Cuándo |
|---|---|---|
| **`loader`** | El **menú de Limine**, el bootloader | Elegís qué booteás |
| **`animation`** | El **splash de Plymouth**, la animación | Arranca el kernel |

```bash
boot-preview loader      # el menú
boot-preview animation   # la animación (el daemon queda en background)
boot-preview animation    # controlar el daemon de Plymouth
```

## El menú de Limine (`loader`)

Limine no tiene previewer: no hay nada en el repo upstream, ni herramienta
online, ni preview en la comunidad. La única forma de ver el menú sin
reiniciar es **emularlo**. Este repo lo hace:

```
tu config de Limine  →  ISO híbrido  →  QEMU con KVM  →  el menú real
```

El render es **exacto**: es el mismo bootloader, el mismo wallpaper y la misma
paleta. Tu `/boot/limine.conf` **no se toca**.

## El splash de Plymouth (`animation`)

Plymouth tampoco tiene previewer. Pero su daemon (`plymouthd`) trae un
renderer **X11** que dibuja en una ventana normal:

```
el plymouthd del sistema  →  renderers/x11.so  →  una ventana GTK
```

Sin VM, sin initramfs, en segundos. Es el mismo daemon, los mismos plugins y
el mismo tema que usa la máquina al arrancar.

```
boot-preview/
├── cmd/boot-preview/         # el punto de entrada: un solo binario
├── internal/
│   ├── cli/                  # helpers compartidos (rutas, archivos, comandos)
│   ├── loader/               # el módulo de Limine (el menú)
│   ├── animation/            # el módulo de Plymouth (el splash)
│   ├── wallpaper/            # blur + velo + resolución, para Limine
│   ├── steam/                # el parser de las animaciones de Steam
│   ├── install/              # el instalador
│   ├── verify/               # la verificación post-instalación
│   └── pack/                 # el paquete descargable y el hook de pacman
├── scripts/
│   └── smoke-test.sh         # la batería de pruebas (sin root, para CI)
├── examples/
│   ├── loader.conf           # la config de prueba
│   ├── theming-rosepine.conf # el bloque de theming para aplicar al sistema
│   └── wallpaper/
│       ├── rei.png           # el wallpaper de ejemplo
│       ├── rosepine.png      # un gradiente Rosé Pine alternativo
│       └── rosepine.qoi      # el mismo, en QOI (Limine lo soporta nativamente)
├── docs/
│   ├── FLUJO.md              # el flujo completo, de la config a la pantalla
│   ├── THEMING.md            # las 52 opciones y qué se puede personalizar
│   ├── APLICAR.md            # cómo aplicar el tema al Limine real
│   ├── RENDIMIENTO.md        # las mediciones y la comparativa Go vs bash
│   └── PLYMOUTH.md           # el splash de arranque (que sí es Plymouth)
├── previews/                 # las capturas
├── go.mod
└── LICENSE
```

## Las tres configs de Limine

Limine tiene **más de un** fichero de configuración, y el preview las usa de
forma distinta. Esta tabla es la que hay que tener clara:

| # | Path | Qué es | Quién la escribe | ¿La carga el preview? |
|---|---|---|---|---|
| **1** | `~/.config/boot-preview/loader/limine.conf` | la config **de prueba** — la que edita el usuario para iterar el tema | `install` la instala desde `examples/loader.conf`; el preview la inyecta con el wallpaper | ✅ **por default** |
| **2** | `/boot/limine.conf` | la config **compilada** que Limine lee de verdad en el boot (99 KB, autogenerada) | `limine-mkinitcpio` la genera desde la #3 | ✅ solo con `--real` (se lee con sudo) |
| **3** | `/etc/default/limine` | la **fuente** — cmdline del kernel, boot order, y el theming que agregues | **la editás vos** | ❌ nunca se toca |

**El flujo del sistema real**:

```
/etc/default/limine          ← la fuente: la editás vos
        │  limine-mkinitcpio
        ▼
/boot/limine.conf            ← la compilada: la regenera el hook de initramfs
        │  el firmware la busca en la partición FAT
        ▼
   Limine arranca y lee esa
```

**Y el preview** usa la **#1** (una config tuya, de prueba) o, con `--real`,
lee la **#2** (la compilada real) para mostrarte cómo quedó tu setup de verdad.
**Nunca escribe ninguna de las dos.**

Los assets van aparte, en la partición FAT que Limine lee:

| Path | Qué es |
|---|---|
| `/boot/splash.png` | el wallpaper del boot |
| `/boot/font.F16` | la fuente custom (CP437 bitmap) |

## La VM del preview

Es deliberadamente mínima: no bootea nada, solo muestra el menú.

| Aspecto | Valor |
|---|---|
| **Tamaño del ISO** | **4.9 MB** |
| Contenido | 4 binarios de Limine + `limine.conf` + el wallpaper |
| Kernel / initramfs / rootfs | **ninguno** |
| Memoria | 512 MB |
| Aceleración | KVM (`/dev/kvm`) |
| Firmware | OVMF (UEFI), igual que la máquina real |
| Arranque | ~2.9 s |

El ISO son literalmente los cuatro binarios que instala Limine
(`BOOTX64.EFI` 377 KB, `limine-bios-cd.bin` 26 KB, `limine-bios.sys` 334 KB,
`limine-uefi-cd.bin` 3.1 MB) más el conf y el PNG. El de 3.1 MB es la imagen
UEFI completa de Limine: no se puede recortar sin recompilar el bootloader.

**Flags de QEMU, medidos** (4 corridas cada variante, mismo ISO):

| Configuración | Media |
|---|---|
| **Actual** (`-enable-kvm -m 512 -display none`) | **2.80-2.91 s** |
| + `-cpu host` | 3.56 s ❌ |
| + `-cpu host -m 256` | 3.39 s ❌ |
| + `-machine q35` | 2.98 s |

`-cpu host` **empeora** el arranque: le pasa a OVMF todas las features de la
CPU real, y las tiene que inicializar. Para un bootloader no aporta nada.

Se sacó `-vnc :0` del modo sin GUI: abría el puerto 5900 sin que nadie lo
usara (y chocaba si había otra VM). El screendump va por el socket del
monitor, no por VNC.

## Plymouth (el splash de arranque)

Lo que se ve **después** de elegir CachyOS. No es Limine: es otro sistema.

```bash
boot-preview animation access               # una vez por sesión: acceso de root al X
boot-preview animation                      # ventana con el splash
boot-preview animation shot [archivo]       # captura + contact sheet
boot-preview animation sources              # los temas de Plymouth instalados
boot-preview animation preview <tema>       # tema SOLO del preview
boot-preview animation use <tema>           # aplica un tema al sistema (-R)
boot-preview animation steam-source         # los videos de Steam (cuál está activo)
boot-preview animation steam-convert <vid>  # convierte el video de Steam en tema
```

Usa el **Plymouth oficial del sistema** con el renderer X11: dibuja en una
ventana GTK sobre tu escritorio. Sin VM, sin initramfs, sin QEMU.

> **Requisito**: `plymouthd` exige root, y root necesita acceso al servidor
> X (`xhost +SI:localuser:root`). Sin eso el splash no aparece y no hay
> error. `boot-preview animation access` lo resuelve.

Detalle en [docs/PLYMOUTH.md](docs/PLYMOUTH.md).

**Vicinae no es esto**: es un launcher de escritorio.

## Rendimiento

Medido en esta máquina (8 núcleos, QEMU con KVM):

| Operación | Antes | Ahora | Cómo |
|---|---|---|---|
| `loader shot` (captura) | 15.6 s | **2.8-3.1 s** | polling del framebuffer en vez de `sleep 15` fijo |
| `make-wallpaper` (resize + blur) | 0.9 s | **0.2-0.4 s** | vips (libvips) en vez de ImageMagick |
| `prepare-wallpapers` (2 versiones) | ~2 s | **0.9 s** | idem |
| `animation` (ventana) | ~30 s (VM) | **~3 s** | el renderer X11, sin initramfs ni QEMU |
| `animation shot` | ~40 s (VM) | **10.5 s** | 3 s de arranque + 4 s de muestreo |

El `animation shot` toma 10.5 s a propósito: **8 muestras cada 500 ms** para
capturar la animación en movimiento, no un frame al azar. La ventana en sí
aparece a los ~3 s.

Detalles de los cambios:

- **El `sleep` fijo era el 96% del tiempo.** El script ahora le pregunta al
  monitor de QEMU cada 300 ms. La señal buena no es "hay algo pintado" (el
  logo de TianoCore ya cuenta), sino el **cambio de resolución**: OVMF arranca
  en 1280x800 y Limine la cambia a la `interface_resolution` al tomar el
  control.
- **vips es ~6x más rápido que ImageMagick** para resize+blur (105 ms vs
  681 ms por imagen). El resultado es equivalente (RMSE 0.94%, indistinguible
  a ojo). Se usa vips si está, y si no `magick`: el script sigue funcionando.
- **QOI sigue pasando por ImageMagick**: vips no escribe ese formato.

> El tool está escrito en Go: un solo binario, sin dependencias de runtime.
> El trabajo real lo hacen binarios externos igual (qemu, xorriso, vips), pero
> el arranque es de ~3 ms. Ver [docs/RENDIMIENTO.md](docs/RENDIMIENTO.md).

## El daemon en background

`boot-preview animation` levanta el `plymouthd` **desacoplado**: el comando
devuelve el control y la ventana sigue viva aunque cierres la terminal.

Para controlarlo, el propio tool:

```bash
boot-preview animation status          # ¿está vivo? ¿qué tema muestra?
boot-preview animation show            # mostrar el splash
boot-preview animation hide            # ocultarlo
boot-preview animation reload          # recargar tras editar un PNG
boot-preview animation update "Cargando"  # cambiar el mensaje de estado
boot-preview animation stop            # cerrar el daemon
```

El wrapper cubre los comandos de Plymouth con nombres claros y pone un
timeout (algunos esperan una tecla y colgarían la terminal).

Si preferís el comando en primer plano (que espera, y se cierra con
Ctrl+C):

```bash
boot-preview animation --foreground
```

---

## Instalación

**Sin clonar el repo** (lo más simple):

```bash
curl -fsSL https://github.com/Below-Z3R0/boot-preview/releases/latest/download/boot-preview.tar.gz \
  | tar xz && ./boot-preview/boot-preview install
```

O, desde el repo:

```bash
git clone https://github.com/Below-Z3R0/boot-preview.git
cd boot-preview
go build -o boot-preview ./cmd/boot-preview
./boot-preview install
```

### Instalar solo una parte

Cada módulo se instala por separado, con sus dependencias y su config. Lo
que ya está instalado no se toca:

```bash
boot-preview install loader       # solo Limine (QEMU, xorriso, OVMF…)
boot-preview install animation    # solo Plymouth (ffmpeg, xdotool, xhost)
boot-preview install --no-deps    # sin tocar paquetes del sistema
```

### Verificar y actualizar

```bash
boot-preview verify    # chequea módulos, dependencias y config
boot-preview update    # trae la última versión
```

`verify` compara las versiones de las dependencias con un snapshot que se
guarda al instalar. Si algo del sistema se actualizó, te avisa:

```
  Cambios desde la instalación
    ! plymouth       26.134.222-5 → 26.140.1-1
```

El preview usa el software del sistema, así que ya está usando la versión
nueva: el aviso es informativo.

### El hook de pacman

Para que el sistema te avise **solo** cuando actualizás Plymouth o
Limine:

```bash
boot-preview hooks install     # instala el hook
boot-preview hooks status      # qué disparadores tiene
boot-preview hooks uninstall   # lo saca
```

Solo informa, no modifica nada.

### Qué hace el `install`

Es idempotente: podés volver a correrlo las veces que quieras.

| Paso | Qué hace |
|---|---|
| **1. Dependencias** | verifica (e instala con sudo si falta) `qemu-desktop`, `libisoburn` (xorriso), `imagemagick`, `socat`, `edk2-ovmf` para Limine; `ffmpeg`, `xdotool`, `xorg-xhost` para Plymouth. Avisa si falta `/dev/kvm` |
| **2. El binario** | copia el binario `boot-preview` a `~/.local/bin/`. Los assets (los ejemplos) van a `~/.local/lib/boot-preview/assets/`, **fuera del PATH**: el usuario solo ve un comando. Más un shim en `/usr/local/bin/` para que `sudo boot-preview` funcione (el `secure_path` de sudo no incluye `~/.local/bin`) |
| **3. La config** | crea `~/.config/boot-preview/loader/limine.conf` desde `examples/loader.conf`. Si ya había una, la guarda como `.bak` |
| **4. El wallpaper** | copia el de ejemplo a `~/.config/boot-preview/loader/wallpaper/rei.png` (no lo pisa si ya está) |
| **5. El repo** | guarda la ruta del repo en `~/.config/boot-preview/repo` (el CLI la usa para saber dónde van las capturas) y crea `<repo>/previews/` |

**No toca nada del sistema**: ni `/boot/limine.conf`, ni
`/etc/default/limine`, ni el initramfs. Eso es del `apply`, que es un acto
deliberado y aparte.

O a mano, paso a paso:

```bash
# 1. Dependencias
sudo pacman -S --needed qemu-desktop libisoburn edk2-ovmf imagemagick \
    ffmpeg xdotool xorg-xhost

# 2. Compilar e instalar el binario
go build -o /tmp/boot-preview ./cmd/boot-preview
/tmp/boot-preview install

# O a mano, sin pasar por el instalador:
mkdir -p ~/.local/bin ~/.local/lib/boot-preview/assets
install -m 755 /tmp/boot-preview ~/.local/bin/boot-preview
cp -r examples/* ~/.local/lib/boot-preview/assets/
```

### Preparar un wallpaper con blur

Limine no puede difuminar: dibuja la imagen tal cual. Así que el blur se
aplica **antes**, al preparar el fichero:

```bash
boot-preview loader add MI_IMAGEN.png    # la deja como wallpaper del preview
boot-preview loader wallpapers           # genera la real y la difuminada
```

El `install` además deja un **shim** en `/usr/local/bin/boot-preview` para
que `sudo boot-preview loader` también funcione (ver [la trampa del
`sudo`](#las-trampas)).

### Qué queda instalado y dónde

| Path | Qué es |
|---|---|
| `~/.local/bin/boot-preview` | **el único comando** en el PATH |
| `~/.local/lib/boot-preview/assets/` | los ejemplos (loader.conf, wallpapers) |
| `/etc/pacman.d/hooks/boot-preview.hook` | la definición del hook |
| `/usr/local/bin/boot-preview` | el shim para `sudo boot-preview` |
| `~/.config/boot-preview/repo` | la ruta del repo (para las capturas en `previews/`) |
| `~/.config/boot-preview/loader/limine.conf` | la **config de prueba** del menú |
| `~/.config/boot-preview/loader/wallpaper/rei.png` | el wallpaper de ejemplo (nítido) |
| `~/.config/boot-preview/loader/wallpaper/wallpaper.png` | **la imagen activa** (la que usan preview y `apply`) |
| `~/.config/boot-preview/loader/wallpaper/rei-blur.png` | la difuminada (la genera `loader wallpapers`) |
| `~/.config/boot-preview/animation/theme` | el tema del preview de Plymouth |

Los módulos y los auxiliares viven **fuera del PATH** a propósito: el
usuario ve un comando, no cinco. Se localizan por path relativo al propio
script.

Y **no** toca nada del sistema: ni `/boot/limine.conf`, ni
`/etc/default/limine`, ni el initramfs.

## Uso

Un solo comando, dos herramientas:

```bash
boot-preview <loader|animation> [comando] [opciones]
```

| Herramienta | Qué previsualiza |
|---|---|
| **`loader`** | El menú de Limine (el bootloader) |
| **`animation`** | El splash de Plymouth (la animación) |

### `boot-preview loader` — el menú de Limine

```bash
# VER
boot-preview loader                # preview del menú (lo de siempre)
boot-preview loader real           # con tu config real de /boot/limine.conf
boot-preview loader wall foto.png  # preview con otro wallpaper (sin instalarlo)

# CAPTURAR
boot-preview loader shot           # captura a PNG (van a <repo>/previews/)
boot-preview loader shot mi.png    # captura a un archivo

# EDITAR
boot-preview loader add foto.png   # pone una imagen como wallpaper del preview
boot-preview loader edit           # abre la config del preview en tu editor

# APLICAR AL LIMINE REAL
boot-preview loader apply           # wallpaper + config visual
boot-preview loader apply config    # solo la config visual
boot-preview loader apply wallpaper # solo el wallpaper

# OTROS
boot-preview loader wallpapers     # genera las dos versiones del wallpaper
boot-preview loader help           # la ayuda
```

En la ventana de QEMU: **flechas** para moverte, **ENTER** para bootear,
**E** para editar la config, **F1** para el firmware, **ESC** para salir.
**Ctrl+Q** cierra QEMU.

### `boot-preview animation` — el splash de Plymouth

```bash
# ACCESO (una vez por sesión)
boot-preview animation access       # da acceso de root al servidor X

# VER
boot-preview animation              # ventana con el splash
boot-preview animation x11 15       # idem, con límite de tiempo

# CAPTURAR
boot-preview animation shot         # captura a PNG + contact sheet
boot-preview animation shot mi.png  # captura a un archivo

# TEMAS
boot-preview animation sources      # los temas de Plymouth instalados
boot-preview animation preview <tema>  # tema SOLO del preview
boot-preview animation use <tema>      # tema del SISTEMA (-R)

# STEAM
boot-preview animation steam-source     # los videos de Steam (cuál está activo)
boot-preview animation steam-convert    # convierte el video en tema
boot-preview animation steam-apply      # convierte y lo aplica
```

### Instalación y mantenimiento

```bash
boot-preview install [loader|animation]  # instalar (o solo una parte)
boot-preview verify                      # verificar la instalación
boot-preview update                      # traer la última versión
boot-preview hooks [install|uninstall]   # el hook de pacman
```

### El flujo completo

```bash
boot-preview loader add ~/Pictures/foto.png   # 1. la imagen entra al preview
boot-preview loader                           # 2. la mirás
boot-preview loader edit                      # 3. ajustás la config (colores, margin)
boot-preview loader                          # 4. la volvés a ver
boot-preview loader apply                    # 5. al Limine real
sudo reboot                             # 6. a bootear
```

`apply` es **idempotente**: aplicarlo dos veces no duplica nada en el conf.

### ¿Puede `apply` romper la config?

No. Está verificado con tests sobre el conf real (957 líneas, 174 entradas):

| Test | Resultado |
|---|---|
| Las entradas del menú quedan intactas | 933 líneas no-theming **idénticas** |
| Idempotente (aplicar 2 veces) | md5 **igual** |
| No duplica el bloque | 1 `wallpaper:`, 1 cabecera |
| Se preservan las 174 entradas | 174 → 174 |
| Conf sin theming (simula `limine-update`) → `apply` | vuelve **byte a byte idéntico** |
| Comentarios huérfanos → `apply` | no duplica, vuelve al original |

`apply` **solo** toca:
- las opciones de theming en `/boot/limine.conf` (las 14 de la lista)
- `/boot/splash.png`

Todo lo demás (entradas del kernel, snapshots, `timeout`, `default_entry`) no
se toca.

Además hace un **backup automático** en `/boot/limine.conf.bak-YYYYMMDD-HHMMSS`
antes de escribir (se conservan los 5 más recientes; la ESP no conviene
llenarla).

### Si una actualización rompe el conf

**Caso 1 — `limine-update` regenera el conf y se pierde el theming.**
Es el caso normal. Las entradas del menú las regenera la herramienta, y el
bloque de theming se pierde. Se recupera con:

```bash
boot-preview loader apply
```

Verificado: partiendo de un conf sin theming, `apply` lo deja **idéntico byte
a byte** al original.

**Caso 2 — el conf desapareció entero.**
`apply` **no** puede reconstruir las entradas del menú: no las tiene. Falla
con un mensaje claro y te dice de dónde restaurar:

```bash
sudo ls -1t /boot/limine.conf.bak-* /boot/limine.conf.old
sudo cp /boot/limine.conf.bak-XXXX /boot/limine.conf
boot-preview loader apply
```

**Caso 3 — el splash se perdió.**
`boot-preview loader apply wallpaper` lo reescribe desde
`~/.config/boot-preview/loader/wallpaper/wallpaper.png`.

### Qué hace `apply`

| | Qué toca |
|---|---|
| `apply wallpaper` | copia el wallpaper del preview a `/boot/splash.png` |
| `apply config` | inyecta el bloque de theming de la config del preview en `/boot/limine.conf`, antes de la primera entrada |
| `apply` | las dos cosas |

El path del wallpaper se traduce solo: en el preview es `boot():/wallpaper.png`
(lo que el script mete en el ISO) y en el Limine real queda `boot():/splash.png`.

### Opciones avanzadas

Para lo raro están los flags, ocultos en la ayuda:

| Opción | Qué hace |
|---|---|
| `-c, --config <ruta>` | Config de Limine a usar (default: `~/.config/boot-preview/loader/limine.conf`) |
| `-w, --wallpaper <ruta>` | Wallpaper a incluir en el ISO |
| `--resolution <WxH>` | Resolución de la interfaz (default: `1920x1080`) |
| `--timeout <seg>` | Espera antes de capturar (default: 15) |

## Requisitos

| Pieza | Para qué | Sin ella |
|---|---|---|
| `qemu-system-x86_64` | la VM | nada |
| `libisoburn` (`xorriso`) | construir el ISO | nada |
| `edk2-ovmf` | firmware UEFI | el menú sale en **modo texto**, sin wallpaper |
| `imagemagick` | convertir la captura | la captura sale en PPM |
| `/dev/kvm` | aceleración | la VM va por emulación (lenta) |

**KVM importa**: sin él la VM arranca en minutos en vez de segundos. En CachyOS
con Intel/AMD moderno está presente por defecto.

## Las trampas

Cuatro cosas que hacen fallar esto, y que el script ya resuelve:

### 1. Las opciones globales van ANTES de las entradas

En `limine.conf`, una entrada de menú empieza con `/`. **Cualquier opción
después de esa línea pertenece a esa entrada**, no es global. Si inyectás
`wallpaper:` al final del archivo, Limine lo ignora y te queda el fondo
default.

```ini
# correcto
wallpaper: boot():/wallpaper.png     ← global, antes de las entradas
interface_resolution: 1920x1080

/CachyOS
protocol: linux
```

### 2. `timeout: 0` bootea sin mostrar el menú

| Valor | Efecto |
|---|---|
| `timeout: 0` | **bootea instantáneamente**, sin mostrar el menú |
| `timeout: no` | muestra el menú siempre ← lo que querés para el preview |
| `timeout: 5` | cuenta 5 s y bootea |

Con la config real (que trae `timeout: 5`) Limine autobooteba la entrada
default y se iba de pantalla, mostrando un error de kernel en vez del menú.

### 3. El CD es `-cdrom`, no `-drive if=virtio`

```bash
# mal
-drive file=x.iso,format=raw,if=virtio,media=cdrom   # → "Could not read from CDROM (code 0003)"
# bien
-cdrom x.iso
```

### 4. `sudo boot-preview loader` → "command not found"

`~/.local/bin` no está en el `secure_path` de sudo. Se resuelve con un shim
en `/usr/local/bin/boot-preview` que, si lo invoca root, **baja al usuario
que lo lanzó** — así QEMU no corre como root y los archivos del preview no
quedan propiedad de root.

## Wallpapers: blur y calidad

Limine **no puede difuminar** una imagen — la dibuja tal cual. Y su
`term_margin_gradient` **no es un blur**: es un velo del color de
`term_background`, que se ve mal. La solución es hacer el blur **antes**, en la
imagen, y dejar a Limine solo con el wallpaper.

### Las dos versiones

Limine **no puede difuminar** una imagen — la dibuja tal cual. Y su
`term_margin_gradient` **no es un blur**: es un velo del color de
`term_background`, que se ve mal.

La solución es que **la imagen llegue a Limine ya difuminada**. Limine no
renderiza nada: solo la muestra. El desenfoque se aplica antes, a la imagen.

```bash
# genera las dos versiones de una
prepare-wallpapers
```

| Fichero | Qué es |
|---|---|
| `rei.png` | la fuente, sin blur |
| `rei-blur.png` | la misma, **ya con el blur aplicado** |
| `wallpaper.png` | **la activa**: la que usan el preview y `apply`. `prepare-wallpapers` deja acá la difuminada |

### El pipeline de cada una

```bash
make-wallpaper IMAGEN -o SALIDA [opciones]
```

| Orden | Qué hace |
|---|---|
| 1. `--res 1920x1080` | escala a la resolución de la interfaz con Lanczos, recortando al centro (**1:1, sin reescalado**) |
| 2. `--blur 0x30` | **desenfoque gaussiano pre-renderizado** (solo en la versión difuminada) |
| 3. `--dim 0.4` | velo uniforme encima (opcional) |

```bash
# la real (nítida)
make-wallpaper examples/wallpaper/rei.png -o /tmp/real.png --res 1920x1080

# la difuminada: la imagen ya viene con el blur aplicado
make-wallpaper examples/wallpaper/rei.png -o /tmp/blur.png \
  --res 1920x1080 --blur 0x30
```

> Para el **upscale con IA** va primero [Upscayl](https://github.com/upscayl/upscayl)
> (x4), y esa salida entra acá con `boot-preview loader add`. Este repo no baja
> modelos ni depende de Real-ESRGAN.

Para ver cada una en el preview:

```bash
boot-preview loader wall ~/.config/boot-preview/loader/wallpaper/rei-blur.png
boot-preview loader wall ~/.config/boot-preview/loader/wallpaper/rei.png
```

Y la config de Limine, **sin margen ni degradado** (que es lo que se veía feo):

```ini
term_margin: 0
term_margin_gradient: 0
term_background: FF191924
term_foreground: e0def4
```

> El alpha de `term_background` va **al revés** de lo que dice la doc oficial
> (`common/lib/gterm.c`: `alpha = 255 - A(fg)`). `TT=00` → el color tapa el
> wallpaper (recuadro negro). `TT=FF` → pasa el wallpaper (limpio).

### Otras opciones

El wallpaper se prepara con `boot-preview loader add`:

```bash
# la nitida, tal cual
boot-preview loader add foto.png

# la difuminada (Limine no puede difuminar: se aplica antes)
boot-preview loader wallpapers
```

**Para máxima calidad** hay dos caminos:

1. **Que el wallpaper mida al menos lo que la `interface_resolution`** (si es
   más chico, Limine lo escala y se ve borroso) y que esté en PNG o QOI, que
   son lossless. Limine no tiene opción de calidad de imagen: la calidad es la
   del fichero.

2. **Mejorarlo con IA antes de usarlo**, con
   [Upscayl](https://github.com/upscayl/upscayl) (la app de escritorio, con
   los modelos de Real-ESRGAN y ESRGAN anime). Se usa fuera de este repo:
   escalás la imagen a 4×, la guardás, y esa entra al preview:

   ```bash
   # en Upscayl: escala x4 con el modelo "RealESRGAN Anime"
   boot-preview loader add ~/Pictures/foto-4x.png
   ```

   El truco: upscalear x4 y después **bajar** a la resolución final con
   Lanczos. El resultado tiene más detalle perceptible que el original, sin
   inventar textura nueva.

   Para el blur, `boot-preview loader wallpapers` lo aplica sobre la imagen
   ya mejorada.

### ⚠️ El alpha de `term_background` va al revés

Para que el wallpaper se vea **limpio detrás del texto** (sin panel, sin
velo), el byte de alpha tiene que ser **alto**:

```ini
term_background: FF191924    # FF = el wallpaper se ve completo
term_margin: 64
term_margin_gradient: 4
```

Con `00` el color tapa el wallpaper y sale un **recuadro negro**. La doc
oficial dice "TT stands for transparency", pero el blend en
`common/lib/gterm.c` lo invierte: cuanto más alto el byte, **más wallpaper**
se ve. Los valores útiles: `FF` limpio, `80` panel translúcido, `40` velo
fuerte.

El wallpaper generado va a `/boot/splash.png` para el Limine real, o se
prueba sin tocar nada con:

```bash
boot-preview loader wall ~/Pictures/foto-borrosa.png
```

## Lo que se puede personalizar

**52 opciones globales** en `limine.conf`, 16 de ellas de theming. El detalle
completo está en [`docs/THEMING.md`](docs/THEMING.md).

El resumen:

| | |
|---|---|
| **Wallpaper** | PNG, BMP, JPEG, **QOI**. Repetible → Limine **sortea una al azar** |
| **Estilo** | `tiled`, `centered`, `stretched` |
| **Panel ("card")** | `term_margin` + `term_margin_gradient` + `term_background` con alpha |
| **Paleta** | 16 colores (`term_palette` + `term_palette_bright`), separados por **`;`** |
| **Fuente** | `term_font` (CP437 bitmap, **no TTF**), `term_font_size`, `term_font_scale`, `term_font_spacing` |
| **Interfaz** | `interface_resolution`, `interface_rotation`, `interface_branding`, `interface_help_hidden` |

**Y lo que NO existe**: cards, hover, widgets, iconos por entrada,
animaciones, video, blur. El menú es **texto plano** (`common/menu.c` →
`print_tree()`), dibujado con `print()` de caracteres.

> Limine es **BSD-2-Clause**: forkear `common/menu.c` y recompilar para hacer
> cards es legal. Ver [`docs/THEMING.md`](docs/THEMING.md).

## Aplicar el tema al Limine real

El preview es para iterar; para el cambio de verdad el bloque de theming va
en **`/boot/limine.conf`**, antes de la primera entrada (una línea que empieza
por `/`).

> Contraryendo lo que se suele creer, **`/etc/default/limine` no genera el
> conf**: solo aporta `ESP_PATH`, `KERNEL_CMDLINE` y `BOOT_ORDER`. Y
> **`limine-snapper-sync` preserva** el bloque (verificado: sobrevive intacto
> y el diff de las entradas da cero). El paso a paso está en
> [`docs/APLICAR.md`](docs/APLICAR.md).

```bash
# 1. Backup
sudo cp /boot/limine.conf /boot/limine.conf.pre-theming

# 2. El wallpaper (la version difuminada, ya con el blur aplicado)
sudo cp ~/.config/boot-preview/loader/wallpaper/rei-blur.png /boot/splash.png

# 3. El bloque de theming, antes de la primera entrada
sudo nano /boot/limine.conf

# 4. Reiniciar
sudo reboot
```

No hace falta `limine-mkinitcpio`.

## Documentación

| Documento | Qué cubre |
|---|---|
| [`docs/FLUJO.md`](docs/FLUJO.md) | El flujo completo del menú: de la config a la pantalla, y las 4 trampas en detalle |
| [`docs/THEMING.md`](docs/THEMING.md) | Las 52 opciones, qué se puede personalizar, los límites y el código detrás |
| [`docs/APLICAR.md`](docs/APLICAR.md) | Cómo aplicar el tema al `/etc/default/limine` real, con rollback |
| [`docs/PLYMOUTH.md`](docs/PLYMOUTH.md) | El splash de Plymouth: el renderer X11, el plugin `script`, y cómo convertir un video de Steam en tema |
| [`docs/RENDIMIENTO.md`](docs/RENDIMIENTO.md) | Las mediciones y por qué el tool está en Go |
| [`examples/loader.conf`](examples/loader.conf) | La config de prueba, comentada |
| [`examples/theming-rosepine.conf`](examples/theming-rosepine.conf) | El bloque de theming para pegar en `/etc/default/limine` |
| [`cmd/boot-preview/main.go`](cmd/boot-preview/main.go) | El punto de entrada: el dispatcher de los subcomandos |
| [`internal/loader/`](internal/loader/) | El módulo de Limine |
| [`internal/animation/`](internal/animation/) | El módulo de Plymouth |
| [`internal/wallpaper/`](internal/wallpaper/) | Blur, velo y escalado del wallpaper (Limine no puede difuminar) |
| [`internal/steam/`](internal/steam/) | El parser de las animaciones de startup de Steam |
| [`internal/install/`](internal/install/) | El instalador |
| [`internal/verify/`](internal/verify/) | La verificación post-instalación |
| [`internal/pack/`](internal/pack/) | El paquete descargable y el hook de pacman |
| [`scripts/smoke-test.sh`](scripts/smoke-test.sh) | La batería de pruebas (sin root, corre en CI) |

## Licencia

**MIT**. Todo el código de este repo es original.
