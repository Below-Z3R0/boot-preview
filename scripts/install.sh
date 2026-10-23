#!/usr/bin/env bash
# install.sh — deja limine-preview funcionando.
#
#   ./scripts/install.sh
#
# Que hace, en orden:
#   1. Verifica/instala las dependencias (qemu, xorriso, OVMF, imagemagick)
#   2. Instala los scripts en ~/.local/bin/
#   3. Crea ~/.config/limine-preview/ con la config de prueba y el wallpaper
#   4. Guarda la ruta del repo (para que el CLI sepa dónde está)
#   5. Crea <repo>/previews/ para las capturas
#
# NO toca nada del sistema: ni /boot/limine.conf, ni /etc/default/limine,
# ni el initramfs. Para aplicar el tema de verdad: limine-preview apply
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${XDG_CONFIG_HOME:-$HOME/.config}"
BIN_DIR="$HOME/.local/bin"
PREVIEW_DIR="$PREFIX/limine-preview"

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

# KVM: sin el, la VM va por emulación (lenta pero funciona).
if [[ -e /dev/kvm ]]; then
  ok "/dev/kvm presente — la VM arranca en segundos"
else
  warn "sin /dev/kvm: la VM va a ir por emulación (lenta)"
fi

# ── 2. Los scripts ─────────────────────────────────────────────────────────
step "2/5  Los scripts"

mkdir -p "$BIN_DIR"
install -m 755 "$REPO_DIR/examples/limine-preview.sh" "$BIN_DIR/limine-preview"
ok "$BIN_DIR/limine-preview"

install -m 755 "$REPO_DIR/scripts/make-wallpaper.sh" "$BIN_DIR/make-wallpaper"
ok "$BIN_DIR/make-wallpaper"

install -m 755 "$REPO_DIR/scripts/prepare-wallpapers.sh" "$BIN_DIR/prepare-wallpapers"
ok "$BIN_DIR/prepare-wallpapers"

# Un shim en /usr/local/bin para que `sudo limine-preview` funcione:
# ~/.local/bin no esta en el secure_path de sudo.
if [[ -w /usr/local/bin ]] || sudo -n true 2>/dev/null; then
  cat > /tmp/limine-preview-shim.$$ <<'SHIM'
#!/usr/bin/env bash
# Wrapper de limine-preview para poder correrlo con `sudo`.
# `sudo limine-preview` falla con "command not found" porque ~/.local/bin
# no esta en el secure_path de sudo. Si lo invoca root, baja al usuario que
# lo lanzo: QEMU no debe correr como root.
set -euo pipefail
script="${HOME}/.local/bin/limine-preview"
if [[ ! -x "$script" ]]; then
  script="/home/b3low-z3r0/.local/bin/limine-preview"
fi
if [[ "$(id -u)" == "0" && -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
  exec sudo -u "$SUDO_USER" "$script" "$@"
fi
exec "$script" "$@"
SHIM
  if sudo install -m 755 /tmp/limine-preview-shim.$$ /usr/local/bin/limine-preview 2>/dev/null; then
    ok "/usr/local/bin/limine-preview (shim para sudo)"
  else
    warn "no se pudo instalar el shim en /usr/local/bin (sudo?)"
  fi
  rm -f /tmp/limine-preview-shim.$$
else
  warn "sin shim: `sudo limine-preview` no va a funcionar"
fi

# ── 3. La config de prueba y el wallpaper ───────────────────────────────────
step "3/5  La config de prueba"

mkdir -p "$PREVIEW_DIR/wallpaper"
if [[ -f "$PREVIEW_DIR/limine.conf" ]]; then
  cp "$PREVIEW_DIR/limine.conf" "$PREVIEW_DIR/limine.conf.bak"
  say "hay una config previa: guardada como limine.conf.bak"
fi
install -m 644 "$REPO_DIR/examples/limine-preview.conf" "$PREVIEW_DIR/limine.conf"
ok "$PREVIEW_DIR/limine.conf"

# ── 4. El wallpaper ────────────────────────────────────────────────────────
step "4/5  El wallpaper"

if [[ -f "$PREVIEW_DIR/wallpaper/rei.png" ]]; then
  ok "$PREVIEW_DIR/wallpaper/rei.png (ya estaba)"
else
  install -m 644 "$REPO_DIR/examples/wallpaper/rei.png" "$PREVIEW_DIR/wallpaper/rei.png"
  ok "$PREVIEW_DIR/wallpaper/rei.png"
fi

# ── 5. La ruta del repo y la carpeta de previews ───────────────────────────
step "5/5  El repo y la carpeta de previews"

# El CLI lo lee para saber dónde guardar las capturas (previews/) y dónde
# vive el repo.
printf '%s\n' "$REPO_DIR" > "$PREVIEW_DIR/repo"
ok "$PREVIEW_DIR/repo -> $REPO_DIR"

mkdir -p "$REPO_DIR/previews"
ok "$REPO_DIR/previews (capturas)"

cat > "$PREVIEW_DIR/.gitignore" <<'EOF'
# El wallpaper por defecto y sus variantes no van al repo.
wallpaper/
EOF
ok "$PREVIEW_DIR/.gitignore"

# ── Listo ──────────────────────────────────────────────────────────────────
printf '\n\033[1;38;2;125;207;216m✓ Listo.\033[0m\n\n'
cat <<EOF
  Ver el menú (ventana de QEMU):
      limine-preview

  Verlo con tu config real de /boot/limine.conf:
      limine-preview real

  Capturar a PNG (van a $REPO_DIR/previews/):
      limine-preview shot

  Poner una imagen nueva como wallpaper del preview:
      limine-preview add ~/Pictures/foto.png

  Editar la config del preview:
      limine-preview edit

  Aplicar al Limine real (wallpaper + config visual):
      limine-preview apply

  Generar las dos versiones del wallpaper (real y difuminada):
      limine-preview wallpapers

  En la ventana de QEMU: flechas, ENTER para bootear, E para editar la
  config, F1 para el firmware, ESC para salir. Ctrl+Q cierra QEMU.
EOF
