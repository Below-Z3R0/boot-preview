# Aplicar el tema al Limine real

El preview es para **iterar**. Acá está el flujo para el cambio de verdad,
con rollback.

---

## La forma corta: `boot-preview loader apply`

```bash
boot-preview loader apply             # wallpaper + config visual
boot-preview loader apply config      # solo la config visual
boot-preview loader apply wallpaper   # solo el wallpaper
```

Qué hace, en orden:

1. Copia el wallpaper activo (`~/.config/boot-preview/loader/wallpaper/wallpaper.png`)
   a `/boot/splash.png`.
2. Hace un **backup** en `/boot/limine.conf.bak-YYYYMMDD-HHMMSS` (conserva los
   5 más recientes).
3. Reemplaza el bloque de theming en `/boot/limine.conf`, **antes de la primera
   entrada**. Las 174 entradas del menú no se tocan.
4. Traduce el path del wallpaper: `boot():/wallpaper.png` (el del ISO) pasa a
   `boot():/splash.png` (el del sistema real).

Es **idempotente**: aplicarlo dos veces deja el mismo archivo. Está verificado
con tests (ver el README) — desde un conf sin theming, `apply` lo deja **byte
a byte idéntico** al original.

**Si una actualización rompe el conf:**

```bash
boot-preview loader apply            # si solo se perdió el theming
```

Si el conf desapareció entero, `apply` falla con un mensaje claro (no puede
reconstruir las entradas del menú) y hay que restaurar de un backup:

```bash
sudo ls -1t /boot/limine.conf.bak-*
sudo cp /boot/limine.conf.bak-XXXX /boot/limine.conf
boot-preview loader apply
```

---

Lo que sigue es el detalle de **por qué** funciona así, y cómo hacerlo a mano
si preferís no usar el comando.

## Las tres configs (antes de tocar nada)

| # | Path | Qué es | ¿Se edita a mano? |
|---|---|---|---|
| **1** | `~/.config/boot-preview/loader/limine.conf` | la config de prueba del preview | ✅ sí, es la que iterás |
| **2** | `/boot/limine.conf` | la config que Limine lee en el boot (100 KB) | ✅ **acá va el theming** |
| **3** | `/etc/default/limine` | `ESP_PATH`, `KERNEL_CMDLINE` y `BOOT_ORDER` | ✅ pero **no** el theming |

El preview usa la **#1** (o la **#2** con `--real`). Para el cambio de verdad
se edita la **#2**.

## ⚠️ Lo primero: dónde se edita (verificado en este sistema)

**El theming va en `/boot/limine.conf`, directo.** No en
`/etc/default/limine`.

| Lo que se suele creer | Realidad (verificada) |
|---|---|
| "`/boot/limine.conf` se regenera desde `/etc/default/limine`" | ❌ no. `/etc/default/limine` solo aporta `ESP_PATH`, `KERNEL_CMDLINE` y `BOOT_ORDER` |
| "si lo editás a mano se pierde" | ❌ no. Las herramientas **editan** el conf, no lo regeneran |
| "hay que correr `limine-mkinitcpio`" | ❌ no hace falta para el theming |

**Cómo se comprobó**: se agregó el bloque de theming a `/boot/limine.conf` y
después se corrió `sudo limine-snapper-sync` (que es lo que reescribe el conf
cuando se crea un snapshot). El bloque **sobrevivió** intacto, y el `diff` de
las entradas contra el backup dio **cero diferencias**.

| Herramienta | Qué hace con el conf | ¿Preserva el theming? |
|---|---|---|
| `limine-entry-tool` (`--add-kernel`, `--remove-kernel`) | edita las entradas | ✅ |
| `limine-snapper-sync` | agrega/quita la sección Snapshots | ✅ |
| `limine-mkinitcpio` | llama a `limine-entry-tool` | ✅ |

**El riesgo real** no es la edición manual: es un `limine-update` o un
`limine-install` que **sí** regeneran el conf desde cero. Si eso pasa, el
theming se pierde y hay que volver a aplicar el bloque.

### El checksum del config

`/etc/limine-entry-tool.conf` tiene `ENABLE_VERIFICATION=yes`, pero eso
verifica los **ficheros del kernel** (los `module_path` con `#hash`), no el
config.

El checksum del **config** es otra cosa, y **no está enrolado** en este
sistema: `ENABLE_ENROLL_LIMINE_CONFIG` está comentado en
`/etc/limine-entry-tool.conf` (y también en `/etc/default/limine`). Por eso
editar el conf no hace fallar el boot.

Si algún día se habilita (`ENABLE_ENROLL_LIMINE_CONFIG=yes`), hay que
re-enrolar después de cada cambio:

```bash
sudo limine-enroll-config
```

```bash
cat /etc/mkinitcpio.conf.d/10-limine-snapper-sync.conf 2>/dev/null
# o
grep -r "limine" /etc/mkinitcpio.conf.d/ 2>/dev/null
```

En este sistema, además, `/boot/limine.conf` lo **mantienen** dos
herramientas que lo regeneran automáticamente:

- `limine-entry-tool` — genera las entradas de los kernels
- `limine-snapper-sync` — genera el submenú de snapshots

Por eso el theming tiene que ir en `/etc/default/limine`: las dos
herramientas reescriben `/boot/limine.conf` desde cero y borran lo que
hayas puesto a mano.

---

## Antes de tocar nada: el preview

```bash
# 1. Prepará la config con el theming que querés
cp ~/.config/boot-preview/loader/limine.conf ~/.config/boot-preview/loader/limine.conf.mine

# 2. Poné el bloque de theming (abajo está el de Rosé Pine)

# 3. Mirá cómo queda
boot-preview loader shot /tmp/antes.png

# 4. Iterá hasta que te guste
```

El `--shot` captura el framebuffer real. Cuando te guste, aplicá.

---

## Paso 1 — El wallpaper

```bash
# El wallpaper va en /boot (la partición FAT que Limine lee)
sudo cp ~/.config/boot-preview/loader/wallpaper/wallpaper.png /boot/splash.png

# Verificar
ls -la /boot/splash.png
identify /boot/splash.png    # 1920x1080
```

**Formatos aceptados**: PNG, BMP, JPEG y **QOI**. QOI es más liviano y
decodifica más rápido — ideal para un bootloader.

**Para máxima calidad**: el wallpaper tiene que medir al menos lo que la
`interface_resolution` (si es más chico, Limine lo escala y se ve borroso), y
en PNG o QOI, que son lossless.

**Para el blur**: Limine no puede difuminar — la imagen se dibuja tal cual.
Se difumina antes, con el generador:

```bash
scripts/make-wallpaper.sh examples/wallpaper/rei.png \
  -o /tmp/splash.png --blur 0x30 --dim 0.4 --res 1920x1080
sudo cp /tmp/splash.png /boot/splash.png
```

Si querés QOI:
```bash
magick /boot/splash.png -define qoi:codec=1 /boot/splash.qoi
```

> **Ojo con el doble recorte**: si el wallpaper ya tiene el logo o texto
> recortado, no lo recortes otra vez. El wallpaper de Limine se ve **tal
> cual**, a pantalla completa.

---

## Paso 2 — El bloque de theming

`examples/theming-rosepine.conf` tiene el bloque. Pegalo en
**`/boot/limine.conf`**, después de las opciones globales que ya están y
**antes de la primera entrada** (una línea que empieza por `/`):

```bash
sudo nano /boot/limine.conf
```

```ini
# ── Limine theming (b3l0wz3r0-os) ──
# Wallpaper
wallpaper: boot():/splash.png
wallpaper_style: centered
backdrop: 191724

# El "card" translucido
term_margin: 120
term_margin_gradient: 24
term_background: B01E1E2E
term_foreground: e0def4
term_palette: 191724;eb6f92;f6c177;9ccfd8;31748f;c4a7e7;ebbcba;6e6a86
term_palette_bright: 3a3a52;eb6f92;f6c177;9ccfd8;31748f;c4a7e7;ebbcba;e0def4

# Fuente (CP437 bitmap, NO TTF)
term_font: boot():/font.F16
term_font_size: 8x16
term_font_scale: 1

# Interfaz
interface_resolution: 1920x1080
interface_branding: b3low-z3r0
interface_branding_colour: c4a7e7
interface_help_colour: 9ccfd8
interface_help_colour_bright: e0def4
```

> **La fuente**: si no tenés un `.F16`, **sacá las 4 líneas de `term_font`** y
> dejá solo `term_font_scale: 1`. Limine usa su font built-in (CP437 8x16) y
> el scale la agranda. Para una font custom:
>
> ```bash
> paru -S limine-font-converter
> limine-font-converter -i MiFuente.ttf -o /boot/font.F16 -f 8x16
> ```
>
> El archivo tiene que ser CP437: 256 glifos, 1 byte por fila, 16 por glifo.

---

## Paso 3 — Verificar

```bash
sudo grep -E "^wallpaper|^term_|^interface_" /boot/limine.conf
ls -la /boot/splash.png
```

Deberías ver las líneas del bloque de theming y el wallpaper en `/boot`.

**No hace falta `limine-mkinitcpio`**: el conf ya está actualizado y las
herramientas lo preservan.

---

## Paso 4 — Reiniciar

```bash
sudo reboot
```

---

## Rollback

Si algo quedó mal:

```bash
# 1. Sacar el bloque de theming de /boot/limine.conf
sudo nano /boot/limine.conf

# 2. Reiniciar
sudo reboot
```

O, más rápido, restaurar el backup:

```bash
sudo cp /boot/limine.conf.pre-theming /boot/limine.conf
sudo reboot
```

O quitar solo el wallpaper:

```bash
sudo rm /boot/splash.png
sudo reboot
```

**Antes de todo, un backup**:

```bash
sudo cp /boot/limine.conf /boot/limine.conf.pre-theming
```

---

## El plan B: si Limine no arranca

Si la config quedó rota y la máquina no bootea:

1. **Reiniciá** y entrá al firmware: en Limine, la tecla **F1** lleva al
   setup del firmware. (Con `interface_help_hidden` el texto de ayuda
   desaparece, pero la tecla funciona igual.)
2. Desde el firmware, elegí el **USB/ESP** y arrancá el Limine con la config
   de UEFI por defecto, o entrá al **UEFI Shell** y montá la partición FAT
   para arreglar `limine.conf`.
3. En el peor caso: desde la UEFI Shell, `fs0:` y editá `limine.conf`
   borrando las líneas del theming.

**Por eso el preview importa**: en la VM, si rompés la config, no pasa nada.
Y el theming no puede romper el arranque de todas formas — si una opción es
inválida, Limine la ignora (o avisa) pero **el bootloader sigue booting**.

---

## Verificar que quedó

```bash
# El bloque está en la fuente
grep -E "^wallpaper|^term_|^interface_" /etc/default/limine

# El bloque llegó al compilado
sudo grep -E "^wallpaper|^term_|^interface_" /boot/limine.conf

# El wallpaper está donde Limine lo busca
ls -la /boot/splash.png
```

Y después del reboot, la diferencia se ve a simple vista: el menú deja de
ser el default de Limine y pasa a tener el wallpaper con el panel
translúcido.

---

## Nota: por qué el preview no aplica los cambios

A propósito, `boot-preview loader` **solo lee** `/boot/limine.conf` y **nunca**
escribe en `/boot`. El preview es para decidir; el aplicar es un acto
deliberado con su propio paso de rollback.

Si en algún momento querés un modo `--apply`, que escriba el bloque de
theming en `/etc/default/limine` y regenere el `.conf`, se puede agregar —
pero con backup automático y confirmación interactiva.
