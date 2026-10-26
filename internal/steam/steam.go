// Package steam lee las animaciones de inicio de Steam.
//
// Las que se compran con puntos de Steam no están en steamui/movies/:
// están en config/communityitemscache/startupmovies/<item_id>_<hash>.webm.
// El nombre real ("Elden Ring Ranni") está en el JSON GetStartupMovies de
// localconfig.vdf, y cuál está activa se guarda en config.vdf.
package steam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
					Active:    samePath(real, activePath),
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
				Active: samePath(real, activePath),
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

// startupMovie es un item del JSON GetStartupMovies.
type startupMovie struct {
	CommunityItemID string `json:"communityitemid"`
	Name            string `json:"name"`
	ItemTitle       string `json:"item_title"`
	MovieWebmLocal  string `json:"movie_webm_local_path"`
}

// parseStartupMovies saca el JSON GetStartupMovies del localconfig.vdf.
//
// El JSON viene escapado dentro del VDF: las comillas están como \" .
var startupMoviesRe = regexp.MustCompile(`(?m)"GetStartupMovies"\s+"(.*?)"\s*$`)

func parseStartupMovies(content string) []startupMovie {
	match := startupMoviesRe.FindStringSubmatch(content)
	if match == nil {
		return nil
	}

	// Se desescapan las comillas del JSON.
	raw := strings.ReplaceAll(match[1], `\"`, `"`)

	var movies []startupMovie
	if err := json.Unmarshal([]byte(raw), &movies); err != nil {
		return nil
	}
	return movies
}

// selectedMovie devuelve la ruta de la animación que Steam usa al arrancar.
//
// La guarda en ~/.steam/steam/config/config.vdf:
//
//	"StartupMovie" { "MovieID" "31391441102" "LocalPath" "/communityitemscache/..." }
//
// OJO: el LocalPath no incluye el segmento "config/", pero el archivo vive
// en "<steam>/config/communityitemscache/...".
func selectedMovie(home string) string {
	var localPath string

	for _, root := range steamRoots(home) {
		path := filepath.Join(root, "config", "config.vdf")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		localPath = extractStartupLocalPath(string(data))
		if localPath == "" {
			continue
		}

		// Se prueban las dos variantes: con y sin el segmento "config/".
		for _, candidate := range []string{
			filepath.Join(root, "config", localPath),
			filepath.Join(root, localPath),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}

	return ""
}

// extractStartupLocalPath saca el LocalPath del bloque StartupMovie.
var (
	startupMovieRe = regexp.MustCompile(`"StartupMovie"`)
	localPathRe    = regexp.MustCompile(`"LocalPath"[ \t]*"([^"]*)"`)
)

func extractStartupLocalPath(content string) string {
	loc := startupMovieRe.FindStringIndex(content)
	if loc == nil {
		return ""
	}
	// Solo se mira el bloque StartupMovie: si no, se tomaría el LocalPath
	// de otra sección.
	rest := content[loc[1]:]
	match := localPathRe.FindStringSubmatch(rest)
	if match == nil {
		return ""
	}
	return match[1]
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

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return realPath(a) == realPath(b)
}
