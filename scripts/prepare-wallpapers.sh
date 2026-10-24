#!/usr/bin/env bash
# prepare-wallpapers.sh — genera las dos versiones del wallpaper.
#
# Limine NO puede difuminar una imagen: la dibuja tal cual. Así que el blur
# se aplica ANTES, a la imagen, y lo que llega a Limine es una imagen ya
# difuminada. Limine no renderiza nada: solo la muestra.
#
#   prepare-wallpapers.sh [ORIGEN] [-o CARPETA] [--blur 0x30] [--res WxH]
#
# Genera dos ficheros:
#   rei.png       la imagen real, sin blur
#   rei-blur.png  la misma, ya difuminada
#   rei-blur.png  la misma imagen, ya difuminada de antemano
#
# Con la versión difuminada, el menú se lee encima sin necesidad de panel,
# velo ni degradado de Limine — que es justo lo que se veía mal.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# El source por defecto es el wallpaper YA INSTALADO (que es donde vive
# cuando el script se instala en ~/.local/bin, sin repo al lado). Si no
# esta, se cae al del repo.
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/limine-preview"
src="${CONFIG_DIR}/wallpaper/rei.png"
[[ -f "$src" ]] || src="${REPO_DIR}/examples/wallpaper/rei.png"
out_dir="${CONFIG_DIR}/wallpaper"
blur="0x30"
res="1920x1080"

usage() {
  sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--out)    out_dir="$2"; shift 2 ;;
    --blur)      blur="$2"; shift 2 ;;
    --res)       res="$2"; shift 2 ;;
    -h|--help)   usage; exit 0 ;;
    *)           src="$1"; shift ;;
  esac
done

[[ -f "$src" ]] || { echo "No existe: $src" >&2; exit 1; }
command -v make-wallpaper >/dev/null 2>&1 || {
  echo "FALTA: make-wallpaper (en ~/.local/bin). Corré scripts/install.sh" >&2; exit 1; }

mkdir -p "$out_dir"
base="$(basename "${src%.*}")"

echo "▸ La real (más detalle, sin blur)"
make-wallpaper "$src" -o "$out_dir/$base.png" \
  --res "$res"

echo "▸ La difuminada (ya viene con el blur aplicado)"
make-wallpaper "$src" -o "$out_dir/$base-blur.png" \
  --res "$res" --blur "$blur"

printf '\n  %s\n  %s\n\n' \
  "$out_dir/$base.png" \
  "$out_dir/$base-blur.png"

# La ACTIVA es la difuminada: es la que el usuario quiere en el menu, y es la
# unica que usan tanto el preview como `apply` (antes el preview miraba
# rei.png y `apply` copiaba wallpaper.png, asi que mostraban cosas distintas).
cp -f "$out_dir/$base-blur.png" "$out_dir/wallpaper.png"

cat <<EOF
  La difuminada quedo como la activa (wallpaper.png):
      limine-preview                  → preview con ella
      limine-preview apply            → wallpaper + config al Limine real

  Para volver a la nitida:
      cp $out_dir/$base.png $out_dir/wallpaper.png
      limine-preview apply wallpaper
EOF
