#!/usr/bin/env bash
# plymouth-preview — ver y convertir splashes de Plymouth sin reiniciar.
#
# Por debajo usa el Plymouth OFICIAL (plymouthd + los plugins del sistema)
# dentro de una VM de QEMU con KVM. No es un reproductor de imágenes: es el
# mismo binario y el mismo renderer (DRM) que usa la máquina al arrancar.
#
#   plymouth-preview                     # ventana con el splash (VM)
#   plymouth-preview x11                 # splash en una ventana X11 (SIN VM)
#   plymouth-preview shell               # shell interactiva dentro de la VM
#   plymouth-preview shot [f.png]        # captura el splash y sale
#   plymouth-preview sources             # temas de Plymouth instalados
#   plymouth-preview preview <tema>      # tema SOLO del preview
#   plymouth-preview use <tema>          # tema del SISTEMA (-R, regenera initramfs)
#   plymouth-preview steam-source        # videos de Steam (cuál está activo)
#   plymouth-preview steam-convert <vid> # convierte un video en tema
#   plymouth-preview steam-apply <vid>   # convierte y lo aplica al sistema
#   plymouth-preview build               # regenera el initramfs (cache)
#   plymouth-preview help
#
# LIMITACIÓN CONOCIDA
# -------------------
# El initramfs no tiene un sistema de archivos raíz. Cuando el init de
# mkinitcpio no puede montar el root, lanza la shell de emergencia y el
# splash desaparece. Hay una race con la inicialización del DRM: el splash
# puede verse desde ~1 s hasta ~15 s, de forma inconsistente.
#
# `shot` lo maneja muestreando el framebuffer cada 400 ms y quedándose con
# el frame de más contenido.
set -euo pipefail

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/plymouth-preview"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/plymouth-preview"
SYS_CONF="/etc/plymouth/plymouthd.conf"
THEMES_DIR="/usr/share/plymouth/themes"
THEME_FILE="/tmp/plymouth-preview-theme"
HOOKS_LINE="base udev modconf block preview plymouth filesystems"

say()  { printf '  %s\n' "$*"; }
step() { printf '\n\033[1;38;2;235;194;186m▸\033[0m \033[1m%s\033[0m\n' "$*"; }
ok()   { printf '  \033[38;2;125;207;216m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[38;2;246;153;150m! %s\033[0m\n' "$*"; }

usage() {
  sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
}

# ── HOME real cuando se corre con sudo ────────────────────────────────────
use_real_home() {
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    local uh
    uh="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
    export HOME="$uh"
    CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/plymouth-preview"
    CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/plymouth-preview"
  fi
}

# ── Root ──────────────────────────────────────────────────────────────────
# mkinitcpio necesita root. Se conservan las variables del entorno gráfico:
# sin WAYLAND_DISPLAY (o DISPLAY) QEMU no puede abrir la ventana, y el
# síntoma es que `plymouth-preview` sale sin hacer nada.
need_root() {
  if [[ "$(id -u)" != "0" ]]; then
    exec sudo --preserve-env=HOME,XDG_CACHE_HOME,XDG_CONFIG_HOME,DISPLAY,WAYLAND_DISPLAY,XDG_RUNTIME_DIR,XAUTHORITY,TERM,STEAM_FPS,STEAM_RES \
      "$0" "$@"
  fi
}

# ── Tema del sistema / del preview ────────────────────────────────────────
system_theme() {
  sed -n 's/^[[:space:]]*Theme[[:space:]]*=[[:space:]]*\(.*\)/\1/p' "$SYS_CONF" 2>/dev/null | head -1
}

preview_theme() {
  if [[ -f "$CONFIG_DIR/theme" ]]; then
    cat "$CONFIG_DIR/theme"
  else
    system_theme
  fi
}

# ── Kernel ────────────────────────────────────────────────────────────────
kernel_path() {
  local k
  k="$(ls -1 /usr/lib/modules/*/vmlinuz 2>/dev/null | sort -V | tail -1)"
  [[ -n "$k" ]] || { echo "No encuentro ningún kernel en /usr/lib/modules/" >&2; exit 1; }
  printf '%s\n' "$k"
}

# ── Construir el initramfs ────────────────────────────────────────────────
build_initramfs() {
  local theme kver kpath out tag conf
  theme="$(preview_theme)"
  [[ -n "$theme" ]] || { echo "No hay tema configurado en $SYS_CONF" >&2; exit 1; }
  [[ -d "$THEMES_DIR/$theme" ]] || {
    echo "El tema '$theme' no está en $THEMES_DIR/" >&2; exit 1; }

  kpath="$(kernel_path)"
  kver="$(basename "$(dirname "$kpath")")"

  # El nombre lleva un hash de los HOOKS: si cambia el orden o se agrega uno,
  # el initramfs viejo queda obsoleto (si no, se reusa uno con los hooks
  # viejos y el preview muestra el tema equivocado).
  tag="$(printf '%s' "$HOOKS_LINE" | md5sum | cut -c1-8)"
  out="$CACHE_DIR/$theme-$kver-$tag.img"

  mkdir -p "$CACHE_DIR"

  # El hook `preview` lee este archivo en runtime y escribe
  # /etc/plymouth/plymouthd.conf ANTES de que el hook `plymouth` arranque el
  # daemon. Escribirlo con add_file no sirve: el hook `plymouth` agrega el
  # suyo y gana.
  printf '%s\n' "$theme" > "$THEME_FILE"

  # Cache: si el initramfs es más nuevo que el tema, no se regenera.
  if [[ -f "$out" && "$out" -nt "$THEMES_DIR/$theme" ]]; then
    printf '%s\n' "$out"
    return 0
  fi

  step "Generando el initramfs ($theme, kernel $kver)"
  conf="$CACHE_DIR/mkinitcpio.conf"
  # El hook `plymouth` de mkinitcpio solo incluye el tema ACTIVO del sistema
  # + un par de básicos. El hook `preview` (install) agrega el tema del
  # preview archivo por archivo: `FILES=(directorio)` y `-I directorio` NO
  # sirven, mkinitcpio crea el directorio pero no copia el contenido.
  cat > "$conf" <<EOF
MODULES=(bochs)
BINARIES=()
FILES=()
HOOKS=($HOOKS_LINE)
COMPRESSION="zstd"
EOF

  sudo mkinitcpio -c "$conf" -g "$out" -k "$kver" >/dev/null 2>&1
  sudo chmod 644 "$out"
  ok "$out ($(du -h "$out" | cut -f1))"
  printf '%s\n' "$out"
}

# ── QEMU ──────────────────────────────────────────────────────────────────
# $1 = initramfs, $2 = 1 si es GUI, $3 = socket del monitor
start_qemu() {
  local initrd="$1" gui="$2" mon="$3" kpath
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

  # Con GUI, QEMU NO puede correr como root: en Wayland no accede al socket
  # del usuario aunque se preserven las variables. Se baja al usuario real.
  # Sin GUI (-display none) da igual y se evita el `sudo -u`, que rompía el
  # arranque (el monitor nunca se creaba y el screendump fallaba en silencio).
  if [[ "$gui" == "1" && -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    sudo -u "$SUDO_USER" --preserve-env=DISPLAY,WAYLAND_DISPLAY,XDG_RUNTIME_DIR,XAUTHORITY \
      qemu-system-x86_64 "${args[@]}" >/dev/null 2>&1 &
  else
    qemu-system-x86_64 "${args[@]}" >/dev/null 2>&1 &
  fi
  echo $!
}

# ── Ver en ventana ────────────────────────────────────────────────────────
cmd_view() {
  if [[ -z "${WAYLAND_DISPLAY:-}${DISPLAY:-}" ]]; then
    warn "No hay entorno gráfico (WAYLAND_DISPLAY y DISPLAY vacíos)."
    say "Si estás en una terminal sin display, usá: plymouth-preview shot"
    exit 1
  fi

  local initrd mon pid
  initrd="$(build_initramfs | tail -1)"
  mon="/tmp/plymouth-preview-mon.$$"

  step "Abriendo QEMU"
  warn "El splash dura unos segundos: después cae a la shell de emergencia."
  warn "Cerrá la ventana con Ctrl+Q o esperá a que termine."

  pid="$(start_qemu "$initrd" 1 "$mon")"
  wait "$pid" 2>/dev/null || true
  rm -f "$mon"
}

# ── Capturar ──────────────────────────────────────────────────────────────
cmd_shot() {
  local out="${1:-}"
  local mon="/tmp/plymouth-preview-mon.$$"

  # El trap de EXIT no ve variables `local`: se usan globales con prefijo.
  PP_TMPDIR="$(mktemp -d /tmp/plymouth-preview.XXXXXX)"
  PP_MON="$mon"
  trap 'rm -rf "${PP_TMPDIR:-}" "${PP_MON:-}"' EXIT
  local tmpdir="$PP_TMPDIR"

  local initrd
  initrd="$(build_initramfs | tail -1)"

  local best_size=0 best_frame=""
  local -a candidatos=()

  # Hasta 3 intentos: hay una race con la inicialización del DRM y a veces
  # el splash no llega a dibujarse antes de que caiga a la shell.
  local intento
  for intento in 1 2 3; do
    step "Arrancando QEMU y muestreando el framebuffer (intento $intento/3)"
    best_size=0; best_frame=""; candidatos=()
    mon="/tmp/plymouth-preview-mon.$$-$intento"
    PP_MON="$mon"
    local pid
    pid="$(start_qemu "$initrd" 0 "$mon")"

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
    # Un frame negro pesa ~1.5 KB; uno con contenido, 16-25 KB.
    local size
    size="$(stat -c %s "$png" 2>/dev/null || echo 0)"
    if [[ "$size" -gt 2000 ]]; then
      candidatos+=("$i:$png")
    fi
    if [[ "$size" -gt "$best_size" ]]; then
      best_size="$size"
      best_frame="$png"
    fi
  done

  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true

    [[ -n "$best_frame" && "$best_size" -ge 2000 ]] && break
  done

  if [[ -z "$best_frame" || "$best_size" -lt 2000 ]]; then
    echo "No se pudo capturar el splash después de 3 intentos." >&2
    echo "Ver la sección LIMITACIÓN CONOCIDA en $(basename "$0")" >&2
    exit 1
  fi

  [[ -n "$out" ]] || out="/tmp/plymouth-preview-$(date +%Y%m%d-%H%M%S).png"
  cp "$best_frame" "$out"
  echo "Captura: $out  ($(du -h "$out" | cut -f1))"

  # Contact sheet con 4 frames DISTRIBUIDOS en el tiempo (no los más
  # pesados): así se ve cómo evoluciona la animación, no solo el final.
  if [[ ${#candidatos[@]} -ge 4 ]] && command -v magick >/dev/null 2>&1; then
    local sheet="${out%.png}-sheet.png"
    local total=${#candidatos[@]}
    local -a top=()
    for pos in 0 $((total / 3)) $((2 * total / 3)) $((total - 1)); do
      top+=("${candidatos[$pos]#*:}")
    done
    magick "${top[@]}" -resize 640x360 +append -append "$sheet" 2>/dev/null && \
      echo "Sheet:   $sheet  ($(du -h "$sheet" | cut -f1))"
  fi
}

# ── Temas de Plymouth instalados ──────────────────────────────────────────
cmd_sources() {
  local actual preview
  actual="$(system_theme)"
  preview="$(preview_theme)"

  step "Temas de Plymouth instalados"
  say "directorio: $THEMES_DIR/"
  echo
  for d in "$THEMES_DIR"/*/; do
    local name module marca=""
    name="$(basename "$d")"
    module="$(grep -h '^ModuleName' "$d"/*.plymouth 2>/dev/null | head -1 | cut -d= -f2)"
    [[ "$name" == "$actual" ]] && marca="${marca} \033[38;2;166;227;161m[sistema]\033[0m"
    [[ "$name" == "$preview" ]] && marca="${marca} \033[38;2;125;207;216m[preview]\033[0m"
    printf '    %-30s %-12s%b\n' "$name" "${module:-?}" "$marca"
  done

  echo
  say "sistema: $actual"
  say "preview: $preview"
}

# ── Tema del preview ──────────────────────────────────────────────────────
cmd_preview() {
  local name="${1:-}"
  [[ -n "$name" ]] || { echo "Falta el nombre del tema" >&2; cmd_sources; exit 1; }
  [[ -d "$THEMES_DIR/$name" ]] || {
    echo "No existe el tema: $name" >&2; cmd_sources; exit 1; }
  mkdir -p "$CONFIG_DIR"
  printf '%s\n' "$name" > "$CONFIG_DIR/theme"
  ok "El preview va a usar '$name'"
  say "El sistema sigue con '$(system_theme)'."
  say "Para verlo: plymouth-preview"
}

# ── Tema del sistema ──────────────────────────────────────────────────────
cmd_use() {
  local name="${1:-}"
  [[ -n "$name" ]] || { echo "Falta el nombre del tema" >&2; cmd_sources; exit 1; }
  [[ -d "$THEMES_DIR/$name" ]] || {
    echo "No existe el tema: $name" >&2; cmd_sources; exit 1; }

  step "Aplicando '$name' al SISTEMA"
  warn "Esto regenera el initramfs real (~30 s) y cambia el splash del próximo boot."
  sudo plymouth-set-default-theme -R "$name"
  ok "Tema activo: $(system_theme)"
  say "Para volver al de CachyOS: plymouth-preview use cachyos-bootanimation"
}

# ── Modo X11 (sin VM) ─────────────────────────────────────────────────────
# Plymouth trae el renderer `renderers/x11.so` compilado con GTK3. Plymouth
# elige ese renderer cuando hay DISPLAY y no hay DRM libre, así que se puede
# ver el splash en una ventana normal: sin VM, sin initramfs, sin reboots.
#
# Es la misma técnica que el paquete `plymouth-x11` de Debian/Fedora, que en
# Arch no está empaquetado pero cuyo renderer sí viene con Plymouth.
cmd_x11() {
  local segundos="${1:-8}"
  if [[ -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
    warn "No hay entorno gráfico (DISPLAY y WAYLAND_DISPLAY vacíos)."
    exit 1
  fi

  local theme
  theme="$(preview_theme)"
  [[ -d "$THEMES_DIR/$theme" ]] || { echo "No existe el tema: $theme" >&2; exit 1; }

  step "Plymouth en X11 con el tema '$theme'"

  # Plymouth busca el tema en /run/plymouth/themes antes que en
  # /usr/share/plymouth/themes. Copiarlo ahí permite usar cualquier tema sin
  # tocar el del sistema.
  sudo mkdir -p /run/plymouth/themes
  sudo rm -rf "/run/plymouth/themes/$theme"
  sudo cp -r "$THEMES_DIR/$theme" /run/plymouth/themes/

  # Si ya hay un plymouthd (el del boot), se para.
  sudo plymouth --quit 2>/dev/null || true

  sudo plymouthd --no-daemon --debug --mode=boot \
    --debug-file=/tmp/plymouth-x11.log >/dev/null 2>&1 &
  local pid=$!
  sleep 2
  sudo plymouth --show-splash 2>/dev/null

  say "Mostrando $segundos s. Ctrl+C para cortar."
  local i=0
  while [[ $i -lt "$segundos" ]]; do
    sleep 1
    i=$((i+1))
    sudo plymouth --update="Iniciando $i" 2>/dev/null || true
  done

  sudo plymouth --quit 2>/dev/null || true
  kill "$pid" 2>/dev/null || true
  sudo rm -rf "/run/plymouth/themes/$theme"
  ok "Listo ($(grep -c 'renderer' /tmp/plymouth-x11.log 2>/dev/null || echo 0) líneas de log en /tmp/plymouth-x11.log)"
}

# ── Shell interactiva dentro de la VM ─────────────────────────────────────
# Abre la consola serie de la VM en la terminal: ahí se puede escribir
# `plymouth --show-splash`, `plymouth --update=...`, `plymouth --quit`, etc.
# Es la shell de emergencia del initramfs (trae busybox).
cmd_shell() {
  local initrd kpath mon
  initrd="$(build_initramfs | tail -1)"
  kpath="$(kernel_path)"
  mon="/tmp/plymouth-preview-mon.$$"

  step "Shell interactiva de la VM"
  say "Comandos útiles dentro:"
  say "  plymouth --show-splash      # relanzar la animación"
  say "  plymouth --update=hola      # cambiar el mensaje"
  say "  plymouth --quit             # cerrar el splash"
  say "  ls /run/plymouth            # estado del daemon"
  say "  reboot -f                   # apagar la VM"
  echo
  say "Salir de QEMU: Ctrl+A, después X"

  # -serial stdio: la consola de la VM va a esta terminal (interactiva).
  # -display none: no hace falta ventana; el splash no se ve, pero se puede
  # pedir un screendump desde el monitor... o usar x11 para verlo.
  qemu-system-x86_64 \
    -enable-kvm -m 512 -cpu host \
    -kernel "$kpath" -initrd "$initrd" \
    -append "root=/dev/nonexistent splash quiet console=ttyS0" \
    -vga std -display none \
    -serial stdio \
    -monitor "unix:$mon,server,nowait" \
    -no-reboot
  rm -f "$mon"
}

# ── Steam: qué videos hay ─────────────────────────────────────────────────
steam_dirs() {
  local d real vistos=""
  for d in \
    "$HOME/.local/share/Steam/steamui/movies" \
    "$HOME/.steam/steam/steamui/movies" \
    "$HOME/.steam/steam/legacycompat"; do
    [[ -d "$d" ]] || continue
    # ~/.steam/steam y ~/.local/share/Steam son el mismo sitio (symlink).
    real="$(readlink -f "$d" 2>/dev/null || echo "$d")"
    case "$vistos" in *"|$real|"*) continue ;; esac
    vistos="$vistos|$real|"
    printf '%s\n' "$d"
  done
}

# La animación de inicio ACTIVA que eligió el usuario en Steam.
#
# Steam guarda la selección en ~/.steam/steam/config/config.vdf:
#   "StartupMovie" { "MovieID" "31391441102" "LocalPath" "/communityitemscache/..." }
#
# Las que se compran en la tienda viven en
# config/communityitemscache/startupmovies/<item_id>_<hash>.webm.
steam_selected_movie() {
  local cfg f
  for cfg in "$HOME/.steam/steam/config/config.vdf" \
             "$HOME/.local/share/Steam/config/config.vdf"; do
    [[ -f "$cfg" ]] || continue
    f="$(awk '
      /"StartupMovie"/ { en = 1 }
      en && /"LocalPath"/ {
        sub(/.*"LocalPath"[ \t]*"/, "")
        sub(/".*/, "")
        print
        exit
      }
    ' "$cfg" 2>/dev/null)"
    # OJO: el LocalPath de Steam es "/communityitemscache/..." pero el
    # archivo vive en "<steam>/config/communityitemscache/...": falta el
    # segmento "config".
    local base cand
    for base in "$HOME/.local/share/Steam" "$HOME/.steam/steam"; do
      for cand in "$base/config$f" "$base$f"; do
        if [[ -n "$f" && -f "$cand" ]]; then
          printf '%s\n' "$cand"
          return 0
        fi
      done
    done
  done
  return 1
}

steam_video() {
  # 1) La que el usuario eligió en Steam (comprada en la tienda o del Deck).
  local sel
  sel="$(steam_selected_movie 2>/dev/null || true)"
  if [[ -n "$sel" && -f "$sel" ]]; then
    printf '%s\n' "$sel"; return 0
  fi

  # 2) Si no hay selección, la de Big Picture del cliente.
  local dir video modelo=""
  dir="$(steam_dirs | head -1)"
  [[ -n "$dir" ]] || { echo "No encuentro el directorio de movies de Steam" >&2; return 1; }

  [[ -r /sys/devices/virtual/dmi/id/product_name ]] && \
    modelo="$(cat /sys/devices/virtual/dmi/id/product_name)"

  case "$modelo" in
    *Jupiter*) video="deck_startup.webm" ;;
    *Galileo*) video="oled_startup.webm" ;;
    *)         video="bigpicture_startup.webm" ;;
  esac

  if [[ -f "$dir/$video" ]]; then
    printf '%s\n' "$dir/$video"; return 0
  fi
  for video in bigpicture_startup.webm deck_startup.webm oled_startup.webm; do
    [[ -f "$dir/$video" ]] && { printf '%s\n' "$dir/$video"; return 0; }
  done
  echo "No encuentro ningún video de startup de Steam" >&2
  return 1
}

cmd_steam_source() {
  local actual
  actual="$(steam_video 2>/dev/null || true)"

  step "Animaciones de inicio de Steam"

  # Las compradas en la tienda: Steam las guarda en
  # config/communityitemscache/startupmovies/ y el nombre real está en el
  # JSON "GetStartupMovies" de localconfig.vdf.
  python3 - "$HOME" "$actual" <<'PY'
import json, os, re, sys, subprocess

home, actual = sys.argv[1], sys.argv[2]

# 1) El JSON con los nombres
nombres = {}
for base in (f"{home}/.steam/steam", f"{home}/.local/share/Steam"):
    lc = f"{base}/userdata"
    if not os.path.isdir(lc):
        continue
    for uid in os.listdir(lc):
        f = f"{lc}/{uid}/config/localconfig.vdf"
        if not os.path.isfile(f):
            continue
        txt = open(f, encoding="utf-8", errors="replace").read()
        m = re.search(r'"GetStartupMovies"\s+"(.*?)"\s*$', txt, re.M)
        if not m:
            continue
        raw = m.group(1).replace('\\"', '"').replace("\t", "\t")
        try:
            for item in json.loads(raw):
                iid = item.get("communityitemid")
                if iid:
                    nombres[iid] = item.get("name") or item.get("item_title") or iid
        except Exception:
            pass

# 2) Los archivos descargados
vistos = set()
print("  Compradas en la tienda:")
for base in (f"{home}/.local/share/Steam", f"{home}/.steam/steam"):
    d = f"{base}/config/communityitemscache/startupmovies"
    if not os.path.isdir(d):
        continue
    for f in sorted(os.listdir(d)):
        if not f.endswith(".webm"):
            continue
        ruta = f"{d}/{f}"
        if os.path.realpath(ruta) in vistos:
            continue
        vistos.add(os.path.realpath(ruta))
        iid = f.split("_")[0]
        nombre = nombres.get(iid, iid)
        mark = "  \033[38;2;125;207;216m*\033[0m" if os.path.realpath(ruta) == os.path.realpath(actual) else "   "
        if mark != "   ":
            print(f"{mark} {nombre:32s} ({iid})  ← la activa")
        else:
            print(f"{mark} {nombre:32s} ({iid})")

# 3) Las que trae el cliente
print()
print("  Del cliente (Steam Big Picture / Deck):")
for base in (f"{home}/.local/share/Steam", f"{home}/.steam/steam"):
    d = f"{base}/steamui/movies"
    if not os.path.isdir(d):
        continue
    for f in sorted(os.listdir(d)):
        if not f.endswith(".webm") or "startup" not in f:
            continue
        ruta = f"{d}/{f}"
        if os.path.realpath(ruta) in vistos:
            continue
        vistos.add(os.path.realpath(ruta))
        mark = "  \033[38;2;125;207;216m*\033[0m" if os.path.realpath(ruta) == os.path.realpath(actual) else "   "
        sufijo = "  ← la activa" if mark != "   " else ""
        print(f"{mark} {f:44s}{sufijo}")
    break
PY

  echo
  say "Activa: ${actual:-ninguna}"
  say "Steam la guarda en config/communityitemscache/startupmovies/"
}

# Nombre legible de una animación de la tienda.
# Los archivos se llaman <item_id>_<hash>.webm; el nombre real ("Elden Ring
# Ranni") está en el JSON GetStartupMovies de localconfig.vdf.
steam_item_name() {
  local iid="$1"
  python3 - "$HOME" "$iid" <<'PY' 2>/dev/null
import json, os, re, sys
home, iid = sys.argv[1], sys.argv[2]
for base in (f"{home}/.steam/steam", f"{home}/.local/share/Steam"):
    lc = f"{base}/userdata"
    if not os.path.isdir(lc):
        continue
    for uid in os.listdir(lc):
        f = f"{lc}/{uid}/config/localconfig.vdf"
        if not os.path.isfile(f):
            continue
        txt = open(f, encoding="utf-8", errors="replace").read()
        m = re.search(r'"GetStartupMovies"\s+"(.*?)"\s*$', txt, re.M)
        if not m:
            continue
        raw = m.group(1).replace('\\"', '"')
        try:
            for item in json.loads(raw):
                if item.get("communityitemid") == iid:
                    print(item.get("name") or item.get("item_title") or "")
                    sys.exit(0)
        except Exception:
            pass
print("")
PY
}

# ── Steam: convertir ──────────────────────────────────────────────────────
convert_video_to_theme() {
  local video="$1" name="$2" fps="$3" res="$4" dest tmp w h frames base
  dest="$THEMES_DIR/$name"

  [[ -f "$video" ]] || { echo "No existe el video: $video" >&2; return 1; }
  command -v ffmpeg >/dev/null 2>&1 || { echo "FALTA: ffmpeg" >&2; return 1; }

  tmp="$(mktemp -d /tmp/plymouth-theme.XXXXXX)"
  w="${res%x*}"; h="${res#*x}"

  step "Extrayendo frames ($(basename "$video"), ${fps}fps, $res)"
  # Crop centrado al aspecto pedido, escalado Lanczos. Sin audio.
  ffmpeg -loglevel error -y -i "$video" \
    -vf "fps=$fps,crop=iw:min(ih\,iw*$h/$w):0:(ih-min(ih\,iw*$h/$w))/2,scale=$w:$h:flags=lanczos" \
    -start_number 0 "$tmp/animation-%03d.png" 2>/dev/null

  frames="$(ls "$tmp"/animation-*.png 2>/dev/null | wc -l)"
  [[ "$frames" -gt 0 ]] || { echo "No se pudo extraer ningún frame" >&2; rm -rf "$tmp"; return 1; }
  ok "$frames frames"

  # Throbber: se reusan los del tema base (el spinner de progreso).
  base="$THEMES_DIR/cachyos-bootanimation"
  [[ -d "$base" ]] && cp "$base"/throbber-*.png "$tmp"/ 2>/dev/null || true

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

# $1 = video (o "auto"), $2 = 1 si además se aplica
cmd_steam_convert() {
  local video="${1:-auto}" apply="${2:-0}" name fps res created dir
  [[ "$video" == "auto" ]] && video="$(steam_video)"

  # Aceptar un nombre relativo dentro de movies/
  if [[ ! -f "$video" ]]; then
    dir="$(steam_dirs | head -1)"
    [[ -f "$dir/$video" ]] && video="$dir/$video"
  fi
  [[ -f "$video" ]] || {
    echo "No existe el video: $video" >&2
    echo "Ver los disponibles: plymouth-preview steam-source" >&2
    exit 1
  }

  fps="${STEAM_FPS:-30}"
  res="${STEAM_RES:-1920x1080}"
  # Nombre del tema.
  # Las compradas se llaman <item_id>_<hash>.webm: se usa el nombre real
  # ("Elden Ring Ranni" -> steam-elden-ring-ranni).
  local base_name item_id legible
  base_name="$(basename "$video" .webm)"
  if [[ "$base_name" =~ ^([0-9]+)_[0-9a-f]+$ ]]; then
    item_id="${BASH_REMATCH[1]}"
    legible="$(steam_item_name "$item_id")"
    if [[ -n "$legible" ]]; then
      base_name="$legible"
    else
      base_name="item-$item_id"
    fi
  fi
  # A slug: minúsculas, espacios y símbolos a guiones.
  base_name="$(printf '%s' "$base_name" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]\+/-/g; s/^-\+//; s/-\+$//')"
  case "$base_name" in
    steam*) name="$base_name" ;;
    *)      name="steam-$base_name" ;;
  esac

  created="$(convert_video_to_theme "$video" "$name" "$fps" "$res" | tail -1)"

  if [[ "$apply" == "1" ]]; then
    cmd_use "$created"
  else
    echo
    say "Para verlo:     plymouth-preview preview $created"
    say "Para aplicarlo: plymouth-preview use $created"
  fi
}

# ── Dispatch ──────────────────────────────────────────────────────────────
case "${1:-view}" in
  view|shot|build|use|x11|shell|steam-convert|steam-apply|"") need_root "$@" ;;
esac
use_real_home

cmd="${1:-view}"
shift || true

case "$cmd" in
  view|"")       cmd_view ;;
  x11)           cmd_x11 "${1:-8}" ;;
  shell)         cmd_shell ;;
  shot)          cmd_shot "${1:-}" ;;
  sources)       cmd_sources ;;
  preview)       cmd_preview "${1:-}" ;;
  use)           cmd_use "${1:-}" ;;
  steam-source)  cmd_steam_source ;;
  steam-convert) cmd_steam_convert "${1:-auto}" 0 ;;
  steam-apply)   cmd_steam_convert "${1:-auto}" 1 ;;
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
