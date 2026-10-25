# Rendimiento: por qué Go y no bash

Este documento tiene dos partes:

1. **El rendimiento del pipeline** (qué tarda en salir un wallpaper o una
   captura) — acá había margen real y se arregló.
2. **El lenguaje del tool** — bash alcanzaba, pero Go da tipado, tests y un
   solo binario. Los números están abajo.

---

## 1. El pipeline, antes y después

Medido en esta máquina (Intel UHD 12 núcleos, QEMU con KVM habilitado).

| Operación | Antes | Ahora | Ganancia |
|---|---|---|---|
| `boot-preview loader shot` | 15.6 s | **2.8 - 3.3 s** | **5x** |
| `boot-preview loader add --blur 0x30 --res 1920x1080` | 0.90 s | **0.32 s** | **2.8x** |
| el mismo, sin blur (sharpen) | 0.85 s | **0.36 s** | 2.4x |
| `boot-preview loader wallpapers` (las 2 versiones) | ~1.9 s | **0.90 s** | 2.1x |

### Qué se cambió

**a) El `sleep 15` fijo.** Era el 96% del tiempo de una captura. Ahora el
tool consulta el monitor de QEMU cada 300 ms.

La primera versión del polling miraba si el framebuffer tenía contenido
(`vips avg > 1`), y **capturaba el logo de TianoCore**: el firmware ya pinta
algo antes de cederle el control a Limine. La señal correcta es el **cambio
de resolución** — OVMF arranca en 1280x800 y Limine pasa a la
`interface_resolution` cuando dibuja el menú.

```
OVMF   1280x800   ← logo TianoCore, NO es el menú
Limine 1920x1080  ← cambio de resolución = el menú ya está
```

Se espera un tick extra (500 ms) después del cambio para que termine de
pintar el texto, y si se agota el timeout se captura lo que haya en vez de
fallar.

**b) ImageMagick → vips (libvips).**

| Herramienta | resize + blur 1920x1080 |
|---|---|
| `magick` (ImageMagick 7) | 681 ms |
| `vips` (libvips) | **105 ms** |
| `ffmpeg` | 251 ms |

El resultado de vips es equivalente al de magick: **RMSE 0.94%** comparando
las dos salidas del mismo wallpaper — indistinguible a ojo.

Dos trampas que aparecieron al migrar:

- `vips crop in.v in.v ...` (mismo archivo de entrada y salida) → **Bus error
  (core dumped)**. libvips no puede leer y escribir el mismo archivo en
  operaciones con acceso aleatorio. Se resuelve alternando entre dos
  temporales (`cur`/`nxt` con un `swap`).
- `vips gaussblur in out SIGMA --min_ampl 0.2` → **"Unknown option"**. En
  vips 8.18 `--min_ampl` no existe; se pasa solo el sigma. Y `vips sharpen`
  toma el sigma como **opción** (`--sigma 0.9`), no como argumento
  posicional.
- El módulo `vips-openslide` de Arch viene sin su librería y tira un warning
  en cada invocación. `VIPS_WARNING=0` **no lo silencia** (sale del loader de
  módulos): hay que redirigir stderr.

**c) QOI sigue con ImageMagick.** vips no escribe QOI, así que esa rama no
cambia.

**d) El escalado del tema de Plymouth.** `vips thumbnail --size both`
**no fuerza el aspecto**: una imagen de 1728×1080 quedaba con bandas. La
vía correcta es `magick -resize "1920x1080!"`, donde el `!` final es lo que
fuerza la proporción exacta. vips queda como atajo solo cuando la imagen ya
tiene la proporción correcta.

---

## 2. Go contra bash

### Arranque del runtime

| Runtime | Arranque |
|---|---|
| `bash -c true` | **1.1 ms** |
| `bun -e 'process.exit(0)'` | 1.4 ms |
| **binario Go** (`boot-preview version`) | **2.3 ms** |
| `python3 -c pass` | 9.9 ms |

Go paga ~1.2 ms más que bash en arrancar. Es un peaje fijo por invocación.

### Los comandos reales

Comparación del binario Go contra los scripts bash, con el mismo trabajo
(mínimo de 5 corridas, en esta máquina):

| Comando | Go | bash | Ganancia |
|---|---|---|---|
| `--help` | 3.2 ms | 5.1 ms | 1.6x |
| `loader --help` | 3.5 ms | 5.0 ms | 1.4x |
| `animation sources` | 3.5 ms | 32.7 ms | **9.3x** |
| `animation steam-source` | 4.2 ms | 27.6 ms | **6.5x** |
| `verify` | 608 ms | 653 ms | 1.1x |
| `animation shot` | 12.2 s | 12.2 s | igual |
| `loader shot` | 3.3 s | 3.4 s | igual |

**Lo que importa**: los comandos lentos (`shot`) son iguales — su tiempo lo
dominan QEMU, Plymouth y los sleeps, no el lenguaje. Los comandos cortos
(listados, ayuda) son **6-9x más rápidos** en Go, porque el bash tenía que
lanzar procesos uno por uno para leer la config y formatear la salida.

### Por qué Go igual

El argumento no es el rendimiento: es la **mantenibilidad**.

- **Tipado**: el parser de Steam y el del config de Limine manejan datos
  estructurados. En bash, un `sed` mal escrito devuelve una cadena vacía y
  el error aparece tres comandos más abajo.
- **Tests**: hay tests unitarios (`go test ./...`) que cubren el orden de
  los frames, la idempotencia del `apply`, el parser de Steam y el parseo
  de la resolución. En bash eso solo se puede probar ejecutando el comando
  entero.
- **Un solo binario**: sin `PATH`, sin `install -m 755` de cada módulo, sin
  que un script encuentre a sus hermanos. `boot-preview` es un archivo.
- **Errores claros**: `fmt.Errorf("resolución inválida: %q", res)` contra
  un `Atoi` que devuelve 0 y un PNG de 0×0.

### Qué NO se optimizó

El grueso del tiempo es de procesos externos: QEMU tarda ~2.5 s en arrancar
con KVM, Plymouth ~3 s en abrir su ventana, ffmpeg ~8 s en convertir un
video. Reescribir el tool en otro lenguaje no mueve esa aguja.

El único comando donde el lenguaje se nota es `verify` (600 ms), y ahí lo
que cuesta es `pacman -Q` por cada dependencia — un proceso por paquete.
Se podría cachear, pero `verify` se corre una vez cada tanto.

---

## 3. La decisión

El tool está en **Go**. Bash se usaba, y funcionaba, pero:

- el CLI iba a crecer (más subcomandos, más opciones) y bash se ponía
  difícil de probar;
- los errores silenciosos (un `sed` que no matchea, un `sleep` que no
  espera) ya habían costado horas.

El precio es 1 ms extra de arranque por invocación. A cambio: tipos, tests,
un binario y errores que dicen qué pasó.

---

## Cómo reproducir las mediciones

```bash
# Arranque de los runtimes
time bash -c true
time bun -e 'process.exit(0)'
time ./boot-preview version

# El pipeline de imagen
time boot-preview loader add IMAGEN --res 1920x1080 --blur 0x30
time vips gaussblur IMAGEN /tmp/x.v 30

# La captura
time boot-preview loader shot /tmp/x.png --timeout 25

# Equivalencia de las dos salidas
magick compare -metric RMSE salida-magick.png salida-vips.png null:

# La batería (sin root, corre en CI)
bash scripts/smoke-test.sh ./boot-preview
```
