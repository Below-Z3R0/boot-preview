#!/usr/bin/env bash
# make-wallpaper.sh — prepara un wallpaper para Limine.
#
# Limine NO puede difuminar una imagen: dibuja el wallpaper tal cual, con el
# escalado que toque. El blur hay que hacerlo ANTES, al preparar el fichero.
# Este script lo hace (y de paso deja la imagen en la resolucion exacta de la
# interfaz, para que no haya reescalado y se vea nitida).
#
#   make-wallpaper.sh IMAGEN [-o SALIDA] [--blur RADIO] [--res WxH]
#                             [--format png|qoi] [--dim N] [--overlay COLOR]
#
#   --blur 0x20      desenfoque gaussiano (0xRADIO). 0x0 = sin desenfoque
#   --res 1920x1080  escala a esa resolucion (Lanczos). Por defecto, la de la
#                    imagen
#   --format qoi     QOI: lossless y se decodifica mas rapido que PNG
#   --dim 0.35       velo negro uniforme encima (0 = nada, 1 = negro)
#   --overlay COLOR  velo de ese color en vez de negro
#
# Por que "--dim" y no "multiplicar el brillo": multiplicar aplasta los
# negros y la imagen se vuelve una mancha sin detalle. El velo uniforme baja
# el brillo manteniendo la estructura.
#
# Ejemplos:
#   # wallpaper difuminado y velado, a la resolucion de la interfaz
#   make-wallpaper.sh ~/Pictures/foto.png --blur 0x24 --dim 0.4 --res 1920x1080
#
#   # version nitida en QOI (la mas rapida de decodificar)
#   make-wallpaper.sh ~/Pictures/foto.png --res 1920x1080 --format qoi
set -euo pipefail

src=""
out=""
blur="0x0"
res=""
format="png"
dim=0.0
overlay="#000000"

usage() {
  sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--out)    out="$2"; shift 2 ;;
    --blur)      blur="$2"; shift 2 ;;
    --res)       res="$2"; shift 2 ;;
    --format)    format="$2"; shift 2 ;;
    --dim)       dim="$2"; shift 2 ;;
    --overlay)   overlay="$2"; shift 2 ;;
    -h|--help)   usage; exit 0 ;;
    *)           src="$1"; shift ;;
  esac
done

[[ -n "$src" ]] || { usage; exit 1; }
[[ -f "$src" ]] || { echo "No existe: $src" >&2; exit 1; }
command -v magick >/dev/null 2>&1 || { echo "FALTA: imagemagick" >&2; exit 1; }

[[ -n "$out" ]] || out="${src%.*}-limine.${format}"

# ── 1. Resolucion destino ─────────────────────────────────────────────────
if [[ -n "$res" ]]; then
  # "^" fuerza el recorte a la relacion de aspecto (como PreserveAspectCrop)
  scale_args=(-resize "${res}^" -gravity center -extent "$res")
else
  scale_args=()
fi

# ── 2. Blur ───────────────────────────────────────────────────────────────
blur_args=()
if [[ "$blur" != "0x0" && -n "$blur" ]]; then
  blur_args=(-blur "$blur")
fi

# ── 3. Velo uniforme (para que el texto del menu se lea encima) ──────────
dim_args=()
if [[ "$dim" != "0" && "$dim" != "0.0" ]]; then
  percent=$(python3 -c "print(int(float('$dim') * 100))")
  dim_args=(-fill "$overlay" -colorize "$percent")
fi

# ── 4. Composicion ───────────────────────────────────────────────────────
tmp_out="$(mktemp /tmp/limine-wp.XXXXXX.png)"
trap 'rm -f "$tmp_out"' EXIT

magick "$src" "${scale_args[@]}" "${blur_args[@]}" "${dim_args[@]}" -depth 8 -strip "$tmp_out"

# El sharpen va DESPUES del resize (devuelve el detalle que el escalado se
# come), pero antes del blur (un blur afterward lo desharia).
if [[ -n "$res" && "$blur" == "0x0" ]]; then
  magick "$tmp_out" -unsharp 0x0.75+0.75+0.01 -depth 8 -strip "$tmp_out"
fi

# ── 5. Salida ────────────────────────────────────────────────────────────
if [[ "$format" == "qoi" ]]; then
  # QOI necesita la extension .qoi para que ImageMagick elija el encoder.
  out="${out%.*}.qoi"
  magick "$tmp_out" -define qoi:codec=1 "$out"
else
  magick "$tmp_out" "PNG32:$out"
fi

printf '%s  %s  %s\n' \
  "$(magick identify -format '%wx%h' "$out")" \
  "$(du -h "$out" | cut -f1)" \
  "$out"
