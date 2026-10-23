# El flujo completo

Qué pasa, en orden, desde que escribís `limine-preview` hasta que el menú de
Limine aparece en la ventana de QEMU.

---

## El mapa: quién es quién

| Pieza | Dónde vive | Qué hace |
|---|---|---|
| **tu config** | `~/.config/limine-preview/limine.conf` | la config de Limine que se prueba |
| **el script** | `~/.local/bin/limine-preview` | arma el ISO y lanza QEMU |
| **xorriso** | `libisoburn` | construye el ISO híbrido |
| **QEMU** | `qemu-desktop` | emula la máquina |
| **OVMF** | `edk2-ovmf` | el firmware UEFI (el mismo que usa tu laptop) |
| **Limine** | `/usr/share/limine/BOOTX64.EFI` | el bootloader que se prueba |

---

## La secuencia, paso a paso

### 1. Parseo de argumentos

```bash
config="${HOME}/.config/limine-preview/limine.conf"
wallpaper="${XDG_CONFIG_HOME:-$HOME/.config}/limine-preview/wallpaper/rosepine.png"
```

Opciones: `-c/--config`, `-w/--wallpaper`, `--real`, `--resolution`,
`--shot`, `--gui`, `--timeout`.

### 2. Si es `--real`: copiar la config del sistema

```bash
real_conf="$(mktemp /tmp/limine-preview-real.XXXXXX.conf)"
sudo cat /boot/limine.conf > "$real_conf"
chmod 644 "$real_conf"
```

> **`sudo cat > archivo`, no `sudo cp archivo`.** Con `sudo cp` el archivo
> queda con el modo de `/boot/limine.conf` (`-rwx------ root`) y el usuario
> no lo puede leer en el paso siguiente. Y `mktemp` evita chocar con un
> archivo viejo de una corrida anterior que quedó como root.

### 3. Inyectar el wallpaper y la resolución

Se borran las opciones de theming previas de la config y se inyectan las
nuevas **antes de la primera entrada de menú**:

```python
text = re.sub(r"^\s*(wallpaper|wallpaper_style|interface_resolution)\s*:.*$", "", text, flags=re.M)
text = re.sub(r"^\s*timeout\s*:.*$", "", text, flags=re.M)

inject = """timeout: no
wallpaper: boot():/wallpaper.png
wallpaper_style: centered
interface_resolution: 1920x1080"""
```

> **Por qué antes de la primera entrada**: en `limine.conf`, una entrada de
> menú empieza con `/`. Cualquier opción después de esa línea **pertenece a
> esa entrada**, no es global. Inyectar al final = opción ignorada.
>
> **Por qué `timeout: no`**: con `timeout: 0` Limine bootea al instante sin
> mostrar el menú; con `timeout: 5` (el de la config real) autobootea la
> entrada default a los 5 s y se va de pantalla. Y como esa entrada apunta a
> un kernel con hash que no está en el ISO, se ve un error
> *"failed to open kernel... Is the path correct?"* en vez del menú.

### 4. Armar el árbol del ISO

```
/tmp/limine-preview/
├── limine.conf          ← tu config + lo inyectado
├── wallpaper.png        ← el wallpaper
├── EFI/BOOT/BOOTX64.EFI ← el binario UEFI
├── limine-uefi-cd.bin   ← el binario UEFI de CD
├── limine-bios-cd.bin   ← el binario BIOS de CD
└── limine-bios.sys      ← el arranque BIOS
```

### 5. Construir el ISO híbrido

```bash
xorriso -as mkisofs -R -r -J -b limine-bios-cd.bin \
  -no-emul-boot -boot-load-size 4 -boot-info-table -hfsplus \
  -apm-block-size 2048 --efi-boot limine-uefi-cd.bin \
  -efi-boot-part --efi-boot-image --protective-msdos-label \
  . -o limine-preview.iso
```

Es **híbrido**: arranca en UEFI y en BIOS.

### 6. Arrancar QEMU

```bash
qemu-system-x86_64 \
  -enable-kvm \
  -m 512 \
  -drive if=pflash,format=raw,readonly=on,file=/usr/share/edk2/x64/OVMF_CODE.4m.fd \
  -drive if=pflash,format=raw,file=/tmp/limine-preview/OVMF_VARS.fd \
  -cdrom /tmp/limine-preview/limine-preview.iso \
  -monitor unix:/tmp/limine-preview/monitor.sock,server,nowait \
  -boot d
```

> **OVMF es obligatorio para ver el wallpaper.** Con SeaBIOS (el default de
> QEMU), Limine arranca en **modo texto**: se ve la lista de entradas pero sin
> imagen de fondo. `OVMF_VARS.4m.fd` se copia a un temporal porque QEMU
> escribe ahí las variables NVRAM y no queremos tocar el original.

### 7. La captura (opcional)

En vez de abrir la ventana, el script habla con el **monitor de QEMU** por un
socket Unix:

```bash
echo "screendump /tmp/limine-preview/shot.ppm" | socat - UNIX-CONNECT:/tmp/limine-preview/monitor.sock
magick shot.ppm -resize 1920x1080 preview.png
```

`screendump` vuelca el framebuffer de la VM a un PPM. Es **el render real**,
no una simulación.

---

## El diagrama del flujo de datos

```
  ~/.config/limine-preview/limine.conf
              │
              │  (--real: sudo cat /boot/limine.conf)
              ▼
      ┌──────────────────────┐
      │  inyectar wallpaper  │
      │  + interface_resolution│
      │  + timeout: no        │
      │  ANTES de la 1ª entrada│
      └──────────┬───────────┘
                 ▼
     /tmp/limine-preview/
       ├── limine.conf
       ├── wallpaper.png
       ├── EFI/BOOT/BOOTX64.EFI
       ├── limine-{uefi,bios}-cd.bin
       └── limine-bios.sys
                 │
                 │  xorriso -as mkisofs
                 ▼
        limine-preview.iso  (híbrido)
                 │
                 │  qemu-system-x86_64 -enable-kvm
                 │  + OVMF (UEFI) + -cdrom
                 ▼
      ┌────────────────────┐
      │  EL MENÚ REAL      │  ← el mismo Limine, el mismo render
      │  de Limine         │
      └────────────────────┘
                 │
                 │  screendump (monitor QEMU)
                 ▼
            preview.png
```

---

## Y la rama del login screen

Esa no pasa por el preview: es Noctalia → D-Bus → accounts-daemon → SDDM.

```
Noctalia [shell].avatar_path
  └─ escribe org.freedesktop.Accounts → IconFile
        └─ accounts-daemon copia a /var/lib/AccountsService/icons/<usuario>
              └─ SDDM lo busca (prioridad nº3) → theme silent → pantalla de login
```

Son **dos consumidores del mismo `avatar_path`**: el shell (recortado con
formas Material 3 por `kitty-shapes`) y el login screen (sin recortar).

---

## Los puntos de fallo, todos

| Si falla… | Qué pasa | Cómo se arregla |
|---|---|---|
| No está `qemu-system-x86_64` | error de command not found | `sudo pacman -S qemu-desktop` |
| No está `xorriso` | el ISO no se construye | `sudo pacman -S libisoburn` |
| No está OVMF | arranca con SeaBIOS → **modo texto, sin wallpaper** | `sudo pacman -S edk2-ovmf` |
| No está `/dev/kvm` | la VM va por emulación (muy lenta) | revisá la virtualización en la BIOS |
| El wallpaper no se ve | `wallpaper:` quedó después de una entrada | que vaya antes de la primera `/` |
| El menú no aparece, bootea | `timeout: 0` o `timeout: 5` | `timeout: no` |
| "Could not read from CDROM" | se usó `-drive if=virtio` | `-cdrom` |
| "failed to open kernel" | la entrada指针 a un kernel que no está en el ISO | es normal si autobootea; usá `timeout: no` |
| El texto se ve diminuto | `term_font_scale` automático = 1×1 a 1920×1080 | `term_font_scale: 2x2` |
| `sudo limine-preview` → command not found | `~/.local/bin` no está en el `secure_path` | el shim en `/usr/local/bin/limine-preview` |
| `cp: Permission denied` | se usó `sudo cp` para la config real | `sudo cat ... > archivo` |

---

## Tiempos

| Tramo | Coste |
|---|---|
| Construir el ISO | ~1 s |
| Arranque de QEMU con KVM | ~2-3 s hasta el menú |
| Arranque sin KVM | 30 s - 2 min |
| Captura (`screendump`) | <1 s |

El `--timeout` (15 s por default) es el margen que espera el script antes de
capturar: tiempo de sobra para que arranque la VM con KVM.

---

## Tus ficheros, de un vistazo

```
~/.local/bin/limine-preview                    el script
/usr/local/bin/limine-preview                   el shim para sudo
~/.config/limine-preview/limine.conf           la config de prueba
~/.config/limine-preview/wallpaper/rosepine.png  el wallpaper
/tmp/limine-preview/                           el ISO y el socket del monitor
/tmp/limine-preview-real.*.conf                la config real (temporal)
```

---

## Nota de diseño

El preview **no toca el bootloader real**: `/boot/limine.conf` solo se lee.
El único momento en que se escribe algo en el sistema es cuando aplicás el
tema de verdad, y ese flujo está en [`APLICAR.md`](APLICAR.md) — con su
rollback.
