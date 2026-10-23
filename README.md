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
└── APLICAR.md             # cómo aplicar el tema al Limine real
scripts/
└── install.sh             # deja todo andando
```

## Instalación

```bash
git clone https://github.com/tu-usuario/limine-preview.git
cd limine-preview
./scripts/install.sh
```

O a mano, paso a paso:

```bash
# 1. Dependencias
sudo pacman -S --needed qemu-desktop libisoburn edk2-ovmf imagemagick

# 2. El script
install -m 755 examples/limine-preview.sh ~/.local/bin/limine-preview

# 3. La config de prueba
mkdir -p ~/.config/limine-preview
install -m 644 examples/limine-preview.conf ~/.config/limine-preview/limine.conf

# 4. El wallpaper
mkdir -p ~/.config/limine-preview/wallpaper
install -m 644 examples/wallpaper/rei.png ~/.config/limine-preview/wallpaper/rei.png
```

El `install.sh` hace todo eso y además deja un **shim** en
`/usr/local/bin/limine-preview` para que `sudo limine-preview` también
funcione (ver [la trampa del `sudo`](#las-trampas)).

## Uso

```bash
# Ver el menu con la config de prueba (ventana de QEMU)
limine-preview

# Verlo con TU config real de /boot/limine.conf
limine-preview --real

# Capturar a PNG y salir (sin ventana)
limine-preview --shot ~/preview.png

# Probar otro wallpaper
limine-preview --wallpaper /ruta/imagen.png

# Otra resolucion de la interfaz
limine-preview --resolution 2560x1440
```

En la ventana de QEMU: **flechas** para moverte, **ENTER** para bootear,
**E** para editar la config, **F1** para el firmware, **ESC** para salir.
**Ctrl+Q** cierra QEMU.

### Opciones

| Opción | Qué hace |
|---|---|
| `-c, --config <ruta>` | Config de Limine a usar (default: `~/.config/limine-preview/limine.conf`) |
| `-w, --wallpaper <ruta>` | Wallpaper a incluir en el ISO |
| `--real` | Usar la config real de `/boot/limine.conf` (lee con sudo) |
| `--resolution <WxH>` | Resolución de la interfaz (default: `1920x1080`) |
| `--shot <ruta.png>` | Captura el framebuffer a PNG y sale (sin ventana) |
| `--gui` | Abre la ventana de QEMU (por defecto) |
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

El preview es para iterar; para el cambio de verdad hay que tocar
`/etc/default/limine` (la fuente), **no** `/boot/limine.conf` (que se
sobrescribe). El paso a paso está en
[`docs/APLICAR.md`](docs/APLICAR.md).

```bash
# 1. El wallpaper
sudo cp examples/wallpaper/rei.png /boot/splash.png

# 2. El bloque de theming en /etc/default/limine
sudo nano /etc/default/limine

# 3. Regenerar la config compilada
sudo limine-mkinitcpio

# 4. Reiniciar
```

## Documentación

| Documento | Qué cubre |
|---|---|
| [`docs/FLUJO.md`](docs/FLUJO.md) | El flujo completo: de la config a la pantalla, y las 4 trampas en detalle |
| [`docs/THEMING.md`](docs/THEMING.md) | Las 52 opciones, qué se puede personalizar, los límites y el código detrás |
| [`docs/APLICAR.md`](docs/APLICAR.md) | Cómo aplicar el tema al `/etc/default/limine` real, con rollback |
| [`examples/limine-preview.conf`](examples/limine-preview.conf) | La config de prueba, comentada |
| [`examples/theming-rosepine.conf`](examples/theming-rosepine.conf) | El bloque de theming para pegar en `/etc/default/limine` |

## Licencia

**MIT**. Todo el código de este repo es original.
