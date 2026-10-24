# Theming en Limine — qué se puede personalizar y hasta dónde

Referencia de las **52 opciones globales** de `limine.conf` en Limine 12.x,
y del límite real: **el menú es texto plano**.

Todo lo de acá está verificado contra el código fuente upstream
(`Limine-Bootloader/Limine` @ `8ad1443`, ChangeLog 12.9.4 del 2026-10-19) y
contra `CONFIG.md` v12.x.

---

## El límite de fondo (leído del código)

El menú se dibuja en `common/menu.c`, función `print_tree()`:

```c
if (current_entry->sub) {
    print(current_entry->expanded ? "[-]" : "[+]");   // grupos
} else if (level) {
    print("──►");                                     // entrada anidada
} else {
    print("   ");                                     // entrada raiz
}
...
if (base_index + max_entries == selected_entry) {
    print("\e[7m");                                    // ← reverse video
}
print(" %s \e[27m\n", current_entry->name);            // ← el nombre
```

**Consecuencia**: la silueta del menú es un árbol de caracteres. No hay
primitivas de dibujo — nada de rectángulos, bordes redondeados, imágenes por
entrada ni eventos de puntero por entrada.

Por eso **cards, hover, widgets e iconos por entrada no existen** en Limine.
No es que falte una opción: es que el modelo de render es un terminal de
texto sobre framebuffer.

Si los querés, la única vía es **forkear y recompilar** — y Limine es
**BSD-2-Clause**, así que es legal (ver "El límite más allá" más abajo).

---

## Las 52 opciones globales

### Mischel

| Opción | Qué hace |
|---|---|
| `timeout` | Segundos antes de autobootear la entrada seleccionada. `0` = bootea al instante **sin mostrar el menú**; `no` = nunca autobootear; `9999` es el tope. Acepta decimales (`0.25`). En UEFI, la variable EFI `LoaderConfigTimeoutOneShot` tiene prioridad |
| `quiet` | Suprime toda la salida salvo panics y warnings importantes. El **menú sigue visible** |
| `serial` | Habilita I/O por puerto serie |
| `serial_baudrate` | Baudrate del serie. Default `115200` (también el máximo). Mínimo `50`, y debe dividir 115200 exacto |
| `global_dtb` | Usa este DTB en vez del que da el firmware (para Limine y para las entradas booted) |
| `default_entry` | Entrada que se selecciona al arrancar. Índice 1-based (`1`) o path (`OSes/Arch Linux`). Los paths usan `/` como separador; los caracteres literales `/`, `\` y `#` se escapan |
| `remember_last_entry` | Recuerda la última entrada booteada. Tiene prioridad sobre `default_entry`. Solo UEFI |
| `graphics` | `no` fuerza modo texto; si no, modo video |
| `verbose` | Información extra durante el boot |
| `terse` | Suprime los mensajes informativos ("Loading kernel", "Loading module"). El menú y los errores siguen |
| `randomise_memory` | Rellena la RAM con basura al arrancar (detección de bugs / seguridad). Ralentiza el boot |
| `hash_mismatch_panic` | `no` no hace panic si un fichero no matchea el hash: avisa y espera una tecla |
| `measured_boot` | Opt-in a measured boot. Forzado a `yes` con Secure Boot + hash enrolado |
| `firmware_logo` | `yes` restaura el logo del firmware OEM al pasar el control al SO. Solo UEFI |
| `keyboard_layout` | Remapea teclas: `dvorak` o `azerty`. Asume que el firmware resuelve US-QWERTY |
| `mouse` | `no` desactiva el mouse. Con `yes`: el puntero sigue la selección, click bootea (o expande un directorio), la rueda scrollea. Usar el mouse **detiene** el countdown, como cualquier tecla |

### Fondo

| Opción | Qué hace |
|---|---|
| `wallpaper` | Path a la imagen de fondo. **BMP, PNG, JPEG y QOI**. Se puede repetir: Limine **sortea una al azar** en cada arranque |
| `wallpaper_style` | `tiled`, `centered` o `stretched`. Default `stretched` |
| `backdrop` | Color `RRGGBB` de las partes de la pantalla no cubiertas por la imagen, cuando el estilo es `centered` |

### Interfaz

| Opción | Qué hace |
|---|---|
| `interface_resolution` | Resolución de la interfaz de Limine (menú, editor, consola) en `<width>x<height>`. **Solo afecta al bootloader**, no al SO booted. Si no está disponible, Limine elige otra |
| `interface_rotation` | `0`, `90`, `180` o `270` |
| `interface_branding` | Texto que se muestra arriba de la interfaz |
| `interface_branding_colour` | `RRGGBB` del branding. Default `00aaaa` |
| `interface_help_hidden` | Oculta el texto de ayuda con los atajos de teclado |
| `interface_help_colour` | `RRGGBB` de las strings de ayuda. Default `00aa00` |
| `interface_help_colour_bright` | `RRGGBB` del dígito del countdown. Si no se pone, se deriva de `interface_help_colour` sumando `0x55` a cada canal (saturando en `0xff`) |

### Terminal gráfico

Ignorado en modo texto.

| Opción | Qué hace |
|---|---|
| `term_font` | Path a una font **CP437** (256 glifos bitmap consecutivos, 1 byte por fila, 16 por glifo). Colección de referencia: `viler-int10h/vga-text-mode-fonts` |
| `term_font_size` | Tamaño de cada glifo en dots. **El ancho debe ser 8** (el primer valor del par, el `8` de `8x16`); cualquier otro ancho se rechaza y se usa la font default. Default `8x16` |
| `term_font_scale` | Escala X e Y. `2x2` = doble tamaño (útil en high-DPI), `2x1` = solo el doble de ancho, `4x2` = para una font angosta en pantalla alta. Cada factor entre 1 y 8. Si no se pone, sigue a la resolución: `1x1` hasta 2560×1440, `2x2` de ahí en adelante, `4x4` desde 5120×2880 |
| `term_font_spacing` | Espaciado horizontal en píxeles entre glifos. Default 1. Acepta 0; más de 8 se ignora |
| `term_palette` | 8 colores `RRGGBB` separados por **`;`**: negro, rojo, verde, marrón, azul, magenta, cyan, gris |
| `term_palette_bright` | Los 8 bright: gris oscuro, rojo, verde, amarillo, azul, magenta, cyan, blanco |
| `term_background` | Fondo del texto en `TTRRGGBB`. **TT es transparencia** (00 = totalmente transparente). Default `00000000`, o `80000000` con wallpaper |
| `term_foreground` | Color del texto, `RRGGBB` |
| `term_background_bright` | Fondo del texto en modo bright |
| `term_foreground_bright` | Color del texto en modo bright |
| `term_margin` | Margen en píxeles alrededor del terminal. Default 0, o **64** cuando hay wallpaper |
| `term_margin_gradient` | Grosor en píxeles del degradado alrededor del terminal, limitado por `term_margin`. Default 0, o **4** con wallpaper |

### Editor

| Opción | Qué hace |
|---|---|
| `editor_enabled` | `no` desactiva el editor embebido (tecla `E`). Default `yes`, salvo hash enrolado. Desactivado con Secure Boot + hash |
| `editor_highlighting` | `no` desactiva el resaltado de sintaxis |
| `editor_validation` | `no` desactiva el aviso de opciones inválidas o errores de sintaxis |

### Por entrada (opciones locales)

| Opción | Qué hace |
|---|---|
| `comment` | Texto que se muestra en el menú cuando la entrada está seleccionada |
| `protocol` | `linux`, `limine`, `multiboot` (o `multiboot1`), `multiboot2`, `efi`, `efi_boot_entry` o `bios` |
| `cmdline` | Línea de comandos del kernel. `kernel_cmdline` es alias |
| `if_fw_type` | Oculta la entrada si el firmware no es `BIOS` o `UEFI` |
| `if_arch` | Oculta la entrada si la CPU no está en la lista de arquitecturas permitidas |
| `if_loader_arch` | Oculta la entrada si la arquitectura del propio binario de Limine no está en la lista |

Y por protocolo: `path`, `module_path`, `module_string`, `module_cmdline`,
`resolution`, `textmode`, `entry`, `drive`, `partition`, `mbr_id`.

---

## Lo que se puede personalizar, de verdad

### Fondo

```ini
wallpaper: boot():/splash.png
wallpaper_style: centered
backdrop: 191724
```

Formatos: PNG, BMP, JPEG, **QOI**. QOI es interesante: mucho más liviano que
PNG y decodifica más rápido — ideal para un bootloader.

**Wallpapers aleatorios**: repetí la opción y Limine sortea una en cada
arranque. Varianza visual gratis.

### El "card" — el único efecto así

```ini
term_margin: 120
term_margin_gradient: 24
term_background: B01E1E2E
```

`term_background` lleva **alpha en el primer byte** (`TT`): `B0` ≈ 69% de
opacidad. `term_margin` crea el panel, `term_margin_gradient` difumina el
borde hacia el wallpaper.

Es el resultado más parecido a una "card" que Limine puede dar sin
recompilar. **No** es lo mismo que un widget: el panel es único, no uno por
entrada.

### La paleta

```ini
term_palette: 191724;eb6f92;f6c177;9ccfd8;31748f;c4a7e7;ebbcba;6e6a86
term_palette_bright: 3a3a52;eb6f92;f6c177;9ccfd8;31748f;c4a7e7;ebbcba;e0def4
```

**Separador: `;`** (punto y coma). Con `,` no parsea. Son 8 + 8 = 16 colores,
el set completo de la terminal.

### La tipografía

```ini
term_font: boot():/font.F16
term_font_size: 8x16
term_font_scale: 1
term_font_spacing: 1
```

**La font tiene que ser CP437 bitmap**, no TTF/OTF. El formato `.F16` es el
de Limine (16 bytes por glifo, 256 glifos). Se convierte con
`limine-font-converter` (AUR).

El `term_font_scale` automático depende de la resolución: a 1920×1080 es
`1x1` (o `1`, que es lo mismo) es el valor que usa este repo: a 1920x1080
`2x2` duplica ancho y alto y el menú se come la pantalla.

### La interfaz

```ini
interface_resolution: 1920x1080
interface_branding: b3low-z3r0
interface_branding_colour: c4a7e7
interface_help_colour: 9ccfd8
interface_help_hidden: no
```

---

## Blur y "máxima calidad"

### Más calidad: Upscayl

Limine no tiene opción de calidad de imagen, pero se puede **mejorar la
imagen antes** con IA. Para eso está
[Upscayl](https://github.com/upscayl/upscayl), la app de escritorio que trae
los modelos de Real-ESRGAN (y ESRGAN anime).

**El flujo**: se mejora la imagen fuera de este repo y se mete al preview.

```bash
# 1. en Upscayl: escala x4 con el modelo "RealESRGAN Anime" y guardá
# 2. la imagen entra al preview
limine-preview add ~/Pictures/foto-4x.png
```

**El truco**: upscalear x4 y después **bajar** a la resolución final con
Lanczos. El resultado tiene más detalle *percibido* que el original, sin
inventar textura nueva.

Medido con el Laplaciano (nitidez) sobre el wallpaper de ejemplo:

| Versión | Nitidez |
|---|---|
| PNG original | 4.617 |
| frame extraído del MP4 | 4.617 |
| **Real-ESRGAN x4 + downscale** | **7.656** (+66%) |

> Este repo **no** baja modelos ni depende de Real-ESRGAN: el upscale es un
> paso externo, con la app que ya usás.

### Limine no puede difuminar

No hay opción de blur, ni de sharpen, ni de filtro. El wallpaper se decodifica
y se dibuja **tal cual**. Y `term_margin_gradient` **no es un blur**: es un velo
del color de `term_background` mezclado en los bordes del área del texto — se
ve mal, y no es lo que uno quiere.

El desenfoque se hace **antes**, al preparar el fichero — y para eso está
`scripts/make-wallpaper.sh`:

```bash
# wallpaper difuminado y velado, a la resolución exacta de la interfaz
scripts/make-wallpaper.sh ~/Pictures/foto.png \
  --blur 0x30 --dim 0.4 --res 1920x1080

# lo mismo, con el velo en el color de fondo del tema
scripts/make-wallpaper.sh ~/Pictures/foto.png \
  --blur 0x30 --dim 0.45 --overlay "#191724" --res 1920x1080
```

| Opción | Qué hace |
|---|---|
| `--blur 0x30` | desenfoque gaussiano (0xRADIO) |
| `--dim 0.4` | velo uniforme encima (0 = nada, 1 = negro). Baja el brillo **sin** aplastar los negros, que es lo que hace `-evaluate multiply` |
| `--overlay COLOR` | el velo en otro color en vez de negro |
| `--res 1920x1080` | escala a esa resolución con Lanczos y recorta al aspecto |
| `--format qoi` | QOI: lossless y se decodifica más rápido que PNG |

### Máxima calidad

| Regla | Por qué |
|---|---|
| **El wallpaper debe medir al menos lo que la `interface_resolution`** | si es más chico, Limine lo escala (bilinear) y se ve borroso. 1:1 = nítido |
| **PNG o QOI, ambos lossless** | JPEG tiene pérdida y a pantalla completa se nota el bloqueado |
| **QOI si el tiempo de boot importa** | lossless como PNG y mucho más rápido de decodificar |
| **Sin sharpen si hay blur** | el blur posterior deshace el sharpen |

Limine **no tiene ninguna opción de calidad de imagen**: no hay un
`image_quality` ni un `jpeg_quality`. La calidad es la del fichero que le des.

### ⚠️ El detalle que más confunde: el alpha de `term_background` va al revés

`term_background` no es solo "el fondo del texto": es el color con el que se
**mezcla** el wallpaper. Y el byte alto (`TT`) **no funciona como "transparencia"
dice la doc** — funciona al revés.

El blend en `common/lib/gterm.c` es:

```c
#define A(rgb) (uint8_t)(rgb >> 24)
unsigned alpha    = 255 - A(fg);      // ← el byte alto, invertido
unsigned inv_alpha = A(fg) + 1;
r = (alpha * R(fg) + inv_alpha * R(bg)) / 256;   // fg = term_background, bg = wallpaper
```

| `TT` | Qué pasa | Resultado |
|---|---|---|
| `00` | `alpha=255` → manda el **color** | **recuadro negro** sobre el wallpaper |
| `40` | mezcla 75/25 | el wallpaper apenas se insinúa |
| `80` | mezcla 50/50 | el wallpaper se ve a medias ← el look "card" |
| `FF` | `alpha=0` → pasa el **wallpaper** | **wallpaper limpio**, sin panel |

O sea: **cuanto más alto el byte, más wallpaper se ve**. Para el wallpaper
limpio (sin panel, sin velo) es `FF`; para el panel translúcido es `80`.

Medido en el preview real, con el mismo wallpaper:

| `term_background` | Lo que se ve |
|---|---|
| `00191924` | solo el marco exterior con wallpaper; el centro, negro |
| `40191924` | el centro empieza a transparentarse |
| `80191924` | wallpaper a medias, panel translúcido |
| `FF191924` | **wallpaper completo y limpio** |

### El margen también importa

`term_margin` (default **64 px** con wallpaper) define el área donde se dibuja
el texto; `term_margin_gradient` (default **4**) el difuminado del borde. Con
`term_margin: 0` el wallpaper desaparece de la zona del texto — no es un
problema del alpha, es que el área del texto se dibuja siempre con su propio
fondo.

| Config | Resultado |
|---|---|
| `term_background: FF191924` + `term_margin: 0` + `term_margin_gradient: 0` | **wallpaper limpio, sin marco ni velo** ← con el blur pre-renderizado en la imagen |
| `term_background: FF191924` + `term_margin: 64` | wallpaper limpio con un marco sutil |
| `term_background: 80191924` + `term_margin: 120` | panel translúcido con borde difuminado ← el look "card" |

**La combinación recomendada** cuando el blur va pre-renderizado: margin 0,
gradient 0 y alpha `FF`. El wallpaper se ve **tal cual**, y el texto se lee
porque el desenfoque ya está en la imagen.

## Lo que NO se puede

| Pedido | ¿Se puede? | Por qué / alternativa |
|---|---|---|
| **Menú como cards** | ❌ | texto plano; el panel de `term_margin` es lo más cerca |
| **Hover highlight** | ❌ | el mouse mueve la *selección*, no hay eventos por entrada |
| **Iconos por entrada** | ❌ | el nombre es texto plano (se puede con un carácter Unicode) |
| **Animaciones** | ❌ | el framebuffer se dibuja una vez |
| **Video de fondo** | ❌ | → Plymouth (la fase post-Limine) |
| **Blur / filtros** | ❌ | solo color plano con alpha |
| **Múltiples fuentes** | ❌ | una font global |
| **TTF/OTF directo** | ❌ | hay que convertirla a CP437 bitmap |
| **Cursor custom** | ❌ | solo el default |
| **Sonido** | ❌ | → Plymouth (con script theme) |

### El límite más allá: recompilar

Como Limine es **BSD-2-Clause**, podés forkear `common/menu.c` y:

- reemplazar `print(" %s ", name)` por un dibujo de rectángulo con color de
  fondo (Skia/framebuffer propio)
- enganchar el hover con `mouse_get_state()` (el driver ya está: `common/menu.c`
  usa `mouse_get_state()` y `mouse_render_pointer_overlay()`)
- dibujar un borde redondeado por entrada

**El costo**: es C freestanding sobre framebuffer, y **perdés el soporte de
Secure Boot por hash** si no re-enrolás la config. Además tenés que
reconstruir el binario UEFI y reinstalarlo.

### La alternativa: Plymouth

Si lo que querés es **animación, video o transiciones**, eso vive en Plymouth,
que corre **después** de Limine y antes del login. Plymouth sí tiene themes
con scripts, secuencias de PNG y video. La división natural:

| | Limine | Plymouth |
|---|---|---|
| Momento | antes del kernel | después del kernel, antes del login |
| Qué controla | qué sistema booteo | **qué veo mientras booteo** |
| Animaciones | ❌ | ✅ |
| Video | ❌ | ✅ |
| Widgets | ❌ | ✅ (scripts) |

---

## Cómo se ve el menú (anatomía del render)

Por `common/menu.c`, el menú se arma así:

```
(línea en blanco)
        b3low-z3r0                          ← interface_branding, centrado
(línea en blanco)
(línea en blanco)
     ESC Discard and Exit    F10/CTRL+X Boot   ← help, centrado
(línea en blanco)
(línea en blanco)
┌────────────────────────────────────┐      ← la caja, con el titulo centrado
│                                    │          como ┤titulo├
│  ├ [+] CachyOS                     │      ← grupo expandido
│  │  └ linux-cachyos                │      ← entrada anidada (──►)
│  └ Windows 11                      │      ← entrada raiz
│                                    │
│  Btrfs + Snapper                   │      ← comment de la entrada
└────────────────────────────────────┘
```

- La **selección** va en reverse video (`\e[7m`): fondo = foreground, texto = background.
- El **árbol** usa `├`, `│`, `└`, `[+]`/`[-]` para grupos y `──►` para anidadas.
- Una entrada se **anida** con `/` repetidos: `/CachyOS`, `//linux-cachyos`.
- Un `+` después de los `/` marca el grupo como **expandido**: `/+CachyOS`.
- El **comment** de la entrada aparece abajo de todo.

---

## Temas de la comunidad

Limine no tiene un ecosistema de themes como GRUB, pero hay configs
prontas que son **wallpaper + palette**:

| Tema | Qué es |
|---|---|
| [catppuccin/limine](https://github.com/catppuccin/limine) | Las 4 variantes de Catppuccin (Latte, Frappé, Macchiato, Mocha). MIT. Son bloques de config para pegar al principio del `limine.conf` |
| [fallout-limine-theme](https://github.com/) | Tema retro. Manual, para Limine v12 |

Ninguno trae un preview interactivo: son bloques de texto que se pegan en la
config.

---

## Referencias

- [CONFIG.md upstream v12.x](https://github.com/Limine-Bootloader/Limine/blob/v12.x/CONFIG.md) — la referencia de las 52 opciones
- [ChangeLog upstream](https://github.com/Limine-Bootloader/Limine/blob/v12.x/ChangeLog)
- [USAGE.md upstream](https://github.com/Limine-Bootloader/Limine/blob/v12.x/USAGE.md) — Secure Boot, measured boot, ISO
- `common/menu.c` — `print_tree()`, el render del árbol
- `common/lib/gterm.c` — `TERM_MARGIN`, `TERM_MARGIN_GRADIENT`, la carga de la font
