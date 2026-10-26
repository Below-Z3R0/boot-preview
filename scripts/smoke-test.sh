#!/usr/bin/env bash
# smoke-test.sh — prueba el binario de boot-preview de punta a punta.
#
# Uso: scripts/smoke-test.sh [ruta/al/binario]
#
# Corre los comandos que no tocan el sistema (listados, ayuda, errores) y
# verifica que devuelvan lo esperado. Lo que necesita root o una VM se
# prueba a mano: esta batería tiene que poder correr en CI.

set -uo pipefail

BIN="${1:-/tmp/boot-preview}"
pass=0
fail=0

# clean quita los codigos de color.
clean() {
	sed -e 's/\x1b\[[0-9;]*m//g'
}

# ok <nombre> <condicion-true-si-falla>
check() {
	local name="$1"
	shift
	if "$@" >/dev/null 2>&1; then
		printf '  \033[32mok\033[0m   %s\n' "$name"
		pass=$((pass + 1))
	else
		printf '  \033[31mFALLA\033[0m %s\n' "$name"
		fail=$((fail + 1))
	fi
}

# contains <nombre> <texto-esperado> <comando...>
contains() {
	local name="$1" want="$2"
	shift 2
	local out
	out="$("$@" 2>&1 | clean)"
	if [[ "$out" == *"$want"* ]]; then
		printf '  \033[32mok\033[0m   %s\n' "$name"
		pass=$((pass + 1))
	else
		printf '  \033[31mFALLA\033[0m %s\n' "$name"
		printf '         esperaba: %s\n' "$want"
		printf '         obtuvo:  %s\n' "$(echo "$out" | head -3 | tr '\n' ' ')"
		fail=$((fail + 1))
	fi
}

# empty <nombre> <comando...>  (no debe imprimir nada)
empty() {
	local name="$1"
	shift
	local out
	out="$("$@" 2>&1 | clean)"
	if [[ -z "${out//[[:space:]]/}" ]]; then
		printf '  \033[32mok\033[0m   %s\n' "$name"
		pass=$((pass + 1))
	else
		printf '  \033[31mFALLA\033[0m %s\n' "$name"
		printf '         obtuvo: %s\n' "$(echo "$out" | head -3 | tr '\n' ' ')"
		fail=$((fail + 1))
	fi
}

# fails <nombre> <comando...>  (el comando tiene que salir con error)
fails() {
	local name="$1"
	shift
	if "$@" >/dev/null 2>&1; then
		printf '  \033[31mFALLA\033[0m %s (salió con 0)\n' "$name"
		fail=$((fail + 1))
	else
		printf '  \033[32mok\033[0m   %s\n' "$name"
		pass=$((pass + 1))
	fi
}

echo "Probando $BIN"
echo

if [[ ! -x "$BIN" ]]; then
	echo "No existe el binario en $BIN. Compilálo primero:"
	echo "  go build -o $BIN ./cmd/boot-preview"
	exit 1
fi

# ── La ayuda ─────────────────────────────────────────────────────────────
check "--help sale bien" "$BIN" --help
check "loader --help sale bien" "$BIN" loader --help
check "animation --help sale bien" "$BIN" animation --help
check "install --help sale bien" "$BIN" install --help
contains "el help raíz menciona loader" "loader" "$BIN" --help
contains "el help raíz menciona animation" "animation" "$BIN" --help

# ── Los listados ────────────────────────────────────────────────────────
check "animation sources" "$BIN" animation sources
check "animation steam-source" "$BIN" animation steam-source
contains "sources lista los temas" "plymouth" "$BIN" animation sources

# ── Los errores ─────────────────────────────────────────────────────────
# Un comando desconocido tiene que avisar, no crashear.
contains "comando desconocido avisa" "Herramienta desconocida" "$BIN" no-existe
contains "subcomando desconocido avisa" "opción desconocida" "$BIN" animation no-existe

# Un comando que necesita sudo no debe dejar la terminal colgada: se
# comprueba solo que no se cuelgue.
check "animation use sin argumentos no cuelga" "$BIN" animation use --help

# --help en un subcomando con argumento posicional: no es un nombre de
# tema ni de video.
contains "steam-convert --help muestra la ayuda" "boot-preview animation" \
	"$BIN" animation steam-convert --help
contains "loader add --help muestra la ayuda" "boot-preview loader" \
	"$BIN" loader add --help

# Los caminos de error salen con código distinto de 0.
fails "un tema que no existe falla" "$BIN" animation use no-existe
fails "un video que no existe falla" "$BIN" animation steam-convert /no/existe.webm
fails "una resolución inválida falla" "$BIN" loader shot --res basura
fails "un timeout inválido falla" "$BIN" loader shot --timeout abc

# ── El control del daemon (dentro de animation) ───────────────────────
contains "el help de animation lista status" "status" "$BIN" animation --help
contains "el help de animation lista reload" "reload" "$BIN" animation --help
contains "el help de animation lista stop" "stop" "$BIN" animation --help
contains "subcomando animation desconocido avisa" "opción desconocida" "$BIN" animation no-existe

# ── La versión ──────────────────────────────────────────────────────────
contains "version imprime algo" "boot-preview" "$BIN" version

echo
printf 'ok: %d   falla: %d\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]]
