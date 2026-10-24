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
plymouth-preview                 # ventana con el splash
plymouth-preview shot [archivo]  # captura el splash
plymouth-preview themes          # lista los temas
plymouth-preview theme <nombre>  # cambia el tema DEL PREVIEW
plymouth-preview build           # regenera el initramfs (cache)
```

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
