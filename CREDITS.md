# Créditos y licencias

Este repo es **software original** (MIT). Pero trae y se apoya en componentes
de terceros, y cada uno tiene sus propios términos. Esta es la auditoría
completa, leída de los ficheros de licencia reales.

---

## Este repo

| Componente | Licencia | Dónde se lee |
|---|---|---|
| `limine-preview.sh`, `install.sh`, los docs | **MIT** | este repo |
| `examples/wallpaper/rosepine.png` | **original** — generado con ImageMagick para este repo | el gradiente no viene de ningún sitio |

---

## Dependencias de sistema (no se redistribuyen)

Se instalan con `pacman`, no se incluyen en el repo.

| Paquete | Licencia | Para qué |
|---|---|---|
| `qemu-desktop` | GPL-2.0+ | el emulador |
| `libisoburn` (xorriso) | GPL-2.0+ | construir el ISO |
| `edk2-ovmf` | BSD-2-Clause (PicoEFI: 0BSD) | el firmware UEFI |
| `imagemagick` | ImageMagick License (Apache-2.0 compatible) | convertir la captura |
| `socat` | GPL-2.0+ | hablar con el monitor de QEMU |

Ninguna se distribuye acá: se instalan desde los repos de Arch.

---

## Limine

| Componente | Licencia | Dónde se lee |
|---|---|---|
| El bootloader (`/usr/share/limine/BOOTX64.EFI` y los `.bin`) | **BSD-2-Clause** | `limine --version` y el `COPYING` del paquete |
| `CONFIG.md` / `USAGE.md` (de donde salen las 52 opciones) | **BSD-2-Clause** | upstream |

**El repo no incluye código de Limine.** Solo lo usa (lo ejecuta en QEMU) y
cita su documentación. La licencia BSD-2-Clause permite, además, forkearlo
y recompilarlo — que es la vía para hacer cards de verdad.

---

## ⚠️ Assets de terceros — NO se redistribuyen

| Asset | Origen | Términos | ¿Está en el repo? |
|---|---|---|---|
| `rei.png` / `rei.mp4` | **DesktopHut** (`blue-light-anime-girl`), reempaquetado en [SilentSDDM](https://github.com/uiriansan/SilentSDDM) | los del sitio de origen | **No** |
| `ken.*` | MoeWalls (anime Ken Kaneki) | los de MoeWalls | No |
| `silvia.*` | MoeWalls (anime Nissan Silvia) | los de MoeWalls | No |
| `mountain.jpg`, `default.jpg` | Pexels (Joyston Judah) | Pexels License | No |

**Por qué no están**: el código de SilentSDDM es GPL-3.0, pero **los fondos
son assets de terceros que no heredan esa licencia**. Publicarlos en este repo
redistribuiría material de DesktopHut y MoeWalls bajo unos términos que no
son los del proyecto original.

**Cómo usarlos igual**: están instalados en tu sistema, y el preview los
puede usar:

```bash
limine-preview --wallpaper /usr/share/sddm/themes/silent/backgrounds/rei.png
```

Eso es **uso local**, no redistribución: no hay ningún problema.

El wallpaper por defecto del repo es un gradiente Rosé Pine generado para
este proyecto, justamente para no depender de ningún asset externo.

---

## La regla general

> **La licencia del código no cubre los assets que el proyecto empaqueta.**

Un `cp -r` de un theme te lleva también sus wallpapers y sus fuentes, y cada
uno con sus propios términos. Si publicás, auditá por separado:

```bash
# El código
head -3 <componente>/LICENSE

# Las fuentes: ¿el OFL viaja con ellas?
ls <dir-fuentes>/ | grep -i "ofl\|licen"     # vacío = gap de atribución

# Los media: los créditos del README dicen de dónde salió cada uno
grep -i -A2 "background\|wallpaper\|credit" <componente>/README.md
```

El bloque de créditos **es el manifiesto de licencias de los media**: dice
de dónde salió cada archivo, y por lo tanto qué términos aplican.

---

## Lo que SÍ podés hacer con los screenshots

**Publicar capturas de pantalla no es distribuir el software.** Mostrar el
login screen o el menú de Limine en el portfolio no transmite el código
fuente del theme y **no activa ninguna obligación de licencia**.

| Acción | Licencias |
|---|---|
| Instalar y usar Limine / SilentSDDM localmente | ✅ sin obligación |
| Usar `rei.png` como wallpaper local | ✅ sin obligación (uso local) |
| Mostrar screenshots en el portfolio | ✅ no es distribución |
| Publicar **este repo** (código original + gradiente propio) | ✅ MIT |
| Publicar el **código** de SilentSDDM | ⚠️ GPL-3.0: hay que subir el `LICENSE` y ofrecer la fuente |
| Publicar los **wallpapers** de SilentSDDM | ⚠️ los términos de DesktopHut / MoeWalls / Pexels, no la GPL |

---

## Referencias

- [Limine (BSD-2-Clause)](https://github.com/Limine-Bootloader/Limine)
- [SilentSDDM (GPL-3.0)](https://github.com/uiriansan/SilentSDDM) — y sus créditos de fondos
- [Pexels License](https://www.pexels.com/license/)
- [Material 3 shapes — `soramanew/m3shapes`](https://github.com/soramanew/m3shapes)
