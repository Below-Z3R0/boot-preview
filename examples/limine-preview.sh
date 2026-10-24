#!/usr/bin/env bash
# limine-preview — ver el menú de Limine sin reiniciar la máquina.
#
# Construye un ISO híbrido con tu config de Limine y lo abre en QEMU con KVM.
# El render es EXACTO: es el mismo bootloader, el mismo wallpaper y la misma
# paleta. No toca el disco real.
#
#   limine-preview                # preview del menú (lo de siempre)
#   limine-preview real           # con tu config real de /boot/limine.conf
#   limine-preview shot           # captura a PNG (en <repo>/previews/)
#   limine-preview add IMAGEN     # pone una imagen como wallpaper del preview
#   limine-preview edit           # abre la config del preview en un editor
#   limine-preview wall IMAGEN    # preview con otro wallpaper (sin instalarlo)
#   limine-preview wallpapers     # genera las dos versiones del wallpaper
#   limine-preview apply          # aplica wallpaper + config al Limine real
#   limine-preview apply config   # solo la config visual
#   limine-preview apply wallpaper # solo el wallpaper
#   limine-preview setup          # instala dependencias y archivos
#   limine-preview help           # esta ayuda
#
set -euo pipefail

# ── Rutas ────────────────────────────────────────────────────────────────────
LIMINE_DIR="${LIMINE_DIR:-/usr/share/limine}"
WORK_DIR="${WORK_DIR:-/tmp/limine-preview}"

# BUG FIX: abajo hay un `rm -rf "$WORK_DIR"`. Si alguien exporta WORK_DIR=/ o
# WORK_DIR=$HOME, eso se lleva medio sistema. Se valida antes de tocar nada.
if [[ -z "$WORK_DIR" || "$WORK_DIR" == "/" || "$WORK_DIR" == "$HOME" || "$WORK_DIR" == "/etc" || "$WORK_DIR" == "/usr" ]]; then
  echo "WORK_DIR peligroso: '$WORK_DIR'" >&2
  echo "  (no puede ser vazio, /, \$HOME, /etc ni /usr)" >&2
  exit 1
fi
# La config de PRUEBA (la que carga el preview por default).
# Respeta XDG_CONFIG_HOME, igual que install.sh.
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/limine-preview"
DEFAULT_CONF="$CONFIG_DIR/limine.conf"
# El wallpaper por defecto es el del repo. Cualquier otro se pasa con `wall`.
# La imagen ACTIVA: una sola para el preview y para el Limine real. La fija
# `prepare-wallpapers` (deja la difuminada) o `add`. Antes apuntaba a rei.png,
# que era la nitida, mientras que `apply` copiaba wallpaper.png (la
# difuminada): el preview y el sistema real mostraban cosas distintas.
DEFAULT_WALLPAPER="$CONFIG_DIR/wallpaper/wallpaper.png"

# La ruta del repo, la guarda install.sh. Se usa para las capturas (previews/).
REPO_DIR=""
[[ -f "$CONFIG_DIR/repo" ]] && REPO_DIR="$(cat "$CONFIG_DIR/repo")"
[[ -d "$REPO_DIR" ]] || REPO_DIR=""

conf="${DEFAULT_CONF}"
wallpaper="${DEFAULT_WALLPAPER}"
shot=""
gui=1
resolution="1920x1080"
timeout_secs="15"
real=0
command="preview"
add_image=""
apply_what="all"

usage() {
  cat <<'EOF'
limine-preview — ver el menú de Limine sin reiniciar

USO
  limine-preview [comando] [opciones]

VER
  (ninguno)        Preview del menú, con la config de prueba
  real             Preview con tu config real de /boot/limine.conf
  wall <imagen>    Preview con otro wallpaper (sin instalarlo)

CAPTURAR
  shot [archivo]   Captura el menú a un PNG. Sin nombre va a
                   <repo>/previews/ con timestamp

EDITAR
  add <imagen>     Pone una imagen como wallpaper del preview
  edit             Abre la config del preview en tu editor

APLICAR AL LIMINE REAL
  apply             Wallpaper + config visual
  apply config      Solo la config visual
  apply wallpaper   Solo el wallpaper

WALLPAPERS
  wallpapers       Genera las dos versiones (real y difuminada)

OTROS
  setup            Instala dependencias y archivos
  help             Esta ayuda

EJEMPLOS
  limine-preview
  limine-preview shot
  limine-preview add ~/Pictures/foto.png
  limine-preview edit
  limine-preview apply

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
  add)         command="add"
                if [[ $# -lt 1 ]]; then echo "Falta la imagen" >&2; exit 1; fi
                add_image="$1"; shift ;;
  edit)        command="edit" ;;
  apply)       command="apply"; apply_what="all"
                if [[ $# -gt 0 && "$1" != -* ]]; then
                  case "$1" in
                    config|wallpaper) apply_what="$1"; shift ;;
                    *) echo "Opción de apply desconocida: $1" >&2
                       echo "  (valores: config, wallpaper)" >&2; exit 1 ;;
                  esac
                fi ;;
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

  add)
    # Pone una imagen como wallpaper del preview. La copia al directorio de
    # wallpapers y actualiza el default, para que quede "instalada" y no
    # haya que volver a pasarle la ruta.
    [[ -f "$add_image" ]] || { echo "No existe: $add_image" >&2; exit 1; }
    mkdir -p "$CONFIG_DIR/wallpaper"
    target="$CONFIG_DIR/wallpaper/wallpaper.png"
    cp "$add_image" "$target"
    ok_msg="  ✓ $add_image  ->  $target"
    echo "$ok_msg"
    # El preview usa el wallpaper recien puesto
    wallpaper="$target"
    echo ""
    echo "  Para verlo:  limine-preview"
    echo "  Para aplicarlo al Limine real:  limine-preview apply wallpaper"
    exit 0
    ;;

  edit)
    # Abre la config del preview en un editor.
    editor="${VISUAL_EDITOR:-${EDITOR:-}}"
    if [[ -z "$editor" ]]; then
      for candidate in code nvim micro nano vim; do
        if command -v "$candidate" >/dev/null 2>&1; then editor="$candidate"; break; fi
      done
    fi
    [[ -n "$editor" ]] || { echo "No encuentro un editor (proba con EDITOR=...)" >&2; exit 1; }
    echo "  Abriendo $CONFIG_DIR/limine.conf con $editor"
    exec "$editor" "$CONFIG_DIR/limine.conf"
    ;;

  apply)
    # Aplica al Limine real. El wallpaper va a /boot/splash.png (la ESP) y el
    # bloque de theming a /boot/limine.conf, antes de la primera entrada.
    need_wallpaper=0
    need_config=0
    case "$apply_what" in
      all)      need_wallpaper=1; need_config=1 ;;
      config)   need_config=1 ;;
      wallpaper) need_wallpaper=1 ;;
    esac

    echo "▸ Aplicando al Limine real"
    echo ""

    # ── El wallpaper ──
    if [[ $need_wallpaper == 1 ]]; then
      src_wall="$CONFIG_DIR/wallpaper/wallpaper.png"
      [[ -f "$src_wall" ]] || src_wall="$DEFAULT_WALLPAPER"
      [[ -f "$src_wall" ]] || { echo "No hay wallpaper que aplicar" >&2; exit 1; }
      # OJO: /boot es la ESP en vfat (fmask=0077), asi que los permisos los
      # fija el mount y `chmod` no hace nada ahi (devuelve 0 y no cambia
      # nada). No hace falta normalizar el modo: Limine lee la ESP directo,
      # sin pasar por el kernel.
      sudo cp "$src_wall" /boot/splash.png
      echo "  ✓ wallpaper  ->  /boot/splash.png"
    fi

    # ── La config visual ──
    if [[ $need_config == 1 ]]; then
      # Red de seguridad: el conf real tiene 174 entradas del menu. Si algo
      # sale mal al reescribirlo, se puede volver con este backup.
      backup="/boot/limine.conf.bak-$(date +%Y%m%d-%H%M%S)"
      sudo cp /boot/limine.conf "$backup" 2>/dev/null && \
        echo "  ✓ backup      ->  $backup"
      # /boot es la ESP: no conviene llenarla de backups. Se conservan los 5
      # mas recientes.
      sudo bash -c 'ls -1t /boot/limine.conf.bak-* 2>/dev/null | tail -n +6 | xargs -r rm -f'

      # Se toma el bloque de theming de la config del PREVIEW (que es la
      # que edita el usuario con `limine-preview edit`) y se inyecta en el
      # conf real.
      sudo python3 - "$CONFIG_DIR/limine.conf" <<'PY'
import re, sys

preview_path = sys.argv[1]
real_path = "/boot/limine.conf"

preview = open(preview_path, encoding="utf-8", errors="replace").read()

# Si una actualizacion borro el conf entero, `apply` no puede reconstruir las
# entradas del menu (no las tiene). Fallar con un mensaje claro es mejor que
# un traceback.
import os
if not os.path.exists(real_path):
    print("  ! No existe /boot/limine.conf")
    print("    apply solo reescribe el bloque de theming; las entradas del")
    print("    menu no se pueden reconstruir desde aca.")
    print("    Restauralo desde un backup:")
    print("      sudo ls -1t /boot/limine.conf.bak-* /boot/limine.conf.old")
    print("      sudo cp /boot/limine.conf.bak-XXXX /boot/limine.conf")
    sys.exit(1)

real = open(real_path, encoding="utf-8", errors="replace").read()

# Las opciones de theming que se llevan al Limine real.
THEME_OPTS = [
    "wallpaper", "wallpaper_style", "backdrop",
    "term_margin", "term_margin_gradient", "term_background", "term_foreground",
    "term_palette", "term_palette_bright", "term_font", "term_font_size",
    "term_font_scale", "term_font_spacing",
    "interface_resolution", "interface_rotation", "interface_branding",
    "interface_branding_colour", "interface_help_colour",
    "interface_help_colour_bright", "interface_help_hidden",
]

block = []
for line in preview.splitlines():
    m = re.match(r"^\s*([a-z_0-9]+)\s*:", line)
    if m and m.group(1) in THEME_OPTS:
        # El wallpaper del preview apunta al ISO (boot():/wallpaper.png);
        # en el Limine real va a /boot/splash.png.
        if m.group(1) == "wallpaper":
            line = "wallpaper: boot():/splash.png"
        block.append(line)

if not block:
    print("  ! La config del preview no tiene opciones de theming")
    sys.exit(1)

# Se quitan las del conf real (y el bloque anterior, si lo hubiera).
#
# La cabecera y los comentarios del bloque se borran SIEMPRE, aunque hayan
# quedado huerfanos (por ejemplo si una actualizacion borro las opciones y
# dejo los comentarios): si no, al reinsertar el bloque se duplican.
OWN_COMMENTS = (
    "# ── Limine theming (b3l0wz3r0-os) ──",
    "# Aplicado por limine-preview apply",
    "# y volvé a aplicar para cambiarlo",
)

lines = []
skipping = False
for line in real.splitlines():
    m = re.match(r"^\s*([a-z_0-9]+)\s*:", line)
    if m and m.group(1) in THEME_OPTS:
        continue
    if any(line.strip().startswith(c) for c in OWN_COMMENTS):
        skipping = True
        continue
    if skipping:
        if line.strip().startswith("#") or not line.strip():
            continue
        skipping = False
    lines.append(line)

# Y se inserta el bloque nuevo antes de la primera entrada.
insert_at = len(lines)
for index, line in enumerate(lines):
    if line.lstrip().startswith("/"):
        insert_at = index
        break

header = ["# ── Limine theming (b3l0wz3r0-os) ──",
          "# Aplicado por limine-preview apply. Editá la config del preview",
          "# y volvé a aplicar para cambiarlo."]
new = lines[:insert_at] + header + block + lines[insert_at:]
open(real_path, "w", encoding="utf-8").write("\n".join(new) + "\n")
print(f"  ✓ config visual  ->  /boot/limine.conf ({len(block)} opciones)")
PY
    fi

    echo ""
    echo "  Para verlo antes de reiniciar:"
    echo "      limine-preview real"
    echo ""
    echo "  Para que tome efecto:  sudo reboot"
    exit 0
    ;;

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

# ── Donde va la captura ─────────────────────────────────────────────────────
# Sin nombre de archivo: va a <repo>/previews/ con timestamp, para que las
# capturas no se mezclen con el resto de tus ficheros.
if [[ "$command" == "shot" ]]; then
  if [[ -z "$shot" ]]; then
    if [[ -n "$REPO_DIR" ]]; then
      shot_dir="$REPO_DIR/previews"
    else
      shot_dir="$PWD"
    fi
    mkdir -p "$shot_dir"
    shot="$shot_dir/limine-preview-$(date +%Y%m%d-%H%M%S).png"
  elif [[ "$shot" != /* ]]; then
    # Un nombre relativo va al directorio de previews tambien.
    if [[ -n "$REPO_DIR" ]]; then
      mkdir -p "$REPO_DIR/previews"
      shot="$REPO_DIR/previews/$(basename "$shot")"
    else
      shot="$PWD/$shot"
    fi
  fi
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
  # Sin GUI no hace falta ningun display: el screendump va por el monitor.
  # Antes se agregaba `-vnc :0`, que abria el puerto 5900 sin que nadie lo
  # usara (y chocaba si habia otra VM).
  qemu_args+=(-display none)
fi

qemu-system-x86_64 "${qemu_args[@]}" &
qemu_pid=$!

cleanup() {
  kill "$qemu_pid" 2>/dev/null || true
  wait "$qemu_pid" 2>/dev/null || true
  if [[ -n "${REAL_CONF_TMP:-}" ]]; then
    rm -f "$REAL_CONF_TMP"
  fi
  return 0
}
trap cleanup EXIT

# Espera a que el monitor responda
for _ in $(seq 1 50); do
  [[ -S "$monitor_sock" ]] && break
  sleep 0.2
done

# ── Captura del framebuffer ────────────────────────────────────────────────
if [[ -n "$shot" ]]; then
  raw="$WORK_DIR/shot.ppm"
  rm -f "$raw"

  # En vez de esperar un tiempo fijo, se le pregunta al monitor si Limine ya
  # dibujó el menu. Ahorra ~10s de los 15 que costaba antes.
  #
  # La señal NO es "hay algo pintado": el logo de TianoCore (OVMF) ya cuenta
  # como pintado, y se terminaba capturando el logo en vez del menu. La señal
  # buena es el CAMBIO DE RESOLUCION: OVMF arranca en 1280x800 y Limine la
  # cambia a la interface_resolution al tomar el control.
  deadline=$(( $(date +%s) + timeout_secs ))
  echo "Esperando a que arranque el menu (max ${timeout_secs}s)..."
  settled=0

  while :; do
    echo "screendump $raw" | socat - "UNIX-CONNECT:$monitor_sock" >/dev/null 2>&1 || true

    if [[ -s "$raw" ]]; then
      if command -v vips >/dev/null 2>&1; then
        size="$(vipsheader "$raw" 2>/dev/null | sed -n 's/.*: \([0-9]*x[0-9]*\).*/\1/p')"
      else
        size="$(magick identify -format '%wx%h' "$raw" 2>/dev/null)"
      fi

      if [[ "$size" == "$resolution" ]]; then
        # Ya es la resolucion del menu: falta que termine de pintar (Limine
        # dibuja el fondo y despues el texto).
        if [[ $settled == 1 ]]; then
          break
        fi
        settled=1
        sleep 0.5
        continue
      fi
    fi

    if [[ $(date +%s) -ge $deadline ]]; then
      echo "  se agoto el tiempo de espera, capturando lo que haya"
      break
    fi
    sleep 0.3
  done

  if [[ -s "$raw" ]]; then
    # vips es ~6x mas rapido que ImageMagick para esto
    if command -v vips >/dev/null 2>&1; then
      vips copy "$raw" "$shot" 2>/dev/null || magick "$raw" "$shot" 2>/dev/null || cp "$raw" "$shot"
    else
      magick "$raw" "$shot" 2>/dev/null || cp "$raw" "$shot"
    fi
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
