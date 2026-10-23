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

# vips (libvips) es ~6x mas rapido que ImageMagick para resize+blur, pero no
# escribe QOI. Se prefiere vips y se cae a magick para esa salida.
have_vips=0
command -v vips >/dev/null 2>&1 && have_vips=1
if [[ $have_vips == 0 ]]; then
  command -v magick >/dev/null 2>&1 || { echo "FALTA: imagemagick (o vips)" >&2; exit 1; }
fi
[[ "$format" == "qoi" ]] && command -v magick >/dev/null 2>&1 || true

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
if [[ $have_vips == 1 ]]; then
  # libvips NO puede leer y escribir el mismo archivo: en operaciones con
  # acceso aleatorio (crop) revienta con "Bus error (core dumped)". Por eso
  # se alterna entre dos temporales, uno lee y el otro escribe.
  tmp_a="$(mktemp /tmp/limine-wp.XXXXXX.v)"
  tmp_b="$(mktemp /tmp/limine-wp.XXXXXX.v)"
  tmp_out="$(mktemp /tmp/limine-wp.XXXXXX.png)"
  trap 'rm -f "$tmp_a" "$tmp_b" "$tmp_out"' EXIT

  # Nota: el modulo openslide de Arch viene sin su lib y tira un warning en
  # cada invocacion. VIPS_WARNING=0 NO lo silencia (sale del loader de modulos,
  # antes de leer la variable), asi que cada llamada a vips lleva 2>/dev/null.

  cur="$tmp_a"; nxt="$tmp_b"
  swap() { local t="$cur"; cur="$nxt"; nxt="$t"; }

  # Ejecuta un paso de vips y detecta el fallo. Sin esto, `set -e` no alcanza:
  # un `cmd; swap` deja correr el swap aunque el comando haya fallado, y el
  # script termina con un PNG vacio sin decir nada.
  run_vips() {
    local desc="$1"; shift
    if ! "$@" >/dev/null 2>&1; then
      echo "  error: vips fallo en '$desc'" >&2
      exit 1
    fi
    swap
  }

  if [[ -n "$res" ]]; then
    tw="${res%x*}"; th="${res#*x}"
    sw="$(vipsheader -f width  "$src" 2>/dev/null)"; sh="$(vipsheader -f height "$src" 2>/dev/null)"
    scale="$(awk -v sw="$sw" -v sh="$sh" -v tw="$tw" -v th="$th" \
      'BEGIN{ s1=tw/sw; s2=th/sh; printf "%.6f", (s1>s2?s1:s2) }')"
    nw="$(awk -v w="$sw" -v s="$scale" 'BEGIN{ printf "%d", w*s+0.5 }')"
    nh="$(awk -v h="$sh" -v s="$scale" 'BEGIN{ printf "%d", h*s+0.5 }')"

    run_vips "resize" vips resize "$src" "$nxt" "$scale"
    # Simetrico o no, el recorte al centro solo hace falta si sobra imagen.
    if [[ "$nw" -gt "$tw" || "$nh" -gt "$th" ]]; then
      run_vips "crop" vips crop "$cur" "$nxt" "$(( (nw - tw) / 2 ))" "$(( (nh - th) / 2 ))" "$tw" "$th"
    fi
  else
    run_vips "copy" vips copy "$src" "$nxt"
  fi

  # Sharpen: DESPUES del resize (devuelve el detalle que el escalado se come)
  # y ANTES del blur (un blur afterward lo desharia).
  if [[ -n "$res" && "$blur" == "0x0" ]]; then
    run_vips "sharpen" vips sharpen "$cur" "$nxt" --sigma 0.9
  fi

  # Blur gaussiano. En ImageMagick `0xSIGMA` es radio 0 + sigma SIGMA, que es
  # exactamente el sigma de vips.
  if [[ "$blur" != "0x0" && -n "$blur" ]]; then
    sigma="${blur#0x}"
    run_vips "gaussblur" vips gaussblur "$cur" "$nxt" "$sigma"
  fi

  # Velo uniforme. Con negro alcanza un `linear` (out = in * (1-dim)); con
  # otro color hace falta mezclar, y ahi vuelve magick.
  if [[ "$dim" != "0" && "$dim" != "0.0" ]]; then
    if [[ "$overlay" == "#000000" || "$overlay" == "black" ]]; then
      factor="$(awk -v d="$dim" 'BEGIN{ printf "%.6f", 1-d }')"
      run_vips "linear" vips linear "$cur" "$nxt" "$factor" 0
    else
      percent=$(awk -v d="$dim" 'BEGIN{ printf "%d", d*100 }')
      magick "$cur" -fill "$overlay" -colorize "$percent" "$nxt" >/dev/null 2>&1; swap
    fi
  fi

  vips copy "$cur" "$tmp_out" >/dev/null 2>&1
else
  tmp_out="$(mktemp /tmp/limine-wp.XXXXXX.png)"
  trap 'rm -f "$tmp_out"' EXIT

  magick "$src" "${scale_args[@]}" "${blur_args[@]}" "${dim_args[@]}" -depth 8 -strip "$tmp_out"

  # El sharpen va DESPUES del resize (devuelve el detalle que el escalado se
  # come), pero antes del blur (un blur afterward lo desharia).
  if [[ -n "$res" && "$blur" == "0x0" ]]; then
    magick "$tmp_out" -unsharp 0x0.75+0.75+0.01 -depth 8 -strip "$tmp_out"
  fi
fi

# ── 5. Salida ────────────────────────────────────────────────────────────
if [[ "$format" == "qoi" ]]; then
  # QOI necesita la extension .qoi para que ImageMagick elija el encoder.
  # (vips no sabe escribir QOI, por eso esta rama siempre pasa por magick.)
  out="${out%.*}.qoi"
  magick "$tmp_out" -define qoi:codec=1 "$out"
elif [[ $have_vips == 1 ]]; then
  # $tmp_out ya es un PNG escrito por vips: no hace falta re-codificar.
  cp "$tmp_out" "$out"
else
  magick "$tmp_out" "PNG32:$out"
fi

printf '%s  %s  %s\n' \
  "$(if [[ $have_vips == 1 ]]; then vipsheader -f width "$out" 2>/dev/null; else magick identify -format '%w' "$out" 2>/dev/null; fi)x$(if [[ $have_vips == 1 ]]; then vipsheader -f height "$out" 2>/dev/null; else magick identify -format '%h' "$out" 2>/dev/null; fi)" \
  "$(du -h "$out" | cut -f1)" \
  "$out"
