#!/usr/bin/env bash
# plymouth-preview — ver el splash de Plymouth sin reiniciar la máquina.
#
# Construye un initramfs con el theme activo del sistema y lo arranca en QEMU.
# El splash lo dibuja Plymouth de verdad: mismo tema, mismos frames, mismo
# renderer (DRM).
#
#   plymouth-preview                    # ventana con el splash
#   plymouth-preview shot [f.png]       # captura el splash y sale
#   plymouth-preview themes             # lista los temas instalados
#   plymouth-preview theme <nom>        # cambia el tema DEL PREVIEW
#   plymouth-preview use <nom>          # aplica un tema AL SISTEMA (-R)
#   plymouth-preview sources            # videos de animación que tiene Steam
#   plymouth-preview from-steam         # convierte el video de Steam en tema
#   plymouth-preview from-steam --apply # ...y lo aplica al sistema
#   plymouth-preview build              # solo regenera el initramfs (cache)
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

  # El nombre lleva un hash de los HOOKS: si se cambia el orden o se agrega
  # uno, el initramfs viejo queda obsoleto y hay que regenerarlo (si no, se
  # reusa uno con los hooks viejos y el preview muestra el tema equivocado).
  local hooks_line="base udev modconf block preview plymouth filesystems"
  local tag
  tag="$(printf '%s' "$hooks_line" | md5sum | cut -c1-8)"
  out="$CACHE_DIR/$theme-$kver-$tag.img"

  mkdir -p "$CACHE_DIR"

  # El nombre del tema va al initramfs (/preview-theme). El hook `preview`
  # lo lee en runtime y escribe /etc/plymouth/plymouthd.conf ANTES de que el
  # hook `plymouth` lance el daemon. Escribir el conf directo con add_file no
  # sirve: el hook `plymouth` agrega el suyo y gana.
  printf '%s\n' "$theme" > /tmp/plymouth-preview-theme

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
HOOKS=($hooks_line)
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
  # Frames con contenido, ordenados por peso (los negros pesan ~1.5 KB).
  local -a candidatos=()
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
    if [[ "$size" -gt 2000 ]]; then
      candidatos+=("$size:$png")
    fi
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

  # Contact sheet: los 4 frames con contenido más distintos entre sí.
  # `shot` se queda con el frame más pesado (suele ser el final de la
  # animación); el sheet muestra cómo evoluciona.
  if [[ ${#candidatos[@]} -ge 4 ]] && command -v magick >/dev/null 2>&1; then
    local sheet="${out%.png}-sheet.png"
    local -a top=()
    while IFS= read -r linea; do
      top+=("${linea#*:}")
    done < <(printf '%s\n' "${candidatos[@]}" | sort -t: -k1 -rn | head -4)
    magick "${top[@]}" -resize 640x360 +append -append "$sheet" 2>/dev/null && \
      echo "Sheet:   $sheet  ($(du -h "$sheet" | cut -f1))"
  fi
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

# ── Steam ─────────────────────────────────────────────────────────────────
# Steam Big Picture usa `bigpicture_startup.webm` (empaquetado, 1920x1200,
# 60 fps, 6 s). En Steam Deck hay además deck_startup.webm y oled_startup.webm.
# No hay forma de "elegir" el video desde Steam en desktop: es el que trae.
steam_dirs() {
  local d real vistos=""
  for d in \
    "$HOME/.local/share/Steam/steamui/movies" \
    "$HOME/.steam/steam/steamui/movies" \
    "$HOME/.steam/steam/legacycompat"; do
    [[ -d "$d" ]] || continue
    # ~/.steam/steam y ~/.local/share/Steam son el mismo sitio (symlink):
    # se deduplica por ruta real, no por la ruta escrita.
    # (Sin arrays asociativos: la funcion corre en un subshell con
    # process substitution y `local -A` no sobrevive.)
    real="$(readlink -f "$d" 2>/dev/null || echo "$d")"
    case "$vistos" in *"|$real|"*) continue ;; esac
    vistos="$vistos|$real|"
    printf '%s\n' "$d"
  done
}

# El video que Steam usa para la animación de entrada a Big Picture.
steam_video() {
  local dir video
  dir="$(steam_dirs | head -1)"
  [[ -n "$dir" ]] || { echo "No encuentro el directorio de movies de Steam" >&2; return 1; }

  # En Steam Deck (Jupiter = LCD, Galileo = OLED) el startup es otro.
  local modelo=""
  [[ -r /sys/devices/virtual/dmi/id/product_name ]] && \
    modelo="$(cat /sys/devices/virtual/dmi/id/product_name)"

  case "$modelo" in
    *Jupiter*) video="deck_startup.webm" ;;
    *Galileo*) video="oled_startup.webm" ;;
    *)         video="bigpicture_startup.webm" ;;
  esac

  if [[ -f "$dir/$video" ]]; then
    printf '%s\n' "$dir/$video"
    return 0
  fi
  # Fallback: el primero que haya.
  for video in bigpicture_startup.webm deck_startup.webm oled_startup.webm; do
    [[ -f "$dir/$video" ]] && { printf '%s\n' "$dir/$video"; return 0; }
  done
  echo "No encuentro ningún video de startup de Steam" >&2
  return 1
}

cmd_sources() {
  local dir actual
  actual="$(steam_video 2>/dev/null || true)"
  step "Videos de animación que tiene Steam"
  while IFS= read -r dir; do
    [[ -d "$dir" ]] || continue
    say "$dir"
    for f in "$dir"/*startup*.webm "$dir"/*suspend*.webm; do
      [[ -f "$f" ]] || continue
      local info
      info="$(ffprobe -v error -select_streams v:0 \
        -show_entries stream=width,height,r_frame_rate -show_entries format=duration \
        -of csv=p=0 "$f" 2>/dev/null | tr '\n' ' ')"
      if [[ "$f" == "$actual" ]]; then
        printf '  \033[38;2;125;207;216m* %s\033[0m  (%s) ← el activo\n' "$(basename "$f")" "$info"
      else
        printf '    %s  (%s)\n' "$(basename "$f")" "$info"
      fi
    done
  done < <(steam_dirs)

  [[ -n "$actual" ]] && { echo; say "Activo: $actual"; }
}

# Convierte un video en un tema de Plymouth (two-step + los frames).
convert_video_to_theme() {
  local video="$1" name="$2" fps="$3" res="$4" dest
  dest="/usr/share/plymouth/themes/$name"

  [[ -f "$video" ]] || { echo "No existe el video: $video" >&2; return 1; }
  command -v ffmpeg >/dev/null 2>&1 || { echo "FALTA: ffmpeg" >&2; return 1; }

  local tmp
  tmp="$(mktemp -d /tmp/plymouth-theme.XXXXXX)"

  step "Extrayendo frames ($(basename "$video"), ${fps}fps, $res)"
  # crop centrado al aspecto pedido, escalado Lanczos. Se descarta el audio.
  local w h
  w="${res%x*}"; h="${res#*x}"
  ffmpeg -loglevel error -y -i "$video" \
    -vf "fps=$fps,crop=iw:min(ih\\,iw*$h/$w):0:(ih-min(ih\\,iw*$h/$w))/2,scale=$w:$h:flags=lanczos" \
    -start_number 0 "$tmp/animation-%03d.png" 2>/dev/null

  local frames
  frames="$(ls "$tmp"/animation-*.png 2>/dev/null | wc -l)"
  [[ "$frames" -gt 0 ]] || { echo "No se pudo extraer ningún frame" >&2; rm -rf "$tmp"; return 1; }
  ok "$frames frames"

  # Throbber: se reusan los del tema base si existen (el spinner de progreso).
  local base="/usr/share/plymouth/themes/cachyos-bootanimation"
  if [[ -d "$base" ]]; then
    cp "$base"/throbber-*.png "$tmp"/ 2>/dev/null || true
  fi

  cat > "$tmp/$name.plymouth" <<EOF
[Plymouth Theme]
Name=$name
Description=Generado desde $(basename "$video")
ModuleName=two-step

[two-step]
ImageDir=$dest
HorizontalAlignment=.5
VerticalAlignment=.5
Transition=none
TransitionDuration=0.0
BackgroundStartColor=0x000000
BackgroundEndColor=0x000000
ProgressBarBackgroundColor=0x1a1a1a
ProgressBarForegroundColor=0x6ba8d9
MessageBelowAnimation=false
EOF

  step "Instalando en $dest"
  sudo rm -rf "$dest"
  sudo mkdir -p "$dest"
  sudo cp "$tmp"/animation-*.png "$dest"/
  sudo cp "$tmp"/throbber-*.png "$dest"/ 2>/dev/null || true
  sudo cp "$tmp/$name.plymouth" "$dest"/
  rm -rf "$tmp"

  ok "Tema '$name' instalado ($(du -sh "$dest" 2>/dev/null | cut -f1))"
  printf '%s\n' "$name"
}

cmd_from_steam() {
  local video="" name="" fps=30 res="1920x1080" apply=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --video)  video="$2"; shift 2 ;;
      --name)   name="$2"; shift 2 ;;
      --fps)    fps="$2"; shift 2 ;;
      --res)    res="$2"; shift 2 ;;
      --apply)  apply=1; shift ;;
      *) echo "Opción desconocida: $1" >&2; exit 1 ;;
    esac
  done

  [[ -n "$video" ]] || video="$(steam_video)"
  [[ -f "$video" ]] || { echo "No encuentro el video" >&2; exit 1; }

  # Nombre por defecto: steam-<nombre del video sin extensión>
  if [[ -z "$name" ]]; then
    name="steam-$(basename "$video" .webm | tr '_' '-')"
  fi

  local created
  created="$(convert_video_to_theme "$video" "$name" "$fps" "$res" | tail -1)"

  if [[ "$apply" == "1" ]]; then
    cmd_use "$created"
  else
    echo
    say "Para verlo en el preview:  plymouth-preview theme $created"
    say "Para aplicarlo al sistema: plymouth-preview use $created"
  fi
}

# Aplica un tema al SISTEMA (no al preview).
cmd_use() {
  local name="${1:-}"
  [[ -n "$name" ]] || { echo "Falta el nombre del tema" >&2; exit 1; }
  [[ -d "/usr/share/plymouth/themes/$name" ]] || {
    echo "No existe el tema: $name" >&2; cmd_themes; exit 1; }

  step "Aplicando '$name' al sistema"
  warn "Esto regenera el initramfs real (~30s) y cambia el splash del próximo boot."
  sudo plymouth-set-default-theme -R "$name"
  ok "Tema activo: $(system_theme)"
  say "El próximo arranque va a mostrar '$name'."
  say "Para volver: plymouth-preview use $(echo "$name" | sed 's/^steam-.*/cachyos-bootanimation/')"
}

# ── Dispatch ──────────────────────────────────────────────────────────────
# Los comandos que construyen el initramfs necesitan root: se re-ejecuta
# antes de tocar los argumentos.
case "${1:-view}" in
  view|shot|build|from-steam|use|"") need_root "$@" ;;
esac
use_real_home

cmd="${1:-view}"
shift || true

case "$cmd" in
  view|"")   cmd_view ;;
  shot)      cmd_shot "${1:-}" ;;
  themes)    cmd_themes ;;
  theme)     cmd_theme "${1:-}" ;;
  use)       cmd_use "${1:-}" ;;
  sources)   cmd_sources ;;
  from-steam) use_root "$@"; cmd_from_steam "$@" ;;
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
