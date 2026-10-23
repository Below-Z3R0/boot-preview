#!/usr/bin/env bash
# install.sh — deja limine-preview funcionando.
#
#   ./scripts/install.sh              # instala lo basico
#   ./scripts/install.sh --with-theming   # ademas copia el wallpaper a /boot
#
# Que hace:
#   1. Verifica/instala las dependencias (qemu, xorriso, OVMF)
#   2. Compone el script a ~/.local/bin/limine-preview
#   3. Compone la config de prueba a ~/.config/limine-preview/limine.conf
#   4. Copia el wallpaper de ejemplo
#   5. (opcional) deja el tema listo para aplicar al /etc/default/limine real
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${XDG_CONFIG_HOME:-$HOME/.config}"
BIN_DIR="$HOME/.local/bin"
PREVIEW_DIR="$PREFIX/limine-preview"
with_theming=0

for arg in "$@"; do
  case "$arg" in
    --with-theming) with_theming=1 ;;
    -h|--help)
      sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "opción desconocida: $arg" >&2; exit 1 ;;
  esac
done

say() { printf '  %s\n' "$*"; }
step() { printf '\n\033[1;38;2;235;194;186m▸\033[0m \033[1m%s\033[0m\n' "$*"; }
warn() { printf '  \033[38;2;246;153;150m! %s\033[0m\n' "$*"; }
ok()   { printf '  \033[38;2;125;207;216m✓\033[0m %s\n' "$*"; }

# ── 1. Dependencias ────────────────────────────────────────────────────────
step "1/4  Dependencias"

need_sudo=0
install_pkgs=()

command -v qemu-system-x86_64 >/dev/null 2>&1 || { install_pkgs+=(qemu-desktop); need_sudo=1; }
command -v xorriso            >/dev/null 2>&1 || { install_pkgs+=(libisoburn); need_sudo=1; }
command -v magick             >/dev/null 2>&1 || { install_pkgs+=(imagemagick); need_sudo=1; }

if [[ ${#install_pkgs[@]} -gt 0 ]]; then
  say "Faltan: ${install_pkgs[*]}"
  if [[ $need_sudo == 1 ]]; then
    say "Instalando con sudo (puede pedir contraseña)..."
    sudo pacman -S --needed --noconfirm "${install_pkgs[@]}" >/dev/null
  else
    pacman -S --needed --noconfirm "${install_pkgs[@]}" >/dev/null
  fi
  ok "instalados: ${install_pkgs[*]}"
else
  ok "qemu-system-x86_64, xorriso y magick ya estaban"
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

# Los modelos de Real-ESRGAN (para el upscale con IA). El binario ya esta
# instalado pero busca los .param/.bin en ~/.local/bin/models/.
ESR_MODELS_DIR="$BIN_DIR/models"
if [[ ! -f "$ESR_MODELS_DIR/realesrgan-x4plus-anime.param" ]]; then
  say "Bajando los modelos de Real-ESRGAN (~47 MB)..."
  if curl -sSL -o /tmp/esr-models.zip \
      https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-ubuntu.zip \
      2>/dev/null && unzip -j -o /tmp/esr-models.zip 'models/*' -d "$ESR_MODELS_DIR" >/dev/null 2>&1; then
    ok "modelos de Real-ESRGAN en $ESR_MODELS_DIR"
  else
    warn "no se pudieron bajar los modelos de Real-ESRGAN (el upscale con IA queda sin usar)"
  fi
  rm -f /tmp/esr-models.zip
else
  ok "modelos de Real-ESRGAN ya estaban"
fi

# ── 2. El script ───────────────────────────────────────────────────────────
step "2/4  El script"

mkdir -p "$BIN_DIR"
install -m 755 "$REPO_DIR/examples/limine-preview.sh" "$BIN_DIR/limine-preview"
ok "$BIN_DIR/limine-preview"

# El generador de wallpapers (blur + velo + resolucion)
install -m 755 "$REPO_DIR/scripts/make-wallpaper.sh" "$BIN_DIR/make-wallpaper"
ok "$BIN_DIR/make-wallpaper"

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

# ── 3. La config de prueba ─────────────────────────────────────────────────
step "3/4  La config de prueba"

mkdir -p "$PREVIEW_DIR"
if [[ -f "$PREVIEW_DIR/limine.conf" ]]; then
  cp "$PREVIEW_DIR/limine.conf" "$PREVIEW_DIR/limine.conf.bak"
  say "hay una config previa: guardada como limine.conf.bak"
fi
install -m 644 "$REPO_DIR/examples/limine-preview.conf" "$PREVIEW_DIR/limine.conf"
ok "$PREVIEW_DIR/limine.conf"

# ── 4. El wallpaper ────────────────────────────────────────────────────────
step "4/4  El wallpaper"

mkdir -p "$PREVIEW_DIR/wallpaper"
install -m 644 "$REPO_DIR/examples/wallpaper/rei.png" "$PREVIEW_DIR/wallpaper/rei.png"
ok "$PREVIEW_DIR/wallpaper/rei.png"

if [[ $with_theming == 1 ]]; then
  step "extra  Copiar el wallpaper a /boot (para el Limine real)"
  sudo cp "$REPO_DIR/examples/wallpaper/rei.png" /boot/splash.png
  ok "/boot/splash.png"
  warn "esto NO aplica el tema: falta el bloque en /etc/default/limine"
  warn "mira docs/APLICAR.md para el paso a paso completo"
fi

# ── Listo ──────────────────────────────────────────────────────────────────
printf '\n\033[1;38;2;125;207;216m✓ Listo.\033[0m\n\n'
say "Ver el menu (ventana de QEMU):"
printf '    \033[1mlimine-preview\033[0m\n\n'
say "Verlo con tu config real de /boot/limine.conf:"
printf '    \033[1mlimine-preview --real\033[0m\n\n'
say "Capturar a PNG:"
printf '    \033[1mlimine-preview --shot ~/preview.png\033[0m\n\n'
say "Los comandos con la ventana abierta: flechas, ENTER para bootear,"
say "E para editar la config, F1 para el firmware, ESC para salir."
say "Ctrl+Q cierra QEMU."
