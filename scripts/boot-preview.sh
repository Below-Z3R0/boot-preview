#!/usr/bin/env bash
# boot-preview — previsualizar el arranque de Linux sin reiniciar.
#
#   boot-preview loader      el menú de Limine (el bootloader)
#   boot-preview animation   el splash de Plymouth (la animación)
#
# Son dos sistemas distintos que aparecen en pantallas distintas:
#
#   Limine    → el menú donde elegís qué booteás
#   Plymouth  → el splash gráfico mientras arranca el kernel
#
# Además, el tool se instala y se mantiene desde acá:
#
#   boot-preview install [loader|animation]   # instalar
#   boot-preview verify                       # verificar la instalación
#   boot-preview update                       # actualizar
set -euo pipefail

# ── Rutas ────────────────────────────────────────────────────────────────────
SELF="$(readlink -f "${BASH_SOURCE[0]}")"
SELF_DIR="$(cd "$(dirname "$SELF")" && pwd)"

# El repo: se busca al lado del script (instalado) o en el source.
REPO_DIR=""
if [[ -f "$SELF_DIR/loader.sh" && -f "$SELF_DIR/animation.sh" ]]; then
  REPO_DIR="$SELF_DIR"
elif [[ -f "$SELF_DIR/../scripts/loader.sh" ]]; then
  REPO_DIR="$(cd "$SELF_DIR/.." && pwd)"
fi

# La config del usuario: guarda la ruta del repo (la usa loader.sh).
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/boot-preview"
BIN_DIR="$HOME/.local/bin"
LIB_DIR="$HOME/.local/lib/boot-preview"
if [[ -z "$REPO_DIR" && -f "$CONFIG_DIR/repo" ]]; then
  REPO_DIR="$(cat "$CONFIG_DIR/repo")"
  [[ -d "$REPO_DIR" ]] || REPO_DIR=""
fi

say()  { printf '  %s\n' "$*"; }
step() { printf '\n\033[1;38;2;235;194;186m▸\033[0m \033[1m%s\033[0m\n' "$*"; }
ok()   { printf '  \033[38;2;125;207;216m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[38;2;246;153;150m! %s\033[0m\n' "$*"; }

usage() {
  cat <<'EOF'
boot-preview — previsualizar el arranque de Linux sin reiniciar

USO
  boot-preview <herramienta> [comando] [opciones]

HERRAMIENTAS
  loader      El menú de Limine (el bootloader)
              Construye un ISO híbrido con tu config y lo abre en QEMU
              con KVM. El render es exacto: mismo bootloader, mismo
              wallpaper, misma paleta. No toca el disco real.

  animation   El splash de Plymouth (la animación)
              Usa el plymouthd oficial con el renderer X11: dibuja en
              una ventana sobre tu escritorio. Sin VM, sin initramfs.

INSTALACIÓN Y MANTENIMIENTO
  install [loader|animation]  Instala el tool (o solo una parte)
  verify                       Verifica que todo esté en su sitio
  update                       Trae la última versión
  hooks [install|uninstall]   El hook de pacman que avisa tras actualizar

AYUDA DE CADA UNO
  boot-preview loader --help
  boot-preview animation --help

EJEMPLOS
  boot-preview loader              # preview del menú de Limine
  boot-preview loader shot         # captura del menú a PNG
  boot-preview animation           # ventana con el splash de Plymouth
  boot-preview animation shot      # captura del splash a PNG
  boot-preview install animation   # instalar solo el módulo de Plymouth
EOF
}

# ── Verificar la instalación ────────────────────────────────────────────────
# Comprueba que los módulos, las dependencias y la config estén. Compara las
# versiones de las dependencias con el snapshot de state.json: si algo
# cambió (por ejemplo tras una actualización del sistema), lo avisa.
cmd_verify() {
  local fail=0 warn_count=0

  step "Verificando la instalación"

  echo "  Módulos"
  if [[ -x "$BIN_DIR/boot-preview" ]]; then
    printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "boot-preview" "$BIN_DIR/boot-preview"
  else
    printf '    \033[38;2;246;153;150m✗\033[0m %-22s no instalado\n' "boot-preview"
    fail=1
  fi

  local m
  for m in loader.sh animation.sh; do
    if [[ -f "$LIB_DIR/$m" ]]; then
      printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "$m" "$LIB_DIR/$m"
    else
      printf '    \033[38;2;246;153;150m-\033[0m %-22s no instalado\n' "$m"
    fi
  done

  echo
  echo "  Dependencias de Limine"
  local bin
  for bin in qemu-system-x86_64 xorriso socat magick; do
    if command -v "$bin" >/dev/null 2>&1; then
      printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "$bin" "$(command -v "$bin")"
    else
      printf '    \033[38;2;246;153;150m✗\033[0m %-22s falta (boot-preview install loader)\n' "$bin"
      fail=1
    fi
  done
  if [[ -f /usr/share/edk2/x64/OVMF_CODE.4m.fd ]]; then
    printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "edk2-ovmf" "presente"
  else
    printf '    \033[38;2;246;153;150m✗\033[0m %-22s falta (boot-preview install loader)\n' "edk2-ovmf"
    fail=1
  fi
  if [[ -e /dev/kvm ]]; then
    printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "/dev/kvm" "presente"
  else
    printf '    \033[38;2;246;153;150m!\033[0m %-22s sin KVM: la VM va por emulación (lenta)\n' "/dev/kvm"
    warn_count=$((warn_count+1))
  fi

  echo
  echo "  Dependencias de Plymouth"
  for bin in ffmpeg xdotool xhost; do
    if command -v "$bin" >/dev/null 2>&1; then
      printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "$bin" "$(command -v "$bin")"
    else
      printf '    \033[38;2;246;153;150m-\033[0m %-22s no instalado\n' "$bin"
    fi
  done
  if [[ -f /usr/lib/plymouth/renderers/x11.so ]]; then
    printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "renderers/x11.so" "presente"
  else
    printf '    \033[38;2;246;153;150m✗\033[0m %-22s falta (reinstalá plymouth)\n' "renderers/x11.so"
    fail=1
  fi

  echo
  echo "  Config"
  if [[ -f "$CONFIG_DIR/loader/limine.conf" ]]; then
    printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "loader/limine.conf" "ok"
  else
    printf '    \033[38;2;246;153;150m-\033[0m %-22s no instalado\n' "loader/limine.conf"
  fi
  if [[ -f "$CONFIG_DIR/loader/wallpaper/wallpaper.png" ]]; then
    printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "loader/wallpaper" "ok"
  else
    printf '    \033[38;2;246;153;150m-\033[0m %-22s no instalado\n' "loader/wallpaper"
  fi
  if [[ -f "$CONFIG_DIR/animation/theme" ]]; then
    printf '    \033[38;2;125;207;216m✓\033[0m %-22s %s\n' "animation/theme" "$(cat "$CONFIG_DIR/animation/theme")"
  else
    printf '    \033[38;2;246;153;150m-\033[0m %-22s sin tema (usa el del sistema)\n' "animation/theme"
  fi

  # ── Cambios desde la instalación ──
  # El snapshot de state.json tiene las versiones de las dependencias al
  # instalar. Si ahora son distintas, algo se actualizó en el sistema.
  if [[ -f "$CONFIG_DIR/state.json" ]]; then
    echo
    echo "  Cambios desde la instalación"
    local changes
    changes="$(python3 - "$CONFIG_DIR/state.json" << 'PY'
import json, subprocess, sys

try:
    state = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)

for pkg, oldver in sorted(state.get("deps", {}).items()):
    out = subprocess.run(["pacman", "-Q", pkg], capture_output=True, text=True)
    if out.returncode != 0:
        continue
    newver = out.stdout.strip().split()[-1]
    if newver != oldver:
        print(f"{pkg}\t{oldver}\t{newver}")
PY
)"
    if [[ -n "$changes" ]]; then
      local pkg old new
      while IFS=$'\t' read -r pkg old new; do
        [[ -n "$pkg" ]] || continue
        printf '    \033[38;2;246;153;150m!\033[0m %-14s %s → %s\n' "$pkg" "$old" "$new"
      done <<< "$changes"
      echo
      say "El preview usa el software del sistema, así que ya está"
      say "usando la versión nueva."
      warn_count=$((warn_count+1))
    else
      printf '    \033[38;2;125;207;216m✓\033[0m %s\n' "ninguno"
    fi
  fi

  echo
  if [[ "$fail" == "0" ]]; then
    printf '\033[1;38;2;125;207;216m✓ Listo.\033[0m'
    [[ "$warn_count" -gt 0 ]] && printf ' (%d avisos)\n' "$warn_count" || printf '\n'
  else
    printf '\033[1;38;2;246;153;150m✗ Faltan cosas.\033[0m\n'
    say "Corré: boot-preview install [loader|animation]"
    exit 1
  fi
}

# ── Actualizar ──────────────────────────────────────────────────────────────
# Trae la última versión del tool. Hay dos fuentes:
#   - El repo local, si tiene remote: git pull + reinstalar.
#   - El script suelto del repo en GitHub, si el repo no está clonado.
cmd_update() {
  step "Buscando actualizaciones"

  local remote_url="https://github.com/Below-Z3R0/boot-preview"

  # 1) Si hay un repo local con remote, se actualiza con git.
  if [[ -n "$REPO_DIR" && -d "$REPO_DIR/.git" ]] && git -C "$REPO_DIR" remote get-url origin >/dev/null 2>&1; then
    say "Repo local: $REPO_DIR"
    local before after
    before="$(git -C "$REPO_DIR" rev-parse --short HEAD)"
    if git -C "$REPO_DIR" pull --ff-only >/dev/null 2>&1; then
      after="$(git -C "$REPO_DIR" rev-parse --short HEAD)"
      if [[ "$before" == "$after" ]]; then
        printf '  \033[38;2;125;207;216m✓\033[0m ya está al día (%s)\n' "$after"
      else
        printf '  \033[38;2;125;207;216m✓\033[0m actualizado: %s → %s\n' "$before" "$after"
        say "Reinstalando los scripts..."
        bash "$REPO_DIR/scripts/install.sh" --no-deps >/dev/null 2>&1 || true
        printf '  \033[38;2;125;207;216m✓\033[0m scripts reinstallados\n'
      fi
    else
      printf '  \033[38;2;246;153;150m!\033[0m git pull falló (¿cambios locales?)\n'
    fi
    return 0
  fi

  # 2) Si no hay repo, se descarga el dispatcher suelto.
  say "Sin repo local: descargando el dispatcher de $remote_url"
  if ! command -v curl >/dev/null 2>&1; then
    printf '  \033[38;2;246;153;150m!\033[0m necesita curl\n'
    exit 1
  fi

  local tmp
  tmp="$(mktemp)"
  if ! curl -fsSL "$remote_url/raw/main/scripts/boot-preview.sh" -o "$tmp"; then
    rm -f "$tmp"
    printf '  \033[38;2;246;153;150m!\033[0m no se pudo descargar (¿sin red?)\n'
    exit 1
  fi

  # Se comprueba que el script es válido antes de pisar el instalado.
  if ! bash -n "$tmp"; then
    rm -f "$tmp"
    printf '  \033[38;2;246;153;150m!\033[0m el script descargado no es válido bash\n'
    exit 1
  fi

  install -m 755 "$tmp" "$BIN_DIR/boot-preview"
  rm -f "$tmp"
  printf '  \033[38;2;125;207;216m✓\033[0m dispatcher actualizado\n'
  say "Los módulos se actualizan con: boot-preview install"
}

# ── Hook de pacman ──────────────────────────────────────────────────────────
# Avisa cuando Plymouth, Limine o alguna dependencia de los previews se
# actualiza. Solo informa: no toca nada.
#
# El hook lee los targets por stdin (NeedsTargets) y avisa si alguno es de
# los que le importan.
cmd_hooks() {
  local action="${1:-install}"
  local hook_dir="/etc/pacman.d/hooks"
  local hook_file="$hook_dir/boot-preview.hook"
  local hook_src="${REPO_DIR:+$REPO_DIR/}scripts/boot-preview-hook.sh"
  local hook_dst="/usr/local/lib/boot-preview/boot-preview-hook.sh"

  case "$action" in
    install)
      step "Instalando el hook de pacman"
      [[ -f "$hook_src" ]] || {
        echo "No encuentro boot-preview-hook.sh" >&2
        exit 1
      }
      sudo mkdir -p "$(dirname "$hook_dst")" "$hook_dir"
      sudo install -m 755 "$hook_src" "$hook_dst"
      ok "$hook_dst"

      sudo cp "$REPO_DIR/scripts/boot-preview.hook" "$hook_file"
      ok "$hook_file"

      echo
      say "A partir de ahora, cuando se actualice Plymouth, Limine o una"
      say "dependencia, pacman va a avisar que conviene revalidar."
      ;;
    uninstall|remove)
      step "Quitando el hook de pacman"
      sudo rm -f "$hook_file" "$hook_dst"
      ok "eliminados"
      ;;
    status)
      step "Estado del hook de pacman"
      if [[ -f "$hook_file" ]]; then
        ok "$hook_file"
        say "disparadores:"
        grep -E "^Target" "$hook_file" | sed 's/^/    /'
      else
        say "no instalado (boot-preview hooks install)"
      fi
      ;;
    *)
      echo "Uso: boot-preview hooks [install|uninstall|status]" >&2
      exit 1
      ;;
  esac
}

# ── Dispatch ────────────────────────────────────────────────────────────────
case "${1:-}" in
  loader)
    shift
    script="${REPO_DIR:+$REPO_DIR/}scripts/loader.sh"
    if [[ ! -f "$script" ]]; then
      script="$(dirname "$SELF")/loader.sh"
    fi
    [[ -f "$script" ]] || {
      echo "No encuentro loader.sh. Corré: boot-preview setup" >&2
      exit 1
    }
    exec bash "$script" "$@"
    ;;

  animation)
    shift
    script="${REPO_DIR:+$REPO_DIR/}scripts/animation.sh"
    if [[ ! -f "$script" ]]; then
      script="$(dirname "$SELF")/animation.sh"
    fi
    [[ -f "$script" ]] || {
      echo "No encuentro animation.sh. Corré: boot-preview setup" >&2
      exit 1
    }
    exec bash "$script" "$@"
    ;;

  install|setup)
    shift
    script="${REPO_DIR:+$REPO_DIR/}scripts/install.sh"
    [[ -f "$script" ]] || {
      echo "No encuentro install.sh. Bajá el repo y corré:" >&2
      echo "    ./scripts/install.sh" >&2
      exit 1
    }
    exec bash "$script" "$@"
    ;;

  verify|doctor)
    cmd_verify
    ;;

  update)
    cmd_update
    ;;

  hooks)
    shift
    cmd_hooks "${1:-install}"
    ;;

  help|-h|--help)
    usage
    ;;

  "")
    usage
    ;;

  *)
    echo "Herramienta desconocida: $1" >&2
    echo >&2
    usage
    exit 1
    ;;
esac
