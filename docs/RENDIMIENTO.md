# Rendimiento: los scripts y la pregunta de Bun

Dos cosas distintas que se mezclan fácil:

1. **El rendimiento del pipeline** (qué tarda en salir un wallpaper o una
   captura) — acá había margen real y se arregló.
2. **El lenguaje de los scripts** — acá la respuesta es que no cambia nada,
   y los números lo muestran.

---

## 1. El pipeline, antes y después

Medido en esta máquina (Intel UHD 12 núcleos, QEMU con KVM habilitado).

| Operación | Antes | Ahora | Ganancia |
|---|---|---|---|
| `limine-preview shot` | 15.6 s | **2.8 - 3.1 s** | **5x** |
| `make-wallpaper --blur 0x30 --res 1920x1080` | 0.90 s | **0.32 s** | **2.8x** |
| `make-wallpaper` sin blur (sharpen) | 0.85 s | **0.36 s** | 2.4x |
| `prepare-wallpapers` (las 2 versiones) | ~1.9 s | **0.90 s** | 2.1x |

### Qué se cambió

**a) El `sleep 15` fijo.** Era el 96% del tiempo de una captura. Ahora el
script consulta el monitor de QEMU cada 300 ms.

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

---

## 2. ¿Se puede pasar todo a TypeScript + Bun?

**Sí, se puede. No conviene.** Estos son los números.

### Arranque del runtime

| Runtime | Arranque |
|---|---|
| `bash -c true` | **2.2 ms** |
| `python3 -c pass` | 52 ms |
| `bun -e 'process.exit(0)'` | **117 ms** |

**Bun tarda ~53x más en arrancar que bash.** Cada invocación de
`limine-preview` o `make-wallpaper` pagaría ese peaje antes de hacer nada.

### El mismo script, en los dos lenguajes

Un script que hace lo mismo: parsea argumentos, lee un conf, reescribe un
bloque con regex y copia un archivo.

| Versión | Por invocación (20 iteraciones) |
|---|---|
| bash | **18.1 ms** |
| TypeScript + Bun | 23.6 ms |

**Bun queda 1.3x más lento** para el tipo de trabajo que hacen estos
scripts.

### Por qué

Los scripts son **orquestadores**: casi todo su tiempo lo pasan esperando a
binarios externos.

| Pieza del preview | Tiempo | ¿Se podría reescribir en TS? |
|---|---|---|
| `qemu-system-x86_64` | 2-3 s (con KVM) | No — es el emulador |
| `xorriso` (construir ISO) | 105 ms | No — hay librerías, pero no ganás |
| `vips` (imagen) | 88 ms | Tal vez, pero vips ya usa SIMD |
| copiar 4 binarios | 6 ms | Sí, pero es copiar bytes |
| parsear args | <1 ms | Sí, y no importa |

El 90% del tiempo no es del lenguaje. Reescribir en Bun mueve el 10%
restante a un runtime que arranca 53x más lento.

### Dónde Bun sí gana

| Caso | Bash | Bun |
|---|---|---|
| Procesar un JSON grande | jq + shell | **Bun gana** |
| HTTP, APIs, concurrencia | curl + xargs | **Bun gana** |
| Lógica de negocio compleja | frágil | **Bun gana** |
| Scripts de 30 líneas que llaman binarios | **Bash gana** | — |
| Arranque en cada invocación | **Bash gana** | — |

La regla práctica: **si el script es un pegamento entre binarios, bash. Si
tiene lógica propia (parseo, validación, red), TypeScript.**

`limine-preview` es 100% pegamento. Por eso se queda en bash.

### Si algún día se migrara

Sería por mantenibilidad, no por velocidad:

| Ventaja de Bun | Costo |
|---|---|
| Tipos y autocompletado | +117 ms por invocación |
| Tests unitarios reales | Bun como dependencia |
| Manejo de errores con stack traces | Recompilar al distribuir (`bun build --compile`) |
| Parsing de argumentos robusto | 2 runtimes en el repo (bash para setup, TS para el resto) |

Lo único que **sí** valdría la pena mover a Bun/TS sería el tratamiento de
imágenes — pero para eso ya está vips, que es C con SIMD y le gana a
cualquier cosa que se escriba arriba.

---

## Cómo reproducir las mediciones

```bash
# Arranque de los runtimes
time bash -c true
time bun -e 'process.exit(0)'

# El pipeline de imagen
time make-wallpaper IMAGEN -o /tmp/x.png --res 1920x1080 --blur 0x30
time vips gaussblur IMAGEN /tmp/x.v 30

# La captura
time limine-preview shot /tmp/x.png --timeout 25

# Equivalencia de las dos salidas
magick compare -metric RMSE salida-magick.png salida-vips.png null:
```
