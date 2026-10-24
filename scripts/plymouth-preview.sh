#!/usr/bin/env bash
# plymouth-preview — ver el splash de Plymouth sin reiniciar la máquina.
#
# Construye un initramfs con el theme activo del sistema y lo arranca en QEMU.
# El splash lo dibuja Plymouth de verdad: mismo tema, mismos frames, mismo
# renderer (DRM).
#
#   plymouth-preview              # ventana con el splash
#   plymouth-preview shot [f.png] # captura el splash y sale
#   plymouth-preview themes       # lista los temas instalados
#   plymouth-preview theme <nom>  # cambia el tema del preview (no del sistema)
#   plymouth-preview build        # solo regenera el initramfs (cache)
#   plymouth-preview help
#
# LIMITACIÓN CONOCIDA
# -------------------
# El initramfs no tiene un sistema de archivos raíz. Cuando el init de
# mkinitcpio no puede montar el root, lanza la shell de emergencia y el
# splash desaparece. En la práctica el splash se ve durante unos segundos
# (a veces más, a veces menos: hay una race con la inicialización del DRM).
#
# `shot` lo maneja muestreando el framebuffer cada 400 ms y quedándose con
# el frame que más contenido tiene: así siempre se obtiene el splash, aunque
# sea de un instante.
#
# Para que el splash se quede de forma estable haría falta un rootfs real
# (kernel + root + init), que es lo que hace una ISO de verdad.
set -euo pipefail

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/plymouth-preview"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/plymouth-preview"
SYS_CONF="/etc/plymouth/plymouthd.conf"

say()  { printf '  %s\n' "$*"; }
step() { printf '\n\033[1;38;2;235;194;186m▸\033[0m \033[1m%s\033[0m\n' "$*"; }
ok()   { printf '  \033[38;2;125;207;216m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[38;2;246;153;150m! %s\033[0m\n' "$*"; }

usage() {
  sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
}

# ── El tema activo del sistema ────────────────────────────────────────────
system_theme() {
  sed -n 's/^[[:space:]]*Theme[[:space:]]*=[[:space:]]*\(.*\)/\1/p' "$SYS_CONF" 2>/dev/null | head -1
}

# ── El tema del preview (override del usuario) ────────────────────────────
preview_theme() {
  local override="$CONFIG_DIR/theme"
  if [[ -f "$override" ]]; then
    cat "$override"
  else
    system_theme
  fi
}

# ── El kernel y su versión ────────────────────────────────────────────────
kernel_path() {
  local k
  k="$(ls -1 /usr/lib/modules/*/vmlinuz 2>/dev/null | sort -V | tail -1)"
  [[ -n "$k" ]] || { echo "No encuentro ningún kernel en /usr/lib/modules/" >&2; exit 1; }
  printf '%s\n' "$k"
}

# ── Construir el initramfs ────────────────────────────────────────────────
build_initramfs() {
  local theme kver kpath out
  theme="$(preview_theme)"
  [[ -n "$theme" ]] || { echo "No hay tema configurado en $SYS_CONF" >&2; exit 1; }
  [[ -d "/usr/share/plymouth/themes/$theme" ]] || {
    echo "El tema '$theme' no está en /usr/share/plymouth/themes/" >&2; exit 1; }

  kpath="$(kernel_path)"
  kver="$(basename "$(dirname "$kpath")")"
  out="$CACHE_DIR/$theme-$kver.img"

  mkdir -p "$CACHE_DIR"

  # Cache: si el initramfs es más nuevo que el theme, no se regenera.
  if [[ -f "$out" && "$out" -nt "/usr/share/plymouth/themes/$theme" ]]; then
    printf '%s\n' "$out"
    return 0
  fi

  step "Generando el initramfs ($theme, kernel $kver)"

  # bochs es el driver de la VGA de QEMU (-vga std). Sin él el framebuffer
  # queda en modo texto y Plymouth no tiene dónde dibujar.
  local conf="$CACHE_DIR/mkinitcpio.conf"
  cat > "$conf" <<EOF
MODULES=(bochs)
BINARIES=()
FILES=()
HOOKS=(base udev plymouth modconf block filesystems)
COMPRESSION="zstd"
EOF

  sudo mkinitcpio -c "$conf" -g "$out" -k "$kver" >/dev/null 2>&1
  sudo chmod 644 "$out"
  ok "$out ($(du -h "$out" | cut -f1))"
  printf '%s\n' "$out"
}

# ── Arrancar QEMU ─────────────────────────────────────────────────────────
# $1 = initramfs, $2 = 1 si es GUI, $3 = socket del monitor
start_qemu() {
  local initrd="$1" gui="$2" mon="$3"
  local kpath
  kpath="$(kernel_path)"

  local args=(
    -enable-kvm -m 1024 -cpu host
    -kernel "$kpath"
    -initrd "$initrd"
    -append "root=/dev/nonexistent splash quiet"
    -vga std
    -monitor "unix:$mon,server,nowait"
    -no-reboot
  )

  if [[ "$gui" == "1" ]]; then
    args+=(-display gtk)
  else
    args+=(-display none)
  fi

  qemu-system-x86_64 "${args[@]}" >/dev/null 2>&1 &
  echo $!
}

# ── Capturar el mejor frame ───────────────────────────────────────────────
cmd_shot() {
  local out="${1:-}"
  local mon="/tmp/plymouth-preview-mon.$$"
  # El trap de EXIT no ve las variables `local`: al salir ya no existen y
  # con `set -u` eso es un "unbound variable". Se usan globales con prefijo.
  PP_TMPDIR="$(mktemp -d /tmp/plymouth-preview.XXXXXX)"
  PP_MON="$mon"
  trap 'rm -rf "${PP_TMPDIR:-}" "${PP_MON:-}"' EXIT
  local tmpdir="$PP_TMPDIR"

  local initrd
  initrd="$(build_initramfs | tail -1)"

  step "Arrancando QEMU y muestreando el framebuffer"
  local pid
  pid="$(start_qemu "$initrd" 0 "$mon")"

  local best_size=0 best_frame=""
  local i=0
  # 60 muestras * 0.4s = 24s de ventana
  while [[ $i -lt 60 ]]; do
    sleep 0.4
    i=$((i+1))
    local raw="$tmpdir/shot-$i.ppm"
    echo "screendump $raw" | socat - "UNIX-CONNECT:$mon" >/dev/null 2>&1 || continue
    [[ -s "$raw" ]] || continue
    local png="$tmpdir/shot-$i.png"
    if command -v vips >/dev/null 2>&1; then
      vips copy "$raw" "$png" >/dev/null 2>&1 || continue
    else
      magick "$raw" "$png" 2>/dev/null || continue
    fi
    # Un frame con contenido pesa mucho más que uno negro.
    local size
    size="$(stat -c %s "$png" 2>/dev/null || echo 0)"
    if [[ "$size" -gt "$best_size" ]]; then
      best_size="$size"
      best_frame="$png"
    fi
  done

  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true

  if [[ -z "$best_frame" || "$best_size" -lt 2000 ]]; then
    echo "No se pudo capturar el splash (el initramfs cayó a la shell de emergencia)." >&2
    echo "Ver la sección LIMITACIÓN CONOCIDA en $(basename "$0")" >&2
    exit 1
  fi

  if [[ -z "$out" ]]; then
    out="/tmp/plymouth-preview-$(date +%Y%m%d-%H%M%S).png"
  fi
  cp "$best_frame" "$out"
  echo "Captura: $out  ($(du -h "$out" | cut -f1))"
}

# ── Ver en ventana ────────────────────────────────────────────────────────
cmd_view() {
  local initrd
  initrd="$(build_initramfs | tail -1)"

  step "Abriendo QEMU"
  warn "El splash dura unos segundos: el initramfs no tiene root y cae a"
  warn "la shell de emergencia. Cerrá la ventana con Ctrl+Q."

  local mon="/tmp/plymouth-preview-mon.$$"
  local pid
  pid="$(start_qemu "$initrd" 1 "$mon")"
  wait "$pid" 2>/dev/null || true
  rm -f "$mon"
}

# ── Temas ─────────────────────────────────────────────────────────────────
cmd_themes() {
  local current
  current="$(preview_theme)"
  step "Temas instalados"
  for d in /usr/share/plymouth/themes/*/; do
    local name
    name="$(basename "$d")"
    if [[ "$name" == "$current" ]]; then
      printf '  \033[38;2;125;207;216m* %s\033[0m  (activo)\n' "$name"
    else
      printf '    %s\n' "$name"
    fi
  done
}

cmd_theme() {
  local name="${1:-}"
  [[ -n "$name" ]] || { echo "Falta el nombre del tema" >&2; cmd_themes; exit 1; }
  [[ -d "/usr/share/plymouth/themes/$name" ]] || {
    echo "No existe el tema: $name" >&2; exit 1; }
  mkdir -p "$CONFIG_DIR"
  printf '%s\n' "$name" > "$CONFIG_DIR/theme"
  ok "El preview va a usar '$name' (el sistema sigue con '$(system_theme)')"
  say "Para aplicarlo al sistema: sudo plymouth-set-default-theme -R $name"
}

# ── Root ──────────────────────────────────────────────────────────────────
# mkinitcpio necesita root. El script se re-ejecuta con sudo ANTES del
# dispatch (ver abajo), para no perder el comando y sus argumentos.
# Si corremos por sudo, HOME es /root: se usa el HOME del usuario real, que
# es donde están las preferencias y el cache.
use_real_home() {
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    local uh
    uh="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
    export HOME="$uh"
    CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/plymouth-preview"
    CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/plymouth-preview"
  fi
}

# Re-ejecuta con sudo conservando el comando original.
need_root() {
  if [[ "$(id -u)" != "0" ]]; then
    exec sudo --preserve-env=XDG_CACHE_HOME,XDG_CONFIG_HOME "$0" "$@"
  fi
}

# ── Dispatch ──────────────────────────────────────────────────────────────
# Los comandos que construyen el initramfs necesitan root: se re-ejecuta
# antes de tocar los argumentos.
case "${1:-view}" in
  view|shot|build|"") need_root "$@" ;;
esac
use_real_home

cmd="${1:-view}"
shift || true

case "$cmd" in
  view|"")   cmd_view ;;
  shot)      cmd_shot "${1:-}" ;;
  themes)    cmd_themes ;;
  theme)     cmd_theme "${1:-}" ;;
  build)
     step "Construyendo"
     build_initramfs
     ;;
  help|-h|--help) usage ;;
  *)
     echo "Comando desconocido: $cmd" >&2
     echo >&2
     usage
     exit 1
     ;;
esac
