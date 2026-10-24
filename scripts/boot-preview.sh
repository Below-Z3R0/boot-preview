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
# Cada uno tiene su propio subcomando:
#
#   boot-preview loader --help
#   boot-preview animation --help
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
if [[ -z "$REPO_DIR" && -f "$CONFIG_DIR/repo" ]]; then
  REPO_DIR="$(cat "$CONFIG_DIR/repo")"
  [[ -d "$REPO_DIR" ]] || REPO_DIR=""
fi

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
              una ventana GTK sobre tu escritorio. Sin VM, sin initramfs.

CADA UNA
  boot-preview loader --help
  boot-preview animation --help

EJEMPLOS
  boot-preview loader              # preview del menú de Limine
  boot-preview loader shot         # captura del menú a PNG
  boot-preview animation           # ventana con el splash de Plymouth
  boot-preview animation shot      # captura del splash a PNG
EOF
}

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

  setup)
    shift
    script="${REPO_DIR:+$REPO_DIR/}scripts/install.sh"
    [[ -f "$script" ]] || {
      echo "No encuentro install.sh. Bajá el repo y corré:" >&2
      echo "    ./scripts/install.sh" >&2
      exit 1
    }
    exec bash "$script" "$@"
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
