#!/usr/bin/env bash
# limine-preview — ver el menú de Limine sin reiniciar la máquina.
#
# Construye un ISO híbrido con tu config de Limine y lo abre en QEMU con KVM.
# El render es EXACTO: es el mismo bootloader, el mismo wallpaper y la misma
# paleta. No toca el disco real.
#
#   limine-preview                # preview del menú (lo de siempre)
#   limine-preview real           # con tu config real de /boot/limine.conf
#   limine-preview shot           # captura a PNG y sale
#   limine-preview shot mi.png    # captura a un archivo
#   limine-preview wall foto.png  # preview con otro wallpaper
#   limine-preview wallpapers     # regenera las dos versiones del wallpaper
#   limine-preview setup          # instala dependencias y archivos
#   limine-preview help           # esta ayuda
#
set -euo pipefail

# ── Rutas ────────────────────────────────────────────────────────────────────
LIMINE_DIR="${LIMINE_DIR:-/usr/share/limine}"
WORK_DIR="${WORK_DIR:-/tmp/limine-preview}"
# La config de PRUEBA (la que carga el preview por default).
# Respeta XDG_CONFIG_HOME, igual que install.sh.
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/limine-preview"
DEFAULT_CONF="$CONFIG_DIR/limine.conf"
# El wallpaper por defecto es el del repo. Cualquier otro se pasa con `wall`.
DEFAULT_WALLPAPER="$CONFIG_DIR/wallpaper/rei.png"

conf="${DEFAULT_CONF}"
wallpaper="${DEFAULT_WALLPAPER}"
shot=""
gui=1
resolution="1920x1080"
timeout_secs="15"
real=0
command="preview"

usage() {
  cat <<'EOF'
limine-preview — ver el menú de Limine sin reiniciar

USO
  limine-preview [comando] [opciones]

COMANDOS
  (ninguno)        Preview del menú, con la config de prueba
  real             Preview con tu config real de /boot/limine.conf
  shot [archivo]   Captura el menú a un PNG y sale
  wall <imagen>    Preview con otro wallpaper
  wallpapers       Regenera las dos versiones del wallpaper (real y difuminada)
  setup            Instala dependencias y archivos
  help             Esta ayuda

EJEMPLOS
  limine-preview
  limine-preview shot
  limine-preview wall ~/Pictures/foto.png
  limine-preview real

OPCIONES AVANZADAS
  -c, --config <ruta>     Config de Limine a usar
  -w, --wallpaper <ruta>  Wallpaper a incluir en el ISO
      --resolution <WxH>  Resolución de la interfaz (default: 1920x1080)
      --timeout <seg>     Espera antes de capturar (default: 15)
EOF
}

# ── Dispatch ────────────────────────────────────────────────────────────────
cmd="${1:-}"
[[ $# -gt 0 ]] && shift

case "$cmd" in
  ""|preview)  command="preview" ;;
  real)        command="preview"; real=1 ;;
  shot)        command="shot"; gui=0
                if [[ $# -gt 0 && "$1" != -* ]]; then shot="$1"; shift; fi ;;
  wall)        command="preview"
                if [[ $# -lt 1 ]]; then echo "Falta la imagen" >&2; exit 1; fi
                wallpaper="$1"; shift ;;
  wallpapers)  command="wallpapers" ;;
  setup)       command="setup" ;;
  help|-h|--help) usage; exit 0 ;;
  *)
    # Si no es un subcomando conocido, que sea un flag legacy.
    case "$cmd" in
      -c|--config|-w|--wallpaper|--resolution|--timeout|--shot|--real|--gui) ;;
      *) echo "Comando desconocido: $cmd" >&2; usage; exit 1 ;;
    esac
    set -- "$cmd" "$@" ;;
esac

# ── Opciones (comunes a todos los comandos) ─────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -c|--config)     conf="$2"; shift 2 ;;
    -w|--wallpaper)  wallpaper="$2"; shift 2 ;;
    --resolution)    resolution="$2"; shift 2 ;;
    --timeout)       timeout_secs="$2"; shift 2 ;;
    --shot)          shot="$2"; gui=0; shift 2 ;;
    --real)          real=1; shift ;;
    --gui)           gui=1; shift ;;
    -h|--help)       usage; exit 0 ;;
    *) echo "opción desconocida: $1" >&2; usage; exit 1 ;;
  esac
done

# ── Comandos que no son de preview ─────────────────────────────────────────
case "$command" in
  wallpapers)
    if ! command -v prepare-wallpapers >/dev/null 2>&1; then
      echo "FALTA: prepare-wallpapers (corré scripts/install.sh)" >&2
      exit 1
    fi
    exec prepare-wallpapers "$@"
    ;;
  setup)
    repo_dir="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")/.." 2>/dev/null && pwd || true)"
    if [[ -n "$repo_dir" && -x "$repo_dir/scripts/install.sh" ]]; then
      exec "$repo_dir/scripts/install.sh"
    fi
    echo "No encuentro el install.sh del repo. Bajá el repo y corré:" >&2
    echo "    ./scripts/install.sh" >&2
    exit 1
    ;;
esac

# ── Config real del sistema ────────────────────────────────────────────────
if [[ "$real" == "1" ]]; then
  # mktemp evita chocar con un archivo viejo que quedo de una corrida
  # anterior (creado por root y sin permisos de escritura para el usuario).
  real_conf="$(mktemp /tmp/limine-preview-real.XXXXXX.conf)"
  # OJO: `sudo cat ... > archivo` y no `sudo cp ... archivo`. Con `sudo cp` el
  # archivo queda con el modo de /boot/limine.conf (-rwx------ root), y el
  # usuario no lo puede leer en el paso siguiente.
  if ! sudo cat /boot/limine.conf > "$real_conf" 2>/dev/null; then
    echo "No se pudo leer /boot/limine.conf (hace falta sudo)" >&2
    rm -f "$real_conf"
    exit 1
  fi
  chmod 644 "$real_conf"
  conf="$real_conf"
  REAL_CONF_TMP="$real_conf"
  # Si la config real trae su propio wallpaper, se avisa.
  if grep -qE "^wallpaper\s*:" "$real_conf" 2>/dev/null; then
    echo "AVISO: la config real define su propio wallpaper; el inyectado lo pisa." >&2
  fi
fi

# ── Comprobaciones ──────────────────────────────────────────────────────────
for bin in xorriso qemu-system-x86_64; do
  command -v "$bin" >/dev/null 2>&1 || { echo "FALTA: $bin" >&2; exit 1; }
done
[[ -f "$conf" ]]       || { echo "No existe la config: $conf" >&2; exit 1; }
[[ -f "$wallpaper" ]]  || { echo "No existe el wallpaper: $wallpaper" >&2; exit 1; }
[[ -e /dev/kvm ]]     || echo "AVISO: sin /dev/kvm, la VM va a ir por emulación (lenta)" >&2

# Si es una captura sin nombre, se genera uno con timestamp
if [[ -n "$shot" && "$shot" != /* && "$shot" != */* ]]; then
  shot="$PWD/$shot"
elif [[ -n "$shot" && "$shot" != /* ]]; then
  shot="$PWD/$shot"
fi
if [[ "$command" == "shot" && -z "$shot" ]]; then
  shot="$PWD/limine-preview-$(date +%Y%m%d-%H%M%S).png"
fi

# ── Construir el árbol del ISO ──────────────────────────────────────────────
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR/EFI/BOOT"

cp "$LIMINE_DIR/limine-uefi-cd.bin" "$WORK_DIR/"
cp "$LIMINE_DIR/limine-bios-cd.bin"  "$WORK_DIR/"
cp "$LIMINE_DIR/limine-bios.sys"      "$WORK_DIR/"
cp "$LIMINE_DIR/BOOTX64.EFI"          "$WORK_DIR/EFI/BOOT/"

# La config: se copia tal cual y se le inyecta el wallpaper y la resolución.
cp "$conf" "$WORK_DIR/limine.conf"
python3 - "$WORK_DIR/limine.conf" "$wallpaper" "$resolution" <<'PY'
import re, sys

path, wallpaper, resolution = sys.argv[1], sys.argv[2], sys.argv[3]
text = open(path, encoding="utf-8", errors="replace").read()

# Borra las opciones de theming previas para que la config inyectada gane.
text = re.sub(r"^\s*(wallpaper|wallpaper_style|interface_resolution)\s*:.*$", "", text, flags=re.M)

# El timeout se fuerza a "no": si no, Limine autobootea la entrada default
# (que en la config real apunta a un kernel con hash que no esta en el ISO)
# y se va de la pantalla antes de que puedas mirarla.
text = re.sub(r"^\s*timeout\s*:.*$", "", text, flags=re.M)

inject = f"""# ── inyectado por limine-preview ──
timeout: no
wallpaper: boot():/wallpaper.png
wallpaper_style: centered
interface_resolution: {resolution}
"""

# OJO: en Limine las opciones GLOBALES tienen que ir antes de la primera
# entrada de menu (una linea que empieza con "/"). Si se inyectan al final,
# Limine las trata como opciones de esa entrada y las ignora.
lines = text.splitlines()
insert_at = len(lines)
for index, line in enumerate(lines):
    if line.lstrip().startswith("/"):
        insert_at = index
        break

new_lines = lines[:insert_at] + inject.rstrip("\n").split("\n") + lines[insert_at:]
open(path, "w", encoding="utf-8").write("\n".join(new_lines) + "\n")
PY

cp "$wallpaper" "$WORK_DIR/wallpaper.png"

# ── Construir el ISO híbrido (UEFI + BIOS) ──────────────────────────────────
iso="$WORK_DIR/limine-preview.iso"
(
  cd "$WORK_DIR"
  xorriso -as mkisofs -R -r -J -b limine-bios-cd.bin \
    -no-emul-boot -boot-load-size 4 -boot-info-table -hfsplus \
    -apm-block-size 2048 --efi-boot limine-uefi-cd.bin \
    -efi-boot-part --efi-boot-image --protective-msdos-label \
    . -o "$iso" >/dev/null 2>&1
)

echo "ISO listo: $iso"

# ── Arrancar QEMU ───────────────────────────────────────────────────────────
monitor_sock="$WORK_DIR/monitor.sock"
rm -f "$monitor_sock"

qemu_args=(
  -enable-kvm
  -m 512
  -cdrom "$iso"
  -monitor "unix:$monitor_sock,server,nowait"
  -boot d
)

# UEFI (OVMF) — es lo que usa la maquina real, y el menu grafico con
# wallpaper solo funciona bien bajo UEFI. Sin esto, QEMU arranca con SeaBIOS
# y Limine cae a modo texto.
if [[ -f /usr/share/edk2/x64/OVMF_CODE.4m.fd ]]; then
  cp /usr/share/edk2/x64/OVMF_VARS.4m.fd "$WORK_DIR/OVMF_VARS.fd"
  qemu_args=(
    -enable-kvm
    -m 512
    -drive "if=pflash,format=raw,readonly=on,file=/usr/share/edk2/x64/OVMF_CODE.4m.fd"
    -drive "if=pflash,format=raw,file=$WORK_DIR/OVMF_VARS.fd"
    -cdrom "$iso"
    -monitor "unix:$monitor_sock,server,nowait"
    -boot d
  )
  uefi=1
else
  uefi=0
  echo "AVISO: sin OVMF, arranca con SeaBIOS y el menu puede salir en modo texto" >&2
fi

if [[ "$gui" == "1" ]]; then
  qemu_args+=(-display gtk)
else
  qemu_args+=(-display none -vnc :0)
fi

qemu-system-x86_64 "${qemu_args[@]}" &
qemu_pid=$!

cleanup() {
  kill "$qemu_pid" 2>/dev/null || true
  wait "$qemu_pid" 2>/dev/null || true
  [[ -n "${REAL_CONF_TMP:-}" ]] && rm -f "$REAL_CONF_TMP"
}
trap cleanup EXIT

# Espera a que el monitor responda
for _ in $(seq 1 50); do
  [[ -S "$monitor_sock" ]] && break
  sleep 0.2
done

# ── Captura del framebuffer ────────────────────────────────────────────────
if [[ -n "$shot" ]]; then
  echo "Esperando ${timeout_secs}s a que arranque el menu..."
  sleep "$timeout_secs"

  raw="$WORK_DIR/shot.ppm"
  rm -f "$raw"

  # El screendump necesita que la VM ya haya pintado. Se reintenta: con la
  # config real (99 KB, cientos de entradas) el menu tarda mas en dibujarse.
  for attempt in 1 2 3 4 5; do
    echo "screendump $raw" | socat - "UNIX-CONNECT:$monitor_sock" >/dev/null 2>&1 || true
    for _ in $(seq 1 20); do
      [[ -s "$raw" ]] && break 2
      sleep 0.3
    done
    echo "  reintentando la captura ($attempt/5)..."
    sleep 2
  done

  if [[ -s "$raw" ]]; then
    magick "$raw" -resize 1920x1080 "$shot" 2>/dev/null \
      || convert "$raw" -resize 1920x1080 "$shot" 2>/dev/null \
      || cp "$raw" "$shot"
    echo "Captura: $shot"
  else
    echo "No se pudo capturar el framebuffer" >&2
    exit 1
  fi
else
  echo ""
  echo "QEMU corriendo. Para salir: Ctrl+Q (o cerrá la ventana)."
  echo "Monitor QEMU en: $monitor_sock"
  wait "$qemu_pid"
fi
