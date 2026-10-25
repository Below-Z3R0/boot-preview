#!/usr/bin/env bash
# boot-preview animation — ver y convertir splashes de Plymouth sin reiniciar.
#
# Por debajo usa el Plymouth OFICIAL del sistema (plymouthd + sus plugins)
# con el renderer X11: dibuja en una ventana GTK sobre tu escritorio. No es
# un reproductor de imágenes: es el mismo binario, los mismos plugins y el
# mismo tema que usa la máquina al arrancar.
#
#   boot-preview animation                     # ventana con el splash
#   boot-preview animation x11 [segundos]      # idem, con límite de tiempo
#   boot-preview animation shot [archivo.png]  # captura el splash y sale
#   boot-preview animation access              # da acceso de root al servidor X
#   boot-preview animation sources             # temas de Plymouth instalados
#   boot-preview animation preview <tema>      # tema SOLO del preview
#   boot-preview animation use <tema>          # tema del SISTEMA (-R, regenera initramfs)
#   boot-preview animation steam-source        # videos de Steam (cuál está activo)
#   boot-preview animation steam-convert <vid> # convierte un video en tema
#   boot-preview animation steam-apply <vid>   # convierte y lo aplica al sistema
#   boot-preview animation help
#
# REQUISITO
# ---------
# plymouthd exige root, y root solo puede abrir ventanas si el servidor X le
# da acceso. Sin esto el splash no aparece (ni error). Se resuelve una vez
# por sesión con:
#
#   boot-preview animation access
set -euo pipefail

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/boot-preview/animation"

# La ruta del repo (la guarda install.sh). Se usa para las capturas, que van
# a <repo>/previews/animation/. Vive un nivel arriba de la config.
REPO_DIR=""
repo_file="$(dirname "$CONFIG_DIR")/repo"
[[ -f "$repo_file" ]] && REPO_DIR="$(cat "$repo_file")"
[[ -d "$REPO_DIR" ]] || REPO_DIR=""
SYS_CONF="/etc/plymouth/plymouthd.conf"
THEMES_DIR="/usr/share/plymouth/themes"
X11_LOG="/tmp/plymouth-x11.log"

say()  { printf '  %s\n' "$*"; }
step() { printf '\n\033[1;38;2;235;194;186m▸\033[0m \033[1m%s\033[0m\n' "$*"; }
ok()   { printf '  \033[38;2;125;207;216m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[38;2;246;153;150m! %s\033[0m\n' "$*"; }

usage() {
  cat <<'EOF'
boot-preview animation — ver y convertir splashes de Plymouth sin reiniciar

Usa el Plymouth OFICIAL del sistema (plymouthd + sus plugins) con el
renderer X11: dibuja en una ventana GTK sobre tu escritorio. No es un
reproductor de imágenes: es el mismo daemon, los mismos plugins y el mismo
tema que usa la máquina al arrancar. Sin VM, sin initramfs.

USO
  boot-preview animation [comando] [opciones]

VER
  (ninguno)              Ventana con el splash
  x11 [segundos]         Idem, con límite de tiempo (0 = hasta Ctrl+C)

CAPTURAR
  shot [archivo]         Captura el splash a un PNG + contact sheet.
                         Sin nombre va a <repo>/previews/animation/

ACCESO
  access                 Da acceso de root al servidor X (una vez por
                         sesión). Sin esto el splash no aparece y no hay
                         error: plymouthd exige root y root necesita
                         permiso de X.

TEMAS
  sources                Los temas de Plymouth instalados
  preview <tema>         Cambia el tema SOLO del preview
  use <tema>             Aplica el tema al SISTEMA (-R, regenera el
                         initramfs)

STEAM
  steam-source           Los videos de startup (cuál está activo)
  steam-convert <vid>   Convierte un video en tema de Plymouth
  steam-apply <vid>     Convierte y lo aplica al sistema

OTROS
  help                   Esta ayuda

EJEMPLOS
  boot-preview animation
  boot-preview animation access
  boot-preview animation shot
  boot-preview animation sources
  boot-preview animation steam-convert

AJUSTES DE LA CONVERSIÓN
  STEAM_FPS=15 STEAM_RES=1280x720 boot-preview animation steam-convert
                         15 fps a 720p: 105 frames, 45 MB. A 30 fps y
                         1080p son 210 frames y 133 MB.
EOF
}

# ── HOME real cuando se corre con sudo ────────────────────────────────────
use_real_home() {
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    local uh
    uh="$(getent passwd "$SUDO_USER" | cut -d: -f6)"
    export HOME="$uh"
    CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/boot-preview/animation"

# La ruta del repo (la guarda install.sh). Se usa para las capturas, que van
# a <repo>/previews/animation/. Vive un nivel arriba de la config.
REPO_DIR=""
repo_file="$(dirname "$CONFIG_DIR")/repo"
[[ -f "$repo_file" ]] && REPO_DIR="$(cat "$repo_file")"
[[ -d "$REPO_DIR" ]] || REPO_DIR=""
  fi
}

# ── Root ──────────────────────────────────────────────────────────────────
# plymouthd exige root. Se preservan las variables del entorno gráfico: sin
# DISPLAY (o WAYLAND_DISPLAY) el renderer x11.so no puede abrir la ventana.
need_root() {
  if [[ "$(id -u)" != "0" ]]; then
    exec sudo --preserve-env=HOME,XDG_CACHE_HOME,XDG_CONFIG_HOME,DISPLAY,WAYLAND_DISPLAY,XDG_RUNTIME_DIR,XAUTHORITY,TERM,STEAM_FPS,STEAM_RES,PLYMOUTH_SCREEN_W,PLYMOUTH_SCREEN_H \
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

# ── Acceso de root al servidor X ──────────────────────────────────────────
# Sin esto, plymouthd cae al renderer DRM (que el compositor ya tiene tomado)
# y la ventana no aparece: pantalla en negro, sin error.
x11_root_access() {
  sudo --preserve-env=DISPLAY,XAUTHORITY xdotool getdisplaygeometry >/dev/null 2>&1
}

cmd_access() {
  step "Dando acceso de root al servidor X"
  if [[ -z "${DISPLAY:-}" ]]; then
    warn "No hay DISPLAY. X11 necesita un servidor gráfico."
    say "En Wayland Usually usás el Xwayland: export DISPLAY=:1"
    exit 1
  fi

  if x11_root_access; then
    ok "root ya puede abrir ventanas en $DISPLAY"
    return 0
  fi

  if ! command -v xhost >/dev/null 2>&1; then
    warn "Falta xhost (paquete xorg-xhost)."
    say "Instalalo con: sudo pacman -S xorg-xhost"
    exit 1
  fi

  say "Ejecutando: xhost +SI:localuser:root"
  if DISPLAY="$DISPLAY" xhost +SI:localuser:root 2>&1 | sed 's/^/    /'; then
    ok "root puede abrir ventanas en $DISPLAY"
    say "El permiso dura hasta que cierres la sesión gráfica."
  else
    warn "No se pudo dar el acceso."
    say "Probá manualmente:  DISPLAY=$DISPLAY xhost +SI:localuser:root"
    exit 1
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
  say "Para verlo: boot-preview animation"
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
  say "Para volver al de CachyOS: boot-preview animation use cachyos-bootanimation"
}

# ── Arrancar Plymouth en X11 ───────────────────────────────────────────────
# Monta el tema y levanta el daemon. Imprime el PID en stdout.
#
# Variables de salida (globales):
#   PP_ETC_DIR    config propia montada sobre /etc/plymouth
#   PP_RUN_THEME  copia del tema en /run/plymouth/themes
#   PP_PID        PID del plymouthd
start_plymouth_x11() {
  local theme="$1"

  PP_ETC_DIR="$(mktemp -d /tmp/boot-preview-animation-etc.XXXXXX)"
  PP_RUN_THEME="/run/plymouth/themes/$theme"

  # El tema va a /run/plymouth/themes, que Plymouth mira ANTES que /usr/share.
  sudo mkdir -p /run/plymouth/themes
  sudo rm -rf "$PP_RUN_THEME"
  sudo cp -r "$THEMES_DIR/$theme" /run/plymouth/themes/

  # El .plymouth del tema en /run debe apuntar a SI MISMO (ImageDir y
  # ScriptFile), si no Plymouth lee los frames del directorio equivocado.
  local ply_file="$PP_RUN_THEME/$theme.plymouth"
  if [[ -f "$ply_file" ]]; then
    sudo sed -i "s#^ImageDir=.*#ImageDir=$PP_RUN_THEME#" "$ply_file"
    sudo sed -i "s#^ScriptFile=.*#ScriptFile=$PP_RUN_THEME/script.script#" "$ply_file"
  fi

  # /etc/plymouth/plymouthd.conf del sistema dice Theme=cachyos-bootanimation
  # y gana sobre /run. Se monta una config propia sobre /etc/plymouth en un
  # namespace de mounts privado: el sistema NO se toca.
  cat > "$PP_ETC_DIR/plymouthd.conf" << EOF
[Daemon]
Theme=$theme
ShowDelay=0.5
DeviceTimeout=10
EOF
  chmod 644 "$PP_ETC_DIR/plymouthd.conf"

  sudo plymouth --quit 2>/dev/null || true
  sleep 1

  # CLAVE: sudo NO preserva DISPLAY, y sin él el renderer x11.so no puede
  # abrir la ventana GTK. env NO acepta "A,B,C" como lista: hay que pasar
  # cada variable por separado, con el valor del entorno del usuario.
  local env_args=()
  local v
  for v in DISPLAY XAUTHORITY WAYLAND_DISPLAY XDG_RUNTIME_DIR TERM; do
    [[ -n "${!v:-}" ]] && env_args+=("$v=${!v}")
  done

  # plymouthd con --debug vuelca todo a stdout: se redirige a /dev/null
  # porque el log verbose ya va al archivo.
  sudo unshare -m --propagation private \
    bash -c "mount --bind '$PP_ETC_DIR' /etc/plymouth && exec env ${env_args[*]} plymouthd --no-daemon --debug --debug-file='$X11_LOG' --mode=boot" >/dev/null 2>&1 &
  PP_PID=$!
  sleep 3

  sudo --preserve-env=DISPLAY,XAUTHORITY plymouth --show-splash 2>/dev/null
}

# ── Preview en X11 ────────────────────────────────────────────────────────
# Plymouth trae el renderer renderers/x11.so compilado con GTK3 y lo prueba
# PRIMERO (PLY_RENDERER_TYPE_AUTO). Es la misma técnica que el paquete
# plymouth-x11 de Debian/Fedora, que en Arch no está empaquetado pero cuyo
# renderer sí viene con Plymouth.
#
# El sistema NO se toca: el tema va a /run/plymouth/themes (que Plymouth mira
# antes que /usr/share) y la config se monta sobre /etc/plymouth en un
# namespace de mounts privado.
cmd_x11() {
  local segundos="${1:-0}"
  local theme
  theme="$(preview_theme)"
  [[ -d "$THEMES_DIR/$theme" ]] || { echo "No existe el tema: $theme" >&2; exit 1; }

  if [[ -z "${DISPLAY:-}" ]]; then
    warn "No hay DISPLAY. X11 necesita un servidor gráfico."
    say "En Wayland: export DISPLAY=:1 (Xwayland)"
    exit 1
  fi

  step "Plymouth en X11 con el tema '$theme'"

  if ! x11_root_access; then
    warn "root no puede abrir una ventana en $DISPLAY (control de acceso de X)."
    echo
    say "Solución (una vez por sesión):"
    say "  boot-preview animation access"
    echo
    say "Eso le da a root acceso al servidor X. Plymouth dibuja en una"
    say "ventana GTK sobre tu escritorio, con el tema real."
    exit 1
  fi

  start_plymouth_x11 "$theme"
  local pid="$PP_PID"

  if ! kill -0 "$pid" 2>/dev/null; then
    warn "plymouthd terminó. Log:"
    tail -20 "$X11_LOG" 2>/dev/null | sed 's/^/    /'
    cleanup_x11
    exit 1
  fi

  say ""
  say "Ventana abierta con el tema '$theme'."
  say "Desde OTRA terminal:"
  say "  sudo plymouth --reload        # recargar tras editar un PNG"
  say "  sudo plymouth --show-splash   # relanzar la animación"
  say "  sudo plymouth --quit          # cerrar"
  say ""
  say "Ctrl+C para cerrar."

  if [[ "$segundos" == "0" ]]; then
    while kill -0 "$pid" 2>/dev/null; do sleep 1; done
  else
    local i=0
    while [[ $i -lt "$segundos" ]] && kill -0 "$pid" 2>/dev/null; do
      sleep 1
      i=$((i+1))
      sudo --preserve-env=DISPLAY,XAUTHORITY plymouth --update="Iniciando $i" 2>/dev/null || true
    done
  fi

  sudo plymouth --quit 2>/dev/null || true
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  cleanup_x11
  ok "Cerrado"
}

cleanup_x11() {
  sudo rm -rf "${PP_RUN_THEME:-}" 2>/dev/null || true
  rm -rf "${PP_ETC_DIR:-}" 2>/dev/null || true
}

# ── Capturar ──────────────────────────────────────────────────────────────
# Levanta el preview en X11 y captura la ventana de Plymouth con
# ImageMagick. Se toman varias muestras y se queda con la de más contenido:
# durante los primeros frames el splash puede estar en negro.
cmd_shot() {
  local out="${1:-}"
  local theme
  theme="$(preview_theme)"
  [[ -d "$THEMES_DIR/$theme" ]] || { echo "No existe el tema: $theme" >&2; exit 1; }

  if [[ -z "${DISPLAY:-}" ]]; then
    warn "No hay DISPLAY. X11 necesita un servidor gráfico."
    exit 1
  fi
  command -v import >/dev/null 2>&1 || {
    warn "Falta ImageMagick (import)."
    exit 1
  }
  if ! x11_root_access; then
    warn "root no puede abrir una ventana en $DISPLAY."
    say "Solución: boot-preview animation access"
    exit 1
  fi

  start_plymouth_x11 "$theme"
  local pid="$PP_PID"

  # La ventana de Plymouth se llama "plymouthd".
  local wid
  wid="$(DISPLAY="$DISPLAY" xdotool search --name '^plymouthd$' 2>/dev/null | head -1 || true)"
  if [[ -z "$wid" ]]; then
    warn "No encontré la ventana de Plymouth."
    tail -20 "$X11_LOG" 2>/dev/null | sed 's/^/    /'
    sudo plymouth --quit 2>/dev/null || true
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    cleanup_x11
    exit 1
  fi

  step "Capturando (ventana $wid)"
  local tmpdir best_size=0 best_frame="" i
  tmpdir="$(mktemp -d /tmp/plymouth-shot.XXXXXX)"
  local -a candidates=()

  # 8 muestras cada 500 ms: 4 s de animación.
  for i in $(seq 1 8); do
    sleep 0.5
    local png="$tmpdir/shot-$i.png"
    DISPLAY="$DISPLAY" import -window "$wid" "$png" 2>/dev/null || continue
    [[ -s "$png" ]] || continue
    local size
    size="$(stat -c %s "$png" 2>/dev/null || echo 0)"
    # Un frame negro pesa ~2 KB; uno con contenido, 200 KB o mas.
    if [[ "$size" -gt 20000 ]]; then
      candidates+=("$i:$png")
    fi
    if [[ "$size" -gt "$best_size" ]]; then
      best_size="$size"
      best_frame="$png"
    fi
  done

  sudo plymouth --quit 2>/dev/null || true
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  cleanup_x11

  if [[ -z "$best_frame" || "$best_size" -lt 20000 ]]; then
    echo "No se pudo capturar el splash (todas las frames salieron vacías)." >&2
    rm -rf "$tmpdir"
    exit 1
  fi

  # Sin nombre: va a <repo>/previews/animation/. Cada herramienta tiene su
  # propia carpeta. Si no hay repo (instalado desde el bundle), va al
  # directorio de config.
  if [[ -z "$out" ]]; then
    local shot_dir
    if [[ -n "$REPO_DIR" ]]; then
      shot_dir="$REPO_DIR/previews/animation"
    else
      shot_dir="$CONFIG_DIR"
    fi
    mkdir -p "$shot_dir"
    out="$shot_dir/animation-$(date +%Y%m%d-%H%M%S).png"
  elif [[ "$out" != /* && -n "$REPO_DIR" ]]; then
    # Un nombre relativo va al directorio de previews.
    mkdir -p "$REPO_DIR/previews/animation"
    out="$REPO_DIR/previews/animation/$(basename "$out")"
  fi
  mkdir -p "$(dirname "$out")"
  cp "$best_frame" "$out"
  echo "Captura: $out  ($(du -h "$out" | cut -f1))"

  # Contact sheet con 4 frames DISTRIBUIDOS en el tiempo: así se ve cómo
  # evoluciona la animación, no solo el frame más pesado.
  if [[ ${#candidates[@]} -ge 4 ]] && command -v magick >/dev/null 2>&1; then
    local sheet="${out%.png}-sheet.png"
    local total=${#candidates[@]}
    local -a pick=()
    local pos
    for pos in 0 $((total / 3)) $((2 * total / 3)) $((total - 1)); do
      pick+=("${candidates[$pos]#*:}")
    done
    magick "${pick[@]}" -resize 640x360 +append -append "$sheet" 2>/dev/null && \
      echo "Sheet:   $sheet  ($(du -h "$sheet" | cut -f1))"
  fi

  rm -rf "$tmpdir"
}

# ── Steam: qué videos hay ─────────────────────────────────────────────────
steam_dirs() {
  local d real vistos=""
  for d in \
    "$HOME/.local/share/Steam/steamui/movies" \
    "$HOME/.steam/steam/steamui/movies"; do
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

# Nombres de las animaciones de la tienda y cuáles están instaladas.
#
# Los archivos se llaman <item_id>_<hash>.webm; el nombre real ("Elden Ring
# Ranni") está en el JSON "GetStartupMovies" de localconfig.vdf.
#
# Salida: una línea por animación, con el nombre, el id y si es la activa.
steam_movies_table() {
  python3 - "$HOME" "$1" <<'PY'
import json, os, re, sys

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
        raw = m.group(1).replace('\\"', '"')
        try:
            for item in json.loads(raw):
                iid = item.get("communityitemid")
                if iid:
                    nombres[iid] = item.get("name") or item.get("item_title") or iid
        except Exception:
            pass

# 2) Los archivos
vistos = set()
for base in (f"{home}/.local/share/Steam", f"{home}/.steam/steam"):
    d = f"{base}/config/communityitemscache/startupmovies"
    if not os.path.isdir(d):
        continue
    for f in sorted(os.listdir(d)):
        if not f.endswith(".webm"):
            continue
        ruta = f"{d}/{f}"
        real = os.path.realpath(ruta)
        if real in vistos:
            continue
        vistos.add(real)
        iid = f.split("_")[0]
        nombre = nombres.get(iid, iid)
        activa = "1" if real == os.path.realpath(actual) else "0"
        print(f"tienda\t{nombre}\t{iid}\t{activa}\t{ruta}")

# 3) Las que trae el cliente
for base in (f"{home}/.local/share/Steam", f"{home}/.steam/steam"):
    d = f"{base}/steamui/movies"
    if not os.path.isdir(d):
        continue
    for f in sorted(os.listdir(d)):
        if not f.endswith(".webm") or "startup" not in f:
            continue
        ruta = f"{d}/{f}"
        real = os.path.realpath(ruta)
        if real in vistos:
            continue
        vistos.add(real)
        activa = "1" if real == os.path.realpath(actual) else "0"
        print(f"cliente\t{f}\t-\t{activa}\t{ruta}")
    break
PY
}

cmd_steam_source() {
  local actual
  actual="$(steam_video 2>/dev/null || true)"

  step "Animaciones de inicio de Steam"

  local line origen nombre id activa ruta
  local -a tienda=() cliente=()

  while IFS=$'\t' read -r origen nombre id activa ruta; do
    [[ -n "$origen" ]] || continue
    if [[ "$origen" == "tienda" ]]; then
      tienda+=("$nombre"$'\t'"$id"$'\t'"$activa"$'\t'"$ruta")
    else
      cliente+=("$nombre"$'\t'"$id"$'\t'"$activa"$'\t'"$ruta")
    fi
  done < <(steam_movies_table "$actual")

  if [[ ${#tienda[@]} -gt 0 ]]; then
    say "Compradas en la tienda:"
    for line in "${tienda[@]}"; do
      IFS=$'\t' read -r nombre id activa ruta <<< "$line"
      if [[ "$activa" == "1" ]]; then
        printf '    \033[38;2;125;207;216m*\033[0m %-32s (%s)  ← la activa\n' "$nombre" "$id"
      else
        printf '      %-32s (%s)\n' "$nombre" "$id"
      fi
    done
    echo
  fi

  if [[ ${#cliente[@]} -gt 0 ]]; then
    say "Del cliente (Steam Big Picture / Deck):"
    for line in "${cliente[@]}"; do
      IFS=$'\t' read -r nombre id activa ruta <<< "$line"
      if [[ "$activa" == "1" ]]; then
        printf '    \033[38;2;125;207;216m*\033[0m %-44s ← la activa\n' "$nombre"
      else
        printf '      %-44s\n' "$nombre"
      fi
    done
    echo
  fi

  say "Activa: ${actual:-ninguna}"
  say "Steam las guarda en config/communityitemscache/startupmovies/"
}

# Nombre legible de una animación de la tienda.
steam_item_name() {
  local iid="$1"
  steam_movies_table "" 2>/dev/null | awk -F'\t' -v id="$iid" \
    '$1 == "tienda" && $3 == id { print $2; exit }'
}

# ── Detectar las propiedades del video ──────────────────────────────────────
# Los videos de Steam vienen en varias resoluciones y fps (1920x1200 a 30 fps
# los de la tienda, 450x450 a 60 fps los del Deck). Hardcodear 30/1920x1080
# hacia que la animacion se reproduzca mas rapido y no llene la pantalla.
#
# Devuelve: "<fps> <width> <height>"
video_props() {
  local video="$1"

  # ffprobe devuelve los campos en orden alfabetico, asi que se pide cada
  # uno por separado: el orden no se asume.
  local fps width height
  fps="$(ffprobe -v error -select_streams v:0 \
            -show_entries stream=r_frame_rate -of csv=p=0 "$video" 2>/dev/null)"
  width="$(ffprobe -v error -select_streams v:0 \
             -show_entries stream=width -of csv=p=0 "$video" 2>/dev/null)"
  height="$(ffprobe -v error -select_streams v:0 \
              -show_entries stream=height -of csv=p=0 "$video" 2>/dev/null)"

  [[ -n "$width" && -n "$height" ]] || return 1

  # r_frame_rate viene como "30/1" (num/den).
  local num den
  if [[ "$fps" == */* ]]; then
    num="${fps%%/*}"
    den="${fps##*/}"
    fps="$(python3 -c "print(round(float('$num')/float('$den')) if float('$den') else 30)" 2>/dev/null || echo 30)"
  fi
  [[ "$fps" =~ ^[0-9]+$ ]] || fps=30

  printf '%s %s %s\n' "$fps" "$width" "$height"
}

# ── Steam: convertir ──────────────────────────────────────────────────────
convert_video_to_theme() {
  local video="$1" name="$2" fps="$3" res="$4" dest tmp w h base
  dest="$THEMES_DIR/$name"

  [[ -f "$video" ]] || { echo "No existe el video: $video" >&2; return 1; }
  command -v ffmpeg >/dev/null 2>&1 || { echo "FALTA: ffmpeg" >&2; return 1; }

  tmp="$(mktemp -d /tmp/plymouth-theme.XXXXXX)"

  # Lo que se pide se usa; si no, se detecta del video. Un video de la
  # tienda de Steam viene en 1920x1200 a 30 fps; uno del Deck, en 450x450 a
  # 60 fps. Hardcodear un valor hacia que la animacion se reproduzca
  # rapido y no llene la pantalla.
  local v_fps v_w v_h
  read -r v_fps v_w v_h < <(video_props "$video") || true
  [[ -n "$v_fps" ]] || v_fps=30
  [[ -n "$v_w" ]] || v_w=1920
  [[ -n "$v_h" ]] || v_h=1080

  # El aspect ratio se respeta: la resolucion por defecto es la del video,
  # no una fija. El alto se ajusta al ancho pedido para no deformar.
  if [[ -z "$res" || "$res" == "auto" ]]; then
    w="$v_w"
    h="$(python3 -c "print(round($v_h * $v_w / $v_w))")"
    res="${w}x${h}"
  else
    w="${res%x*}"; h="${res#*x}"
  fi
  fps="$v_fps"

  step "Extrayendo frames ($(basename "$video"), ${fps}fps, $res)"
  # Crop centrado al aspecto pedido, escalado Lanczos. Sin audio.
  #
  # El crop NO se pierde nada: los videos de Steam (1920x1200) traen la
  # imagen centrada con bordes negros, asi que recortar a 16:9 la conserva.
  ffmpeg -loglevel error -y -i "$video" \
    -vf "fps=$fps,crop=iw:min(ih\,iw*$h/$w):0:(ih-min(ih\,iw*$h/$w))/2,scale=$w:$h:flags=lanczos" \
    -start_number 0 "$tmp/animation-%03d.png" 2>/dev/null

  # OJO: `frames` es un array, no un contador. Con set -u, un array vacío
  # rompe `${#frames[@]}`.
  #
  # El glob de bash ordena LEXICALMENTE: con más de 999 frames,
  # animation-1000.png vendría antes que animation-999.png y la animación
  # se reproduciría desordenada. Se ordena por número con `sort -V`.
  local -a frames=()
  mapfile -t frames < <(printf '%s\n' "$tmp"/animation-*.png | sort -V)
  [[ ${#frames[@]} -gt 0 && -f "${frames[0]}" ]] || {
    echo "No se pudo extraer ningún frame" >&2; rm -rf "$tmp"; return 1; }
  local total=${#frames[@]}
  ok "$total frames"

  # Throbber: se reusan los del tema base (el spinner de progreso).
  base="$THEMES_DIR/cachyos-bootanimation"
  [[ -d "$base" ]] && cp "$base"/throbber-*.png "$tmp"/ 2>/dev/null || true

  # Plugin "script", no "two-step".
  #
  # two-step ata la animacion al final del boot (UseEndAnimation): sin un
  # boot real nunca arranca. "script" es un loop libre: Plymouth llama a la
  # funcion de refresh cada 1/refresh_rate y ahi se avanza un frame.
  #
  # Un solo sprite al que se le cambia la imagen: muchisima menos memoria
  # que un sprite por frame (105 frames a 1280x720).
  #
  # EL TIMING: Plymouth refresca a FRAMES_PER_SECOND (50 en el plugin
  # script). Si el script avanza 1 frame por refresh, una animacion de
  # 30 fps se reproduce 1.67x mas rapido y se repite a la mitad (que era
  # el bug).
  #
  # Para que la duracion sea la del video, hay que avanzar fps/50 frames
  # por refresh. Como eso no siempre es un entero, se usan dos
  # contadores: uno lleva la parte fraccionaria y el otro avanza los
  # frames completos.
  #
  #   video 30 fps -> 3 frames cada 5 refreshes  (3/5 = 0.6 = 30/50)
  #   video 60 fps -> 6 frames cada 5 refreshes  (6/5 = 1.2 = 60/50)
  #   video 50 fps -> 1 frame por refresh        (exacto)
  # El playback real = refresh_rate * frames_por_refresh. Para que la
  # animacion dure lo mismo que el video (fps del video), los frames por
  # refresh tienen que ser fps/50.
  #
  #   video 30 fps, refresh 50 Hz -> 0.6 frames por refresh
  #   video 60 fps, refresh 50 Hz -> 1.2 frames por refresh
  #
  # Como los frames son enteros, se usan dos contadores: uno avanza cada
  # refresh y otro lleva la parte fraccionaria. El script de Plymouth tiene
  # IF y % (verificado), asi que alcanza.
  #
  #   refreshes_por_frame = 50 / fps_video   (p.ej. 5 refreshes por frame
  #                                            a 10 fps; 1 a 50 fps)
  #   frames_por_refresh = 1 / refreshes_por_frame
  #
  # Se usa un contador que avanza de a "frames_por_refresh" cada vez.
  local step_num step_den
  read -r step_num step_den < <(python3 -c "
from fractions import Fraction
fps = $fps
# refresh de Plymouth = 50 Hz (FRAMES_PER_SECOND del plugin script).
rate = Fraction(50, 1)
avance = Fraction(fps, 1) / rate
print(avance.numerator, avance.denominator)
")
  # Con Fraction: advance = fps/50 = num/den.
  # Se avanza num frames cada den refreshes.
  step_num="$step_num"; step_den="$step_den"

  if [[ "$step_num" == "$step_den" ]]; then
    # 1 frame por refresh: el caso simple.
    step_num=1; step_den=1
  elif [[ "$step_num" -gt "$step_den" ]]; then
    # Mas de un frame por refresh: no se puede fraccionar hacia adelante,
    # asi que se avanza de a num frames y se descarta el resto.
    step_num=$((step_num / step_den)); step_den=1
  fi

  # EL TAMAÑO: el plugin script NO tiene SetScale. El sprite se dibuja a
  # su tamaño nativo, asi que para que llene la pantalla los frames se
  # generan al tamaño de la pantalla (por defecto, la de Plymouth).
  local screen_w screen_h
  screen_w="${PLYMOUTH_SCREEN_W:-1920}"
  screen_h="${PLYMOUTH_SCREEN_H:-1080}"

  if [[ "$w" != "$screen_w" || "$h" != "$screen_h" ]]; then
    step "Escalando a ${screen_w}x${screen_h} (el plugin script no puede escalar el sprite)"
    local -a big=()
    local f
    for f in "${frames[@]}"; do
      big+=("$tmp/big-$(basename "$f")")
      # vips no tiene --height en la operacion resize: se usa thumbnail
      # con --size both (escala sin recortar). El ! de magick fuerza el
      # aspecto exacto.
      vips thumbnail "$f" "${big[-1]}" "$screen_w" --height "$screen_h" --size both >/dev/null 2>&1 \
        || magick "$f" -resize "${screen_w}x${screen_h}!" "${big[-1]}" 2>/dev/null \
        || { echo "No se pudo escalar un frame" >&2; rm -rf "$tmp"; return 1; }
    done
    frames=("${big[@]}")
  fi

  {
    echo "# Animacion generada desde $(basename "$video")."
    echo "# Plugin \"script\" de Plymouth: loop libre, no atado al boot real."
    echo "#"
    echo "# Timing: Plymouth refresca a 50 fps (FRAMES_PER_SECOND del plugin"
    echo "# script). El video va a $fps fps, asi que se avanzan $step_num frames"
    echo "# cada $step_den refreshes para que el timing sea el del video."
    echo ""
    echo "frames = ["
    local f
    for f in "${frames[@]}"; do
      echo "  \"$(basename "$f")\","
    done
    echo "];"
    echo ""
    echo "Window.SetBackgroundTopColor(0, 0, 0);"
    echo "Window.SetBackgroundBottomColor(0, 0, 0);"
    echo ""
    echo "sprite = Sprite(Image(\"$(basename "${frames[0]}")\"));"
    echo "sprite.SetZ(0);"
    echo "sprite.SetOpacity(1);"
    echo ""
    echo "frame = 0;"
    echo "tick = 0;"
    echo ""
    echo "fun Refresh ()"
    echo "{"
    if [[ "$step_num" == "1" && "$step_den" == "1" ]]; then
      echo "  frame = (frame + 1) % ${#frames[@]};"
    else
      # Timing fraccionario: se avanzan $step_num frames cada $step_den
      # refreshes, para que la duracion sea la del video original.
      echo "  tick = (tick + 1) % $step_den;"
      echo "  if (tick == 0) {"
      echo "    frame = (frame + $step_num) % ${#frames[@]};"
      echo "  }"
    fi
    echo "  sprite.SetImage(Image(frames[frame]));"
    echo "}"
    echo ""
    echo "Plymouth.SetRefreshFunction(Refresh);"
  } > "$tmp/script.script"

  cat > "$tmp/$name.plymouth" <<EOF
[Plymouth Theme]
Name=$name
Description=Generado desde $(basename "$video")
ModuleName=script

[script]
ImageDir=$dest
ScriptFile=$dest/script.script
EOF

  step "Instalando en $dest"
  sudo rm -rf "$dest"
  sudo mkdir -p "$dest"
  sudo cp "$tmp"/animation-*.png "$dest"/ 2>/dev/null || true
  sudo cp "$tmp"/big-*.png "$dest"/ 2>/dev/null || true
  sudo cp "$tmp"/throbber-*.png "$dest"/ 2>/dev/null || true
  sudo cp "$tmp/$name.plymouth" "$dest"/
  sudo cp "$tmp/script.script" "$dest"/
  rm -rf "$tmp"

  ok "Tema '$name' instalado ($(du -sh "$dest" 2>/dev/null | cut -f1))"
  printf '%s\n' "$name"
}

# $1 = video (o "auto"), $2 = 1 si además se aplica
cmd_steam_convert() {
  local video="${1:-auto}" apply="${2:-0}" name fps res dir
  [[ "$video" == "auto" ]] && video="$(steam_video)"

  # Aceptar un nombre relativo dentro de movies/
  if [[ ! -f "$video" ]]; then
    dir="$(steam_dirs | head -1)"
    [[ -f "$dir/$video" ]] && video="$dir/$video"
  fi
  [[ -f "$video" ]] || {
    echo "No existe el video: $video" >&2
    echo "Ver los disponibles: boot-preview animation steam-source" >&2
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

  # El nombre del tema va por stdout; los mensajes de progreso, a stderr.
  # Así el `tail -1` de abajo nunca agarra un mensaje por error.
  local created
  created="$(convert_video_to_theme "$video" "$name" "$fps" "$res" 2>/dev/null | tail -1)"
  [[ -n "$created" ]] || { echo "No se pudo convertir el video" >&2; exit 1; }

  if [[ "$apply" == "1" ]]; then
    cmd_use "$created"
  else
    echo
    say "Para verlo:     boot-preview animation preview $created"
    say "Para aplicarlo: boot-preview animation use $created"
  fi
}

# ── Dispatch ──────────────────────────────────────────────────────────────
case "${1:-view}" in
  view|shot|use|x11|steam-convert|steam-apply|"") need_root "$@" ;;
esac
use_real_home

cmd="${1:-view}"
shift || true

case "$cmd" in
  view|"")       cmd_x11 0 ;;
  x11)           cmd_x11 "${1:-0}" ;;
  shot)          cmd_shot "${1:-}" ;;
  access)        cmd_access ;;
  sources)       cmd_sources ;;
  preview)       cmd_preview "${1:-}" ;;
  use)           cmd_use "${1:-}" ;;
  steam-source)  cmd_steam_source ;;
  steam-convert) cmd_steam_convert "${1:-auto}" 0 ;;
  steam-apply)   cmd_steam_convert "${1:-auto}" 1 ;;
  help|-h|--help) usage ;;
  *)
     echo "Comando desconocido: $cmd" >&2
     echo >&2
     usage
     exit 1
     ;;
esac
