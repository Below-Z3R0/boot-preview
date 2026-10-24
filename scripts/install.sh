#!/usr/bin/env bash
# install.sh — deja boot-preview funcionando.
#
#   ./scripts/install.sh
#
# Que hace, en orden:
#   1. Verifica/instala las dependencias de Limine (qemu, xorriso, OVMF,
#      imagemagick) y de Plymouth (ffmpeg, xdotool, xhost)
#   2. Instala los scripts en ~/.local/bin/ y ~/.local/lib/boot-preview/
#   3. Crea ~/.config/boot-preview/ con la config de prueba y el wallpaper
#   4. Guarda la ruta del repo (para que el CLI sepa dónde está)
#   5. Crea <repo>/previews/ para las capturas
#
# NO toca nada del sistema: ni /boot/limine.conf, ni /etc/default/limine,
# ni el initramfs. Para aplicar el tema de verdad: boot-preview loader apply
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${XDG_CONFIG_HOME:-$HOME/.config}"
BIN_DIR="$HOME/.local/bin"
LIB_DIR="$HOME/.local/lib/boot-preview"
CONFIG_DIR="$PREFIX/boot-preview"
LOADER_DIR="$CONFIG_DIR/loader"

say() { printf '  %s\n' "$*"; }
step() { printf '\n\033[1;38;2;235;194;186m▸\033[0m \033[1m%s\033[0m\n' "$*"; }
warn() { printf '  \033[38;2;246;153;150m! %s\033[0m\n' "$*"; }
ok()   { printf '  \033[38;2;125;207;216m✓\033[0m %s\n' "$*"; }

# ── 1. Dependencias ────────────────────────────────────────────────────────
step "1/5  Dependencias"

install_pkgs=()

command -v qemu-system-x86_64 >/dev/null 2>&1 || install_pkgs+=(qemu-desktop)
command -v xorriso            >/dev/null 2>&1 || install_pkgs+=(libisoburn)
command -v magick             >/dev/null 2>&1 || install_pkgs+=(imagemagick)
command -v socat              >/dev/null 2>&1 || install_pkgs+=(socat)

if [[ ${#install_pkgs[@]} -gt 0 ]]; then
  say "Faltan: ${install_pkgs[*]}"
  say "Instalando con sudo (puede pedir contraseña)..."
  sudo pacman -S --needed --noconfirm "${install_pkgs[@]}" >/dev/null
  ok "instalados: ${install_pkgs[*]}"
else
  ok "qemu-system-x86_64, xorriso, magick y socat ya estaban"
fi

# OVMF: sin el, el menu arranca en modo texto y no se ve el wallpaper.
if [[ ! -f /usr/share/edk2/x64/OVMF_CODE.4m.fd ]]; then
  say "Falta el firmware UEFI (edk2-ovmf)"
  sudo pacman -S --needed --noconfirm edk2-ovmf >/dev/null
  ok "edk2-ovmf instalado"
else
  ok "OVMF (UEFI) presente"
fi

# vips (libvips): opcional pero recomendado. Es ~6x mas rapido que ImageMagick
# para el resize+blur de los wallpapers y para leer el framebuffer del preview.
if command -v vips >/dev/null 2>&1; then
  ok "vips (libvips) presente — wallpapers ~6x mas rapidos"
else
  warn "sin vips: se usa ImageMagick (mas lento, pero funciona)"
fi

# KVM: sin el, la VM va por emulación (lenta pero funciona).
if [[ -e /dev/kvm ]]; then
  ok "/dev/kvm presente — la VM arranca en segundos"
else
  warn "sin /dev/kvm: la VM va a ir por emulación (lenta)"
fi

# ── Dependencias de Plymouth ───────────────────────────────────────────────
# ffmpeg: convierte los videos de Steam en temas (extrae los frames).
# xdotool: encuentra la ventana de Plymouth para capturarla.
# xhost:   da acceso de root al servidor X (sin eso el splash no aparece).
ply_pkgs=()
command -v ffmpeg  >/dev/null 2>&1 || ply_pkgs+=(ffmpeg)
command -v xdotool >/dev/null 2>&1 || ply_pkgs+=(xdotool)
command -v xhost   >/dev/null 2>&1 || ply_pkgs+=(xorg-xhost)

if [[ ${#ply_pkgs[@]} -gt 0 ]]; then
  say "Faltan (Plymouth): ${ply_pkgs[*]}"
  sudo pacman -S --needed --noconfirm "${ply_pkgs[@]}" >/dev/null
  ok "instalados: ${ply_pkgs[*]}"
else
  ok "ffmpeg, xdotool y xhost ya estaban"
fi

# El preview de Plymouth necesita el renderer X11: Plymouth ya lo trae
# compilado (paquete plymouth), no hay nada que instalar.
if [[ -f /usr/lib/plymouth/renderers/x11.so ]]; then
  ok "renderer X11 de Plymouth presente (sin VM)"
else
  warn "sin renderers/x11.so: el preview de Plymouth no va a funcionar"
fi

# ── 2. Los scripts ─────────────────────────────────────────────────────────
step "2/5  Los scripts"

mkdir -p "$BIN_DIR" "$LIB_DIR"

# El dispatcher: un solo comando, `boot-preview`.
install -m 755 "$REPO_DIR/scripts/boot-preview.sh" "$BIN_DIR/boot-preview"
ok "$BIN_DIR/boot-preview"

# Los dos modulos van a ~/.local/lib/boot-preview/, fuera del PATH: el
# usuario solo deberia ver `boot-preview`.
install -m 755 "$REPO_DIR/scripts/loader.sh"    "$LIB_DIR/loader.sh"
install -m 755 "$REPO_DIR/scripts/animation.sh" "$LIB_DIR/animation.sh"
ok "$LIB_DIR/{loader,animation}.sh"

# Los auxiliares (tambien fuera del PATH: se invocan desde el modulo).
install -m 755 "$REPO_DIR/scripts/make-wallpaper.sh"     "$LIB_DIR/make-wallpaper.sh"
install -m 755 "$REPO_DIR/scripts/prepare-wallpapers.sh" "$LIB_DIR/prepare-wallpapers.sh"
ok "$LIB_DIR/{make-wallpaper,prepare-wallpapers}.sh"

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

# ── 3. La config de prueba y el wallpaper ───────────────────────────────────
step "3/5  La config de prueba"

mkdir -p "$LOADER_DIR/wallpaper"
if [[ -f "$LOADER_DIR/limine.conf" ]]; then
  cp "$LOADER_DIR/limine.conf" "$LOADER_DIR/limine.conf.bak"
  say "hay una config previa: guardada como limine.conf.bak"
fi
install -m 644 "$REPO_DIR/examples/loader.conf" "$LOADER_DIR/limine.conf"
ok "$LOADER_DIR/limine.conf"

# ── 4. El wallpaper ────────────────────────────────────────────────────────
step "4/5  El wallpaper"

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

# ── 5. La ruta del repo y la carpeta de previews ───────────────────────────
step "5/5  El repo y la carpeta de previews"

# El CLI lo lee para saber dónde guardar las capturas (previews/) y dónde
# vive el repo.
printf '%s\n' "$REPO_DIR" > "$CONFIG_DIR/repo"
ok "$CONFIG_DIR/repo -> $REPO_DIR"

mkdir -p "$REPO_DIR/previews"
ok "$REPO_DIR/previews (capturas)"

cat > "$CONFIG_DIR/.gitignore" <<'EOF'
# El wallpaper por defecto y sus variantes no van al repo.
loader/wallpaper/
EOF
ok "$CONFIG_DIR/.gitignore"

# ── Listo ──────────────────────────────────────────────────────────────────
printf '\n\033[1;38;2;125;207;216m✓ Listo.\033[0m\n\n'
cat <<EOF
  LIMINE (el menú del bootloader):
      boot-preview loader              # preview del menú
      boot-preview loader real         # con tu config real de /boot/limine.conf
      boot-preview loader shot         # captura a PNG
      boot-preview loader add ~/Pictures/foto.png
      boot-preview loader edit         # abrir la config en tu editor
      boot-preview loader apply        # aplicar al Limine real
      boot-preview loader wallpapers   # las dos versiones del wallpaper

  PLYMOUTH (el splash de la animación):
      boot-preview animation access    # una vez por sesión: acceso de root al X
      boot-preview animation           # ventana con el splash
      boot-preview animation shot      # captura a PNG
      boot-preview animation sources   # los temas instalados
      boot-preview animation use <tema>   # aplicarlo al sistema (-R)

  En la ventana de QEMU: flechas, ENTER para bootear, E para editar la
  config, F1 para el firmware, ESC para salir. Ctrl+Q cierra QEMU.
EOF
