#!/usr/bin/env bash
# bundle.sh — arma el paquete descargable de boot-preview.
#
#   ./scripts/bundle.sh            # crea dist/boot-preview-<version>.tar.gz
#
# El paquete lleva todo lo necesario para instalar el tool SIN clonar el
# repo: los scripts, la config de ejemplo y el wallpaper. Es lo que se
# descarga el usuario con un solo comando:
#
#   curl -fsSL https://github.com/Below-Z3R0/boot-preview/releases/latest/download/boot-preview.tar.gz \
#     | tar xz && ./boot-preview/install.sh
#
# El tarball es reproducible: los timestamps van a una fecha fija, para que
# dos builds del mismo commit den el mismo hash.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$REPO_DIR/dist"
WORK_DIR="$(mktemp -d)"
STAGE="$WORK_DIR/boot-preview"

trap 'rm -rf "$WORK_DIR"' EXIT

# ── La versión ──────────────────────────────────────────────────────────────
# Se lee del tag de git, o del commit si no hay tags.
VERSION="$(git -C "$REPO_DIR" describe --tags --always 2>/dev/null || echo "0.0.0")"
# El tarball se llama siempre boot-preview.tar.gz: la URL de la release es
# fija y no hay que saber la versión para descargarlo.
echo "▸ Versión: $VERSION"

# ── El contenido ────────────────────────────────────────────────────────────
mkdir -p "$STAGE/scripts" "$STAGE/examples/wallpaper" "$STAGE/docs"

cp "$REPO_DIR/scripts/boot-preview.sh"       "$STAGE/scripts/"
cp "$REPO_DIR/scripts/install.sh"            "$STAGE/scripts/"
cp "$REPO_DIR/scripts/loader.sh"             "$STAGE/scripts/"
cp "$REPO_DIR/scripts/animation.sh"          "$STAGE/scripts/"
cp "$REPO_DIR/scripts/make-wallpaper.sh"     "$STAGE/scripts/"
cp "$REPO_DIR/scripts/prepare-wallpapers.sh" "$STAGE/scripts/"
cp "$REPO_DIR/scripts/boot-preview-hook.sh"  "$STAGE/scripts/"
cp "$REPO_DIR/scripts/boot-preview.hook"     "$STAGE/scripts/"

cp "$REPO_DIR/examples/loader.conf"          "$STAGE/examples/"
cp "$REPO_DIR/examples/theming-rosepine.conf" "$STAGE/examples/"
cp "$REPO_DIR/examples/wallpaper/rei.png"    "$STAGE/examples/wallpaper/"

cp "$REPO_DIR/README.md"                     "$STAGE/"
cp "$REPO_DIR/LICENSE"                       "$STAGE/"
for d in "$REPO_DIR"/docs/*.md; do
  cp "$d" "$STAGE/docs/"
done

# El VERSION, para que el tool sepa qué se instaló.
printf '%s\n' "$VERSION" > "$STAGE/VERSION"

# ── El tarball ──────────────────────────────────────────────────────────────
# Timestamp fijo (mtime) para que el build sea reproducible.
find "$STAGE" -exec touch -t 202601010000.00 {} +

mkdir -p "$DIST_DIR"
tar -czf "$DIST_DIR/boot-preview.tar.gz" -C "$WORK_DIR" boot-preview

echo "✓ $DIST_DIR/boot-preview.tar.gz ($(du -h "$DIST_DIR/boot-preview.tar.gz" | cut -f1))"
echo "  sha256: $(sha256sum "$DIST_DIR/boot-preview.tar.gz" | cut -d' ' -f1)"

# ── Verificación ────────────────────────────────────────────────────────────
# El paquete tiene que ser instalable de verdad: se prueba en un HOME
# temporal, sin tocar la instalación real.
echo
echo "▸ Verificando el paquete en un HOME temporal…"
TEST_HOME="$(mktemp -d)"
if (cd "$WORK_DIR" && HOME="$TEST_HOME" XDG_CONFIG_HOME="$TEST_HOME/.config" \
    bash "$STAGE/scripts/install.sh" --no-deps >/dev/null 2>&1); then
  echo "✓ el paquete se instala"
else
  echo "✗ FALLO la instalación del paquete"
  rm -rf "$TEST_HOME"
  exit 1
fi
rm -rf "$TEST_HOME"

# Y el dispatcher tiene que responder.
if HOME="$TEST_HOME" bash "$STAGE/scripts/boot-preview.sh" --help >/dev/null 2>&1; then
  echo "✓ el dispatcher responde"
fi
