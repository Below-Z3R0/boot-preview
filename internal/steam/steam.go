// Package steam lee las animaciones de inicio de Steam.
//
// Las que se compran con puntos de Steam no están en steamui/movies/:
// están en config/communityitemscache/startupmovies/<item_id>_<hash>.webm.
// El nombre real ("Elden Ring Ranni") está en el JSON GetStartupMovies de
// localconfig.vdf, y cuál está activa se guarda en config.vdf.
//
//	steam.go   las raíces y la lista de videos (Movies, DefaultVideo)
//	config.go  el parseo de los .vdf (nombres y video activo)
package steam

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// Movie es una animación de inicio.
type Movie struct {
	// Name es el nombre legible ("Elden Ring Ranni").
	Name string
	// ID es el community item id ("31391441102").
	ID string
	// Path es la ruta local del archivo .webm.
	Path string
	// Active dice si es la que Steam usa al arrancar.
	Active bool
	// Purchased dice si viene de la tienda (las del cliente no).
	Purchased bool
}

// steamRoots devuelve las raíces de Steam que existen.
//
// ~/.steam/steam y ~/.local/share/Steam suelen ser el mismo sitio (un
// symlink), así que se deduplican por ruta real.
func steamRoots(home string) []string {
	candidates := []string{
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".steam", "steam"),
	}

	var roots []string
	seen := map[string]bool{}
	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		real, err := filepath.EvalSymlinks(dir)
		if err != nil {
			real = dir
		}
		if seen[real] {
			continue
		}
		seen[real] = true
		roots = append(roots, dir)
	}
	return roots
}

// Movies devuelve todas las animaciones de inicio que hay instaladas.
//
// Las compradas primero (con su nombre real), después las que trae el
// cliente. La que está activa viene marcada.
func Movies(home string) []Movie {
	var movies []Movie
	seen := map[string]bool{}
	names := purchasedNames(home)
	activePath := selectedMovie(home)

	for _, root := range steamRoots(home) {
		// Las compradas en la tienda.
		dir := filepath.Join(root, "config", "communityitemscache", "startupmovies")
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if !strings.HasSuffix(name, ".webm") {
					continue
				}
				path := filepath.Join(dir, name)
				real := realPath(path)
				if seen[real] {
					continue
				}
				seen[real] = true

				id := strings.SplitN(name, "_", 2)[0]
				movies = append(movies, Movie{
					Name:      names[id],
					ID:        id,
					Path:      path,
					Active:    cli.SamePath(real, activePath),
					Purchased: true,
				})
			}
		}

		// Las que trae el cliente (Big Picture / Deck).
		dir = filepath.Join(root, "steamui", "movies")
		entries, err = os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".webm") || !strings.Contains(name, "startup") {
				continue
			}
			path := filepath.Join(dir, name)
			real := realPath(path)
			if seen[real] {
				continue
			}
			seen[real] = true

			movies = append(movies, Movie{
				Name:   name,
				ID:     "-",
				Path:   path,
				Active: cli.SamePath(real, activePath),
			})
		}
	}

	return movies
}

// purchasedNames devuelve el nombre real de cada animación comprada,
// indexado por su community item id.
func purchasedNames(home string) map[string]string {
	names := map[string]string{}

	for _, root := range steamRoots(home) {
		userdata := filepath.Join(root, "userdata")
		entries, err := os.ReadDir(userdata)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(userdata, entry.Name(), "config", "localconfig.vdf")
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for _, item := range parseStartupMovies(string(data)) {
				if item.CommunityItemID == "" {
					continue
				}
				name := item.Name
				if name == "" {
					name = item.ItemTitle
				}
				if name == "" {
					name = item.CommunityItemID
				}
				names[item.CommunityItemID] = name
			}
		}
	}

	return names
}

// ActiveVideo devuelve la animación que Steam usa al arrancar.
func ActiveVideo(home string) string {
	return selectedMovie(home)
}

// DefaultVideo devuelve la animación que se usaría si el usuario no
// eligió ninguna: la del cliente según la máquina.
//
//	Desktop (Big Picture) → bigpicture_startup.webm
//	Steam Deck LCD (Jupiter) → deck_startup.webm
//	Steam Deck OLED (Galileo) → oled_startup.webm
func DefaultVideo(home string) string {
	roots := steamRoots(home)
	if len(roots) == 0 {
		return ""
	}

	model := readProductName()
	wanted := "bigpicture_startup.webm"
	switch {
	case strings.Contains(model, "Jupiter"):
		wanted = "deck_startup.webm"
	case strings.Contains(model, "Galileo"):
		wanted = "oled_startup.webm"
	}

	// Se prueban TODAS las raíces: antes solo se miraba la primera, y si
	// ahí no estaba steamui/movies (instalación vieja), no encontraba
	// nada aunque la otra raíz lo tuviera.
	for _, root := range roots {
		dir := filepath.Join(root, "steamui", "movies")

		if path := filepath.Join(dir, wanted); cli.FileExists(path) {
			return path
		}
		// Si no está la del modelo, la primera que haya.
		for _, fallback := range []string{
			"bigpicture_startup.webm",
			"deck_startup.webm",
			"oled_startup.webm",
		} {
			if path := filepath.Join(dir, fallback); cli.FileExists(path) {
				return path
			}
		}
	}
	return ""
}

// readProductName lee el nombre del producto del DMI (para saber si es
// un Steam Deck).
func readProductName() string {
	const path = "/sys/devices/virtual/dmi/id/product_name"
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ── Utilidades ──────────────────────────────────────────────────────────────

func realPath(path string) string {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return real
}
