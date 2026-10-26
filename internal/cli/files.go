package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CopyFile copia un archivo, creando los directorios que falten.
//
// Preserva los permisos del origen. Es la versión que usan todos los
// paquetes: si un destino está en una carpeta que no existe (por ejemplo
// /run/plymouth/themes), el error tiene que ser claro, no un "no such
// file or directory" a medio camino.
func CopyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(src); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(dest, data, mode)
}

// CopyDir copia una carpeta recursivamente.
func CopyDir(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return CopyFile(src, dest)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			if err := CopyDir(from, to); err != nil {
				return err
			}
			continue
		}
		if err := CopyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

// FileSize devuelve el tamaño de un archivo en un formato legible ("12 MB").
func FileSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	return HumanSize(info.Size())
}

// HumanSize formatea un tamaño en bytes.
//
// Compacto y sin decimales en KB: el tamaño de una captura o de un tema
// se lee mejor así ("653K", "45M") que con "653 KB" o "45.2 MB".
func HumanSize(bytes int64) string {
	const unit = 1024
	size := float64(bytes)
	if size < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	if size < unit*unit {
		return fmt.Sprintf("%.0fK", size/unit)
	}
	if size < unit*unit*unit {
		return fmt.Sprintf("%.1fM", size/(unit*unit))
	}
	// Un tema con muchos frames pasa de 1 GB.
	return fmt.Sprintf("%.1fG", size/(unit*unit*unit))
}

// DirSize devuelve el tamaño total de una carpeta.
func DirSize(path string) string {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return HumanSize(total)
}

// ParseRes parsea una resolución "1920x1080".
//
// Devuelve ancho, alto y un error: si no se puede parsear, el error
// tiene que decir cuál era el valor.
func ParseRes(res string) (int, int, error) {
	parts := strings.Split(strings.ToLower(res), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("resolución inválida: %q (se espera AnchoxAlto)", res)
	}
	width, err := parseInt(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("resolución inválida: %q", res)
	}
	height, err := parseInt(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("resolución inválida: %q", res)
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("resolución inválida: %q", res)
	}
	return width, height, nil
}

// parseInt convierte un string a un entero positivo.
//
// strconv hace el trabajo; lo que se agrega es el rechazo de 0 y de los
// negativos ("0x0" no es una resolución).
func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("no es un número positivo: %q", s)
	}
	return n, nil
}

// ImageSize devuelve el tamaño de una imagen, en píxeles.
//
// Usa vips o ImageMagick, los dos que ya son dependencia del tool.
func ImageSize(path string) (int, int) {
	if HaveCommand("vipsheader") {
		w, errW := Run("vipsheader", "-f", "width", path)
		h, errH := Run("vipsheader", "-f", "height", path)
		if errW == nil && errH == nil {
			width, _ := parseInt(strings.TrimSpace(w))
			height, _ := parseInt(strings.TrimSpace(h))
			if width > 0 && height > 0 {
				return width, height
			}
		}
	}
	if HaveCommand("magick") {
		out, err := Run("magick", "identify", "-format", "%w %h", path)
		if err == nil {
			fields := strings.Fields(out)
			if len(fields) == 2 {
				width, _ := parseInt(fields[0])
				height, _ := parseInt(fields[1])
				return width, height
			}
		}
	}
	return 0, 0
}

// FileExists dice si una ruta existe.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Contains dice si la lista tiene ese elemento.
func Contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// ReplaceFile deja dest con el contenido de src, de forma atómica.
//
// Copiar directo sobre un binario que se está ejecutando da "text file
// busy": el kernel no deja abrirlo para escritura. Escribir a un
// temporal y renombrar no toca el archivo en uso.
func ReplaceFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".boot-preview-replace.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

// SamePath dice si dos rutas son el mismo archivo.
//
// Primero por inodo (os.SameFile): resuelve los symlinks de una. Si
// alguna no existe, compara las rutas reales.
func SamePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if infoA, errA := os.Stat(a); errA == nil {
		if infoB, errB := os.Stat(b); errB == nil {
			return os.SameFile(infoA, infoB)
		}
	}
	realA, errA := filepath.EvalSymlinks(a)
	realB, errB := filepath.EvalSymlinks(b)
	if errA == nil && errB == nil {
		return realA == realB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
