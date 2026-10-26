// Package cli tiene lo que todos los comandos comparten.
//
// Son tres cosas:
//
//   - la salida: los colores y los mensajes (Say, Step, Ok, Warn)
//   - las rutas: dónde está la config, el binario y las capturas (ToolPaths)
//   - el sistema: correr comandos, copiar archivos, medir tamaños
//
// Los paths son de boot-preview, no genéricos: si el tool se renombra o
// cambia de layout, se cambia acá y en un solo lugar.
//
// El paquete se divide por tema, y ese es el orden de lectura:
//
//	output.go   los mensajes y los colores (Say, Step, Ok, Warn)
//	paths.go    ToolPaths: dónde está cada cosa
//	system.go   correr comandos, tiempos, entorno (Run, Timestamp...)
//	files.go    copiar, medir, parsear (CopyFile, ParseRes, SamePath...)
package cli
