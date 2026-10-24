# limine-preview

Ver el **menú de Limine** sin reiniciar la máquina.

Limine no tiene previewer: no hay nada en el repo upstream, ni herramienta
online, ni preview en la comunidad. La única forma de ver el menú sin
reiniciar es **emularlo**. Este repo lo hace:

```
tu config de Limine  →  ISO híbrido  →  QEMU con KVM  →  el menú real
```

El render es **exacto**: es el mismo bootloader, el mismo wallpaper y la
misma paleta. Tu `/boot/limine.conf` **no se toca**.

```
examples/
├── limine-preview.sh      # el script (fuente de verdad)
├── limine-preview.conf    # la config de prueba
├── theming-rosepine.conf  # el bloque de theming para aplicar al sistema
└── wallpaper/rei.png       # el wallpaper de ejemplo
    rosepine.png            # un gradiente Rosé Pine alternativo
    rosepine.qoi            # el mismo, en QOI (Limine lo soporta nativamente)
docs/
├── FLUJO.md               # el flujo completo, de la config a la pantalla
├── THEMING.md             # las 52 opciones y qué se puede personalizar
├── APLICAR.md             # cómo aplicar el tema al Limine real
└── RENDIMIENTO.md         # las mediciones y la comparativa Bun vs bash
scripts/
├── install.sh              # deja todo andando
├── make-wallpaper.sh       # blur + velo + resolución, para Limine
└── prepare-wallpapers.sh   # genera las dos versiones (real y difuminada)
```

## Las tres configs de Limine

Limine tiene **más de un** fichero de configuración, y el preview las usa de
forma distinta. Esta tabla es la que hay que tener clara:

| # | Path | Qué es | Quién la escribe | ¿La carga el preview? |
|---|---|---|---|---|
| **1** | `~/.config/limine-preview/limine.conf` | la config **de prueba** — la que edita el usuario para iterar el tema | `install.sh` la instala desde `examples/limine-preview.conf`; el script la inyecta con el wallpaper | ✅ **por default** |
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

## Rendimiento

Medido en esta máquina (8 núcleos, QEMU con KVM):

| Operación | Antes | Ahora | Cómo |
|---|---|---|---|
| `shot` (captura) | 15.6 s | **2.8-3.1 s** | polling del framebuffer en vez de `sleep 15` fijo |
| `make-wallpaper` (resize + blur) | 0.9 s | **0.2-0.4 s** | vips (libvips) en vez de ImageMagick |
| `prepare-wallpapers` (2 versiones) | ~2 s | **0.9 s** | idem |

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

> Bun no entra acá: arranca en ~117 ms contra los 2 ms de bash, y el trabajo
> real lo hacen binarios externos (qemu, xorriso, vips). Ver
> [docs/RENDIMIENTO.md](docs/RENDIMIENTO.md).

## Instalación

```bash
git clone https://github.com/tu-usuario/limine-preview.git
cd limine-preview
./scripts/install.sh
```

### Qué hace el `install.sh`

Es idempotente: podés volver a correrlo las veces que quieras.

| Paso | Qué hace |
|---|---|
| **1. Dependencias** | verifica (e instala con sudo si falta) `qemu-desktop`, `libisoburn` (xorriso), `imagemagick`, `socat`, `edk2-ovmf`. Avisa si falta `/dev/kvm` |
| **2. Los scripts** | `limine-preview`, `make-wallpaper` y `prepare-wallpapers` a `~/.local/bin/`. Más un shim en `/usr/local/bin/` para que `sudo limine-preview` funcione (el `secure_path` de sudo no incluye `~/.local/bin`) |
| **3. La config** | crea `~/.config/limine-preview/limine.conf` desde `examples/`. Si ya había una, la guarda como `.bak` |
| **4. El wallpaper** | copia el de ejemplo a `~/.config/limine-preview/wallpaper/rei.png` (no lo pisa si ya está) |
| **5. El repo** | guarda la ruta del repo en `~/.config/limine-preview/repo` (el CLI la usa para saber dónde van las capturas) y crea `<repo>/previews/` |

**No toca nada del sistema**: ni `/boot/limine.conf`, ni
`/etc/default/limine`, ni el initramfs. Eso es del `apply`, que es un acto
deliberado y aparte.

O a mano, paso a paso:

```bash
# 1. Dependencias
sudo pacman -S --needed qemu-desktop libisoburn edk2-ovmf imagemagick

# 2. El script
install -m 755 examples/limine-preview.sh ~/.local/bin/limine-preview

# 3. La config de prueba
mkdir -p ~/.config/limine-preview
install -m 644 examples/limine-preview.conf ~/.config/limine-preview/limine.conf

# 4. El wallpaper (rei.png + la activa)
mkdir -p ~/.config/limine-preview/wallpaper
install -m 644 examples/wallpaper/rei.png ~/.config/limine-preview/wallpaper/rei.png
cp ~/.config/limine-preview/wallpaper/rei.png \
   ~/.config/limine-preview/wallpaper/wallpaper.png

# 4b. La ruta del repo (para que `shot` sepa dónde guardar)
dirname "$(readlink -f examples/limine-preview.sh)" | sed 's|/examples$||' \
  > ~/.config/limine-preview/repo

# 5. (opcional) los generadores de wallpapers
install -m 755 scripts/make-wallpaper.sh ~/.local/bin/make-wallpaper
install -m 755 scripts/prepare-wallpapers.sh ~/.local/bin/prepare-wallpapers
```

### Preparar un wallpaper con blur

```bash
make-wallpaper.sh IMAGEN [-o SALIDA] [--blur 0x30] [--dim 0.4]
                            [--res 1920x1080] [--format png|qoi]
                            [--overlay COLOR]
```

El `install.sh` hace todo eso y además deja un **shim** en
`/usr/local/bin/limine-preview` para que `sudo limine-preview` también
funcione (ver [la trampa del `sudo`](#las-trampas)).

### Qué queda instalado y dónde

| Path | Qué es |
|---|---|
| `~/.local/bin/limine-preview` | el script (o `~/.config/limine-preview/limine.conf` si preferís `$XDG_CONFIG_HOME`) |
| `~/.config/limine-preview/limine.conf` | la **config de prueba** (la #1 de la tabla de arriba) |
| `~/.config/limine-preview/wallpaper/rei.png` | el wallpaper de ejemplo (nítido) |
| `~/.config/limine-preview/wallpaper/wallpaper.png` | **la imagen activa** (la que usan preview y `apply`) |
| `~/.config/limine-preview/repo` | la ruta del repo (para las capturas en `previews/`) |
| `~/.config/limine-preview/limine.conf` | la config de prueba |
| `/usr/local/bin/limine-preview` | el shim para `sudo limine-preview` |

Y **no** toca nada del sistema: ni `/boot/limine.conf`, ni
`/etc/default/limine`, ni el initramfs.

## Uso

La CLI es de **subcomandos**, no de flags:

```bash
# VER
limine-preview                # preview del menú (lo de siempre)
limine-preview real           # con tu config real de /boot/limine.conf
limine-preview wall foto.png  # preview con otro wallpaper (sin instalarlo)

# CAPTURAR
limine-preview shot           # captura a PNG (van a <repo>/previews/)
limine-preview shot mi.png    # captura a un archivo

# EDITAR
limine-preview add foto.png   # pone una imagen como wallpaper del preview
limine-preview edit           # abre la config del preview en tu editor

# APLICAR AL LIMINE REAL
limine-preview apply           # wallpaper + config visual
limine-preview apply config    # solo la config visual
limine-preview apply wallpaper # solo el wallpaper

# OTROS
limine-preview wallpapers     # genera las dos versiones del wallpaper
limine-preview setup          # instala dependencias y archivos
limine-preview help           # la ayuda
```

En la ventana de QEMU: **flechas** para moverte, **ENTER** para bootear,
**E** para editar la config, **F1** para el firmware, **ESC** para salir.
**Ctrl+Q** cierra QEMU.

### El flujo completo

```bash
limine-preview add ~/Pictures/foto.png   # 1. la imagen entra al preview
limine-preview                           # 2. la mirás
limine-preview edit                      # 3. ajustás la config (colores, margin)
limine-preview                          # 4. la volvés a ver
limine-preview apply                    # 5. al Limine real
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
limine-preview apply
```

Verificado: partiendo de un conf sin theming, `apply` lo deja **idéntico byte
a byte** al original.

**Caso 2 — el conf desapareció entero.**
`apply` **no** puede reconstruir las entradas del menú: no las tiene. Falla
con un mensaje claro y te dice de dónde restaurar:

```bash
sudo ls -1t /boot/limine.conf.bak-* /boot/limine.conf.old
sudo cp /boot/limine.conf.bak-XXXX /boot/limine.conf
limine-preview apply
```

**Caso 3 — el splash se perdió.**
`limine-preview apply wallpaper` lo reescribe desde
`~/.config/limine-preview/wallpaper/wallpaper.png`.

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
| `-c, --config <ruta>` | Config de Limine a usar (default: `~/.config/limine-preview/limine.conf`) |
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

### 4. `sudo limine-preview` → "command not found"

`~/.local/bin` no está en el `secure_path` de sudo. Se resuelve con un shim
en `/usr/local/bin/limine-preview` que, si lo invoca root, **baja al usuario
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
> (x4), y esa salida entra acá con `limine-preview add`. Este repo no baja
> modelos ni depende de Real-ESRGAN.

Para ver cada una en el preview:

```bash
limine-preview wall ~/.config/limine-preview/wallpaper/rei-blur.png
limine-preview wall ~/.config/limine-preview/wallpaper/rei.png
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

```bash
# velo uniforme encima (si el texto no se lee)
make-wallpaper.sh foto.png --blur 0x30 --dim 0.4 --res 1920x1080

# el velo en el color de fondo del tema
make-wallpaper.sh foto.png --blur 0x30 --dim 0.45 --overlay "#191724"

# versión QOI (lossless y se decodifica más rápido que PNG)
make-wallpaper.sh foto.png --res 1920x1080 --format qoi
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
   limine-preview add ~/Pictures/foto-4x.png
   ```

   El truco: upscalear x4 y después **bajar** a la resolución final con
   Lanczos. El resultado tiene más detalle perceptible que el original, sin
   inventar textura nueva.

   Para el blur, `make-wallpaper.sh` (o `limine-preview wallpapers`) lo aplica
   sobre la imagen ya mejorada.

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
limine-preview wall ~/Pictures/foto-borrosa.png
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
sudo cp ~/.config/limine-preview/wallpaper/rei-blur.png /boot/splash.png

# 3. El bloque de theming, antes de la primera entrada
sudo nano /boot/limine.conf

# 4. Reiniciar
sudo reboot
```

No hace falta `limine-mkinitcpio`.

## Documentación

| Documento | Qué cubre |
|---|---|
| [`docs/FLUJO.md`](docs/FLUJO.md) | El flujo completo: de la config a la pantalla, y las 4 trampas en detalle |
| [`docs/THEMING.md`](docs/THEMING.md) | Las 52 opciones, qué se puede personalizar, los límites y el código detrás |
| [`docs/APLICAR.md`](docs/APLICAR.md) | Cómo aplicar el tema al `/etc/default/limine` real, con rollback |
| [`examples/limine-preview.conf`](examples/limine-preview.conf) | La config de prueba, comentada |
| [`examples/theming-rosepine.conf`](examples/theming-rosepine.conf) | El bloque de theming para pegar en `/etc/default/limine` |
| [`scripts/make-wallpaper.sh`](scripts/make-wallpaper.sh) | Blur, velo y escalado del wallpaper (Limine no puede difuminar) |
| [`scripts/prepare-wallpapers.sh`](scripts/prepare-wallpapers.sh) | Genera las dos versiones: la real y la ya difuminada |

## Licencia

**MIT**. Todo el código de este repo es original.
