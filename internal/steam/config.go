package steam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ── Los .vdf de Steam ───────────────────────────────────────────────────────
//
// Los videos comprados se registran en localconfig.vdf (el JSON
// GetStartupMovies, escapado dentro del VDF) y el elegido en config.vdf
// (bloque StartupMovie). Acá está el parseo; la lista de videos está en
// steam.go.

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
