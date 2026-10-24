#!/usr/bin/env bash
# boot-preview-hook — avisa cuando algo que boot-preview usa se actualizó.
#
# Se instala como hook de pacman en /etc/pacman.d/hooks/. Cuando se
# actualiza Plymouth o Limine (las dos piezas de las que dependen los
# previews), imprime un aviso para que el usuario sepa que conviene
# revalidar.
#
# NO modifica nada: solo informa.
#
# Los targets que dispararon el hook llegan por STDIN (NeedsTargets en el
# .hook), uno por línea.
set -euo pipefail

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/boot-preview"

# Si boot-preview no está instalado, no hay nada que avisar.
if [[ ! -d "$CONFIG_DIR" ]]; then
  exit 0
fi

# Los paquetes cuyo cambio puede afectar a los previews.
watched="plymouth limine qemu-desktop libisoburn imagemagick socat ffmpeg xdotool xorg-xhost"

# Los targets vienen por stdin (NeedsTargets).
targets=""
while read -r line; do
  [[ -n "$line" ]] && targets="$targets $line"
done

if [[ -z "$targets" ]]; then
  exit 0
fi

# Se filtran los que nos interesan.
hit=""
for t in $targets; do
  for w in $watched; do
    [[ "$t" == "$w" ]] && hit="$hit $t"
  done
done

if [[ -z "$hit" ]]; then
  exit 0
fi

printf '\n\033[38;2;235;194;186m▸\033[0m \033[1mboot-preview\033[0m: se actualizó %s\n' "${hit# }"
printf '  \033[38;2;125;207;216m→\033[0m Corré \033[1mboot-preview verify\033[0m para revalidar los previews.\n\n'
