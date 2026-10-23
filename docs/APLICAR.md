# Aplicar el tema al Limine real

El preview es para **iterar**. Acá está el flujo para el cambio de verdad,
con rollback.

---

## ⚠️ Lo primero: dónde se edita

Limine tiene **dos** archivos de configuración y solo uno se edita a mano:

| Archivo | Qué es | ¿Se edita? |
|---|---|---|
| `/etc/default/limine` | la **fuente**: cmdline del kernel, boot order, y lo que agreguemos de theming | ✅ **acá** |
| `/boot/limine.conf` | la config **compilada** que Limine lee en el boot | ❌ se regenera |

`/boot/limine.conf` lo genera `limine-mkinitcpio` desde `/etc/default/limine`.
**Si lo editás a mano, los cambios se pierden en el próximo `mkinitcpio`.**

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
cp ~/.config/limine-preview/limine.conf ~/.config/limine-preview/limine.conf.mine

# 2. Poné el bloque de theming (abajo está el de Rosé Pine)

# 3. Mirá cómo queda
limine-preview --shot /tmp/antes.png

# 4. Iterá hasta que te guste
```

El `--shot` captura el framebuffer real. Cuando te guste, aplicá.

---

## Paso 1 — El wallpaper

```bash
# El wallpaper va en /boot (la partición FAT que Limine lee)
sudo cp examples/wallpaper/rei.png /boot/splash.png

# Verificar
ls -la /boot/splash.png
identify /boot/splash.png    # 1920x1080
```

**Formatos aceptados**: PNG, BMP, JPEG y **QOI**. QOI es más liviano y
decodifica más rápido — ideal para un bootloader.

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
`/etc/default/limine`:

```bash
sudo nano /etc/default/limine
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
term_font_scale: 2x2

# Interfaz
interface_resolution: 1920x1080
interface_branding: b3low-z3r0
interface_branding_colour: c4a7e7
interface_help_colour: 9ccfd8
interface_help_colour_bright: e0def4
```

> **La fuente**: si no tenés un `.F16`, **sacá las 4 líneas de `term_font`** y
> dejá solo `term_font_scale: 2x2`. Limine usa su font built-in (CP437 8x16) y
> el scale la agranda. Para una font custom:
>
> ```bash
> paru -S limine-font-converter
> limine-font-converter -i MiFuente.ttf -o /boot/font.F16 -f 8x16
> ```
>
> El archivo tiene que ser CP437: 256 glifos, 1 byte por fila, 16 por glifo.

---

## Paso 3 — Regenerar la config

```bash
sudo limine-mkinitcpio
```

Verificar que el bloque llegó al `.conf` compilado:

```bash
sudo grep -E "^wallpaper|^term_|^interface_" /boot/limine.conf
```

Deberías ver las líneas del bloque de theming. **Si no aparecen, el bloque
no estaba en `/etc/default/limine`** (o estaba después de una entrada).

---

## Paso 4 — Reiniciar

```bash
sudo reboot
```

---

## Rollback

Si algo quedó mal:

```bash
# 1. Sacar el bloque de theming de /etc/default/limine
sudo nano /etc/default/limine

# 2. Regenerar
sudo limine-mkinitcpio

# 3. Reiniciar
sudo reboot
```

O, más rápido, sin tocar la config:

```bash
# Quitar solo el wallpaper
sudo rm /boot/splash.png
sudo limine-mkinitcpio
sudo reboot
```

**Antes de todo, un backup** ( Limine **no tiene** backup automático de
`/etc/default/limine`):

```bash
sudo cp /etc/default/limine /etc/default/limine.bak
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

A propósito, `limine-preview` **solo lee** `/boot/limine.conf` y **nunca**
escribe en `/boot`. El preview es para decidir; el aplicar es un acto
deliberado con su propio paso de rollback.

Si en algún momento querés un modo `--apply`, que escriba el bloque de
theming en `/etc/default/limine` y regenere el `.conf`, se puede agregar —
pero con backup automático y confirmación interactiva.
