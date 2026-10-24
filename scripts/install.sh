#!/usr/bin/env bash
# install.sh — instala boot-preview (o solo una de sus partes).
#
#   ./scripts/install.sh                    # todo
#   ./scripts/install.sh loader             # solo el módulo de Limine
#   ./scripts/install.sh animation          # solo el módulo de Plymouth
#   ./scripts/install.sh --no-deps          # sin instalar paquetes del sistema
#
# Qué hace, en orden:
#   1. Instala las dependencias (salvo con --no-deps)
#   2. Copia el dispatcher a ~/.local/bin/ y los módulos a
#      ~/.local/lib/boot-preview/ (fuera del PATH: el usuario ve un comando)
#   3. Crea ~/.config/boot-preview/ con la config de la parte instalada
#   4. Guarda un estado (state.json) con las versiones de las dependencias
#
# NO toca nada del sistema: ni /boot/limine.conf, ni /etc/default/limine,
# ni el initramfs. Para aplicar el tema de verdad: boot-preview loader apply
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${XDG_CONFIG_HOME:-$HOME/.config}"
BIN_DIR="$HOME/.local/bin"
LIB_DIR="$HOME/.local/lib/boot-preview"
CONFIG_DIR="$PREFIX/boot-preview"

# ── Opciones ───────────────────────────────────────────────────────────────
WHAT="all"
WITH_DEPS=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    loader|animation|all) WHAT="$1"; shift ;;
    --no-deps)  WITH_DEPS=0; shift ;;
    --deps)     WITH_DEPS=1; shift ;;
    -h|--help)
      cat <<'EOF'
install.sh — instala boot-preview

  install.sh                  Instala todo (loader + animation)
  install.sh loader           Solo el módulo de Limine
  install.sh animation        Solo el módulo de Plymouth
  install.sh --no-deps        No instala paquetes del sistema
EOF
      exit 0 ;;
    *) echo "Opción desconocida: $1" >&2; exit 1 ;;
  esac
done

say() { printf '  %s\n' "$*"; }
step() { printf '\n\033[1;38;2;235;194;186m▸\033[0m \033[1m%s\033[0m\n' "$*"; }
warn() { printf '  \033[38;2;246;153;150m! %s\033[0m\n' "$*"; }
ok()   { printf '  \033[38;2;125;207;216m✓\033[0m %s\n' "$*"; }

want_loader()    { [[ "$WHAT" == "all" || "$WHAT" == "loader" ]]; }
want_animation() { [[ "$WHAT" == "all" || "$WHAT" == "animation" ]]; }

# ── 1. Dependencias ────────────────────────────────────────────────────────
if [[ "$WITH_DEPS" == "1" ]]; then
  step "1/4  Dependencias"
  pkgs=()

  if want_loader; then
    command -v qemu-system-x86_64 >/dev/null 2>&1 || pkgs+=(qemu-desktop)
    command -v xorriso            >/dev/null 2>&1 || pkgs+=(libisoburn)
    command -v magick             >/dev/null 2>&1 || pkgs+=(imagemagick)
    command -v socat              >/dev/null 2>&1 || pkgs+=(socat)
  fi

  if want_animation; then
    command -v ffmpeg  >/dev/null 2>&1 || pkgs+=(ffmpeg)
    command -v xdotool >/dev/null 2>&1 || pkgs+=(xdotool)
    command -v xhost   >/dev/null 2>&1 || pkgs+=(xorg-xhost)
  fi

  if [[ ${#pkgs[@]} -gt 0 ]]; then
    say "Faltan: ${pkgs[*]}"
    say "Instalando con sudo (puede pedir contraseña)..."
    sudo pacman -S --needed --noconfirm "${pkgs[@]}" >/dev/null
    ok "instalados: ${pkgs[*]}"
  else
    ok "todo instalado"
  fi

  if want_loader; then
    # OVMF: sin el, el menu arranca en modo texto y no se ve el wallpaper.
    if [[ ! -f /usr/share/edk2/x64/OVMF_CODE.4m.fd ]]; then
      say "Falta el firmware UEFI (edk2-ovmf)"
      sudo pacman -S --needed --noconfirm edk2-ovmf >/dev/null
      ok "edk2-ovmf instalado"
    else
      ok "OVMF (UEFI) presente"
    fi

    if [[ -e /dev/kvm ]]; then
      ok "/dev/kvm presente — la VM arranca en segundos"
    else
      warn "sin /dev/kvm: la VM va a ir por emulación (lenta)"
    fi

    # vips (libvips): opcional pero recomendado. Es ~6x mas rapido que
    # ImageMagick para el resize+blur de los wallpapers.
    if command -v vips >/dev/null 2>&1; then
      ok "vips (libvips) presente — wallpapers ~6x mas rapidos"
    fi
  fi

  if want_animation; then
    # El preview de Plymouth necesita el renderer X11: Plymouth ya lo trae
    # compilado (paquete plymouth), no hay nada que instalar.
    if [[ -f /usr/lib/plymouth/renderers/x11.so ]]; then
      ok "renderer X11 de Plymouth presente (sin VM)"
    else
      warn "sin renderers/x11.so: el preview de Plymouth no va a funcionar"
    fi
  fi
else
  step "1/4  Dependencias"
  ok "omitidas (--no-deps)"
fi

# ── 2. Los scripts ─────────────────────────────────────────────────────────
step "2/4  Los scripts"

mkdir -p "$BIN_DIR" "$LIB_DIR"

# El dispatcher: un solo comando, `boot-preview`.
install -m 755 "$REPO_DIR/scripts/boot-preview.sh" "$BIN_DIR/boot-preview"
ok "$BIN_DIR/boot-preview"

if want_loader; then
  install -m 755 "$REPO_DIR/scripts/loader.sh" "$LIB_DIR/loader.sh"
  install -m 755 "$REPO_DIR/scripts/make-wallpaper.sh"     "$LIB_DIR/make-wallpaper.sh"
  install -m 755 "$REPO_DIR/scripts/prepare-wallpapers.sh" "$LIB_DIR/prepare-wallpapers.sh"
  ok "$LIB_DIR/{loader,make-wallpaper,prepare-wallpapers}.sh"
fi

if want_animation; then
  install -m 755 "$REPO_DIR/scripts/animation.sh" "$LIB_DIR/animation.sh"
  ok "$LIB_DIR/animation.sh"
fi

# Un shim en /usr/local/bin para que `sudo boot-preview loader` funcione:
# ~/.local/bin no esta en el secure_path de sudo.
if [[ -w /usr/local/bin ]] || sudo -n true 2>/dev/null; then
  cat > /tmp/boot-preview-shim.$$ <<'SHIM'
#!/usr/bin/env bash
# Wrapper de boot-preview para poder correrlo con `sudo`.
# `sudo boot-preview` falla con "command not found" porque ~/.local/bin
# no esta en el secure_path de sudo. Si lo invoca root, baja al usuario que
# lo lanzo: QEMU no debe correr como root (no tiene acceso al socket grafico).
set -euo pipefail
script="${HOME}/.local/bin/boot-preview"
if [[ ! -x "$script" ]]; then
  script="/home/b3low-z3r0/.local/bin/boot-preview"
fi
if [[ "$(id -u)" == "0" && -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
  exec sudo -u "$SUDO_USER" "$script" "$@"
fi
exec "$script" "$@"
SHIM
  if sudo install -m 755 /tmp/boot-preview-shim.$$ /usr/local/bin/boot-preview 2>/dev/null; then
    ok "/usr/local/bin/boot-preview (shim para sudo)"
  else
    warn "no se pudo instalar el shim en /usr/local/bin (sudo?)"
  fi
  rm -f /tmp/boot-preview-shim.$$
else
  warn "sin shim: `sudo boot-preview` no va a funcionar"
fi

# ── 3. La config ───────────────────────────────────────────────────────────
step "3/4  La config"

if want_loader; then
  LOADER_DIR="$CONFIG_DIR/loader"
  mkdir -p "$LOADER_DIR/wallpaper"
  if [[ -f "$LOADER_DIR/limine.conf" ]]; then
    cp "$LOADER_DIR/limine.conf" "$LOADER_DIR/limine.conf.bak"
    say "hay una config previa: guardada como limine.conf.bak"
  fi
  install -m 644 "$REPO_DIR/examples/loader.conf" "$LOADER_DIR/limine.conf"
  ok "$LOADER_DIR/limine.conf"

  if [[ -f "$LOADER_DIR/wallpaper/rei.png" ]]; then
    ok "$LOADER_DIR/wallpaper/rei.png (ya estaba)"
  else
    install -m 644 "$REPO_DIR/examples/wallpaper/rei.png" "$LOADER_DIR/wallpaper/rei.png"
    ok "$LOADER_DIR/wallpaper/rei.png"
  fi

  # La ACTIVA: la que usan el preview y `apply`. Si no existe, se copia la
  # nitida para que todo funcione de entrada; la difuminada la genera
  # `boot-preview loader wallpapers`.
  if [[ -f "$LOADER_DIR/wallpaper/wallpaper.png" ]]; then
    ok "$LOADER_DIR/wallpaper/wallpaper.png (la activa, ya estaba)"
  else
    cp "$LOADER_DIR/wallpaper/rei.png" "$LOADER_DIR/wallpaper/wallpaper.png"
    ok "$LOADER_DIR/wallpaper/wallpaper.png (la activa, copiada de rei.png)"
  fi
fi

if want_animation; then
  mkdir -p "$CONFIG_DIR/animation"
  ok "$CONFIG_DIR/animation"
fi

# La ruta del repo: la usa el CLI para saber dónde van las capturas.
printf '%s\n' "$REPO_DIR" > "$CONFIG_DIR/repo"
ok "$CONFIG_DIR/repo -> $REPO_DIR"

mkdir -p "$REPO_DIR/previews/loader" "$REPO_DIR/previews/animation"
ok "$REPO_DIR/previews/{loader,animation}"

cat > "$CONFIG_DIR/.gitignore" <<'EOF'
# El wallpaper por defecto y sus variantes no van al repo.
loader/wallpaper/
EOF

# ── 4. El estado ───────────────────────────────────────────────────────────
# Snapshot de las versiones de las dependencias. `boot-preview verify` lo
# compara con el estado actual para avisar si algo cambio (por ejemplo tras
# una actualizacion del sistema).
step "4/4  Estado"

state_file="$CONFIG_DIR/state.json"
mkdir -p "$CONFIG_DIR"

python3 - "$state_file" "$WHAT" << 'PY'
import json, os, subprocess, sys, datetime

state_file, what = sys.argv[1], sys.argv[2]

def version(pkg):
    try:
        out = subprocess.run(["pacman", "-Q", pkg], capture_output=True, text=True)
        return out.stdout.strip().split()[-1] if out.returncode == 0 else None
    except Exception:
        return None

# Las dependencias de las que nos importa la version.
watch = ["qemu-desktop", "libisoburn", "edk2-ovmf", "imagemagick", "socat",
         "libvips", "ffmpeg", "xdotool", "xorg-xhost",
         "plymouth", "limine", "xorg-server-xvfb"]

deps = {}
for pkg in watch:
    v = version(pkg)
    if v:
        deps[pkg] = v

state = {}
if os.path.isfile(state_file):
    try:
        state = json.load(open(state_file))
    except Exception:
        state = {}

state["version"] = state.get("version", "0.1.0")
state["updated_at"] = datetime.date.today().isoformat()
state["components"] = sorted(set(state.get("components", []) + (
    ["loader", "animation"] if what == "all" else [what])))
state["deps"] = deps
state["repo"] = state.get("repo", "")

with open(state_file, "w") as f:
    json.dump(state, f, indent=2, sort_keys=True)

print(f"  {len(deps)} dependencias registradas")
PY

ok "$state_file"

# ── Listo ──────────────────────────────────────────────────────────────────
printf '\n\033[1;38;2;125;207;216m✓ Listo.\033[0m\n\n'

case "$WHAT" in
  loader)
    cat <<EOF
  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader real         # con tu config real de /boot/limine.conf
      boot-preview loader shot         # captura a PNG
      boot-preview loader add ~/Pictures/foto.png
      boot-preview loader edit         # abrir la config en tu editor
      boot-preview loader apply        # aplicar al Limine real
      boot-preview loader wallpapers   # las dos versiones del wallpaper
EOF
    ;;
  animation)
    cat <<EOF
  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation sources   # los temas instalados
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)
EOF
    ;;
  *)
    cat <<EOF
  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader shot         # captura a PNG
      boot-preview loader apply        # aplicar al Limine real

  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)
EOF
    ;;
esac

echo
say "Verificá la instalación con:  boot-preview verify"
