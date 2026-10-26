# Cómo trabajar en boot-preview

## Compilar

```bash
go build -o boot-preview ./cmd/boot-preview
```

Para instalarlo en el sistema:

```bash
go build -o /tmp/boot-preview ./cmd/boot-preview
/tmp/boot-preview install
```

## Tests

```bash
go test ./...                              # los unitarios
bash scripts/smoke-test.sh ./boot-preview  # la batería de CLI (sin root)
```

La batería de smoke no necesita root: solo corre comandos que no tocan
el sistema. Para probar los que sí (la ventana de Plymouth, el ISO de
Limine) hay que installar el binario y correrlos a mano.

## El código

El tool es **un solo binario** en Go. La estructura:

```
cmd/boot-preview/    el dispatcher raíz
internal/cli/        lo que todos los comandos comparten
internal/loader/     el preview de Limine (el menú del bootloader)
internal/animation/  el preview de Plymouth (el splash)
internal/install/    el instalador
internal/verify/     la verificación post-instalación
internal/pack/       el bundle y el hook de pacman
internal/steam/      el parser de los videos de Steam
internal/wallpaper/  blur + velo + escalado
```

Cada módulo está dividido **por comando**: un archivo por comando, con
el dispatcher aparte. Cuando agregás un comando:

1. su archivo en el módulo (`<comando>.go`)
2. el caso en el `Run()` del dispatcher
3. la línea en el `usage()` del módulo
4. los tests en `<comando>_test.go` o en `regression_test.go`

## Las reglas del repo

- **`gofmt` siempre**: `gofmt -w .` antes de commitear.
- **`go vet` limpio**: `go vet ./...`.
- **Los comentarios explican el porqué**, no el qué. Si el código se
  explica solo, no lleva comentario.
- **El español va con tildes**: "se aplica", no "se aplica" sin tilde.
- **Un commit por cosa**: el mensaje dice qué cambió y por qué, no qué
  archivos se tocaron.

## Lo que no se toca nunca

- **`/boot/limine.conf`**: solo con `boot-preview loader apply`, que es
  un acto explícito del usuario. El preview trabaja sobre una copia.
- **`/etc/plymouth/plymouthd.conf`**: el preview lo monta en un
  namespace de mounts privado. El sistema no se toca.
- **El initramfs**: solo con `boot-preview animation use <tema>`.

## Cuidado con el daemon de Plymouth

`boot-preview animation` deja el `plymouthd` corriendo en background (se
pone en su propia sesión con `setsid`, así que sobrevive al cierre de la
terminal). Para cerrarlo:

```bash
boot-preview animation stop
```

El PID queda en `/tmp/boot-preview-plymouthd.pid`.
