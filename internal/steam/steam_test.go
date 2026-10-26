package steam

import (
	"testing"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// El parser busca este bloque dentro del localconfig.vdf y desescapa el
// JSON que viene dentro.
//
// OJO: el JSON va en UNA SOLA LINEA. El VDF de Steam lo guarda asi, y el
// regex usa `.*?`, que no cruza saltos de linea.
const moviesJSON = `[{"communityitemid":"31391441102","name":"Elden Ring Ranni","item_title":"Ranni","movie_webm_local_path":"31391441102_88576f998883aafe4ad3821ac8c0cfa4f11d7c84.webm"},{"communityitemid":"25578860771","name":"Ghost Machine","item_title":"Ghost Machine","movie_webm_local_path":"25578860771_7d6e1a0e.webm"}]`

func TestParseStartupMovies(t *testing.T) {
	content := `"UserLocalConfigStore"
{
	"Steam"
	{
		"GetStartupMovies"		"` + escapeJSON(moviesJSON) + `"
	}
}`

	movies := parseStartupMovies(content)
	if len(movies) != 2 {
		t.Fatalf("parseStartupMovies() devolvio %d peliculas, quiero 2", len(movies))
	}

	if movies[0].CommunityItemID != "31391441102" {
		t.Errorf("la primera pelicula es %q, quiero %q", movies[0].CommunityItemID, "31391441102")
	}
	if movies[0].ItemTitle != "Ranni" {
		t.Errorf("el titulo de la primera es %q, quiero %q", movies[0].ItemTitle, "Ranni")
	}
	if movies[0].MovieWebmLocal == "" {
		t.Error("la primera pelicula no tiene movie_webm_local_path")
	}
}

func TestParseStartupMoviesEmpty(t *testing.T) {
	if movies := parseStartupMovies(""); len(movies) != 0 {
		t.Errorf("parseStartupMovies(\"\") devolvio %d, quiero 0", len(movies))
	}
	if movies := parseStartupMovies("no hay nada de plymouth aca"); len(movies) != 0 {
		t.Errorf("parseStartupMovies() devolvio %d, quiero 0", len(movies))
	}
}

func TestParseStartupMoviesBadJSON(t *testing.T) {
	// JSON roto: no debe entrar en panic, solo devolver nada.
	content := `"GetStartupMovies"		"[{roto"`
	if movies := parseStartupMovies(content); len(movies) != 0 {
		t.Errorf("parseStartupMovies() con JSON roto devolvio %d, quiero 0", len(movies))
	}
}

func TestSamePath(t *testing.T) {
	if !cli.SamePath("/usr/share/plymouth", "/usr/share/plymouth/") {
		t.Error("cli.SamePath() con barra final deberia dar true")
	}
	if cli.SamePath("/usr/share/plymouth", "/usr/share/otro") {
		t.Error("cli.SamePath() con rutas distintas deberia dar false")
	}
}

func TestFileExists(t *testing.T) {
	if cli.FileExists("/este/archivo/no/existe/nada") {
		t.Error("cli.FileExists() dio true para un archivo inexistente")
	}
	if !cli.FileExists("/etc/hostname") {
		t.Error("cli.FileExists() dio false para /etc/hostname")
	}
}

func TestSteamRootsDeduplica(t *testing.T) {
	// ~/.steam/steam y ~/.local/share/Steam suelen ser el mismo lugar
	// (symlink). No tiene que aparecer dos veces.
	roots := steamRoots("/home/usuario")
	seen := map[string]bool{}
	for _, root := range roots {
		if seen[root] {
			t.Errorf("steamRoots() devolvio %q dos veces", root)
		}
		seen[root] = true
	}
}

// escapeJSON escapa las comillas para meter el JSON dentro del VDF.
//
// En el VDF real el JSON viene como [{\"communityitemid\":...}]: cada
// comilla del JSON lleva un backslash adelante. El parser los saca con
// ReplaceAll, asi que el fixture tiene que usar exactamente un backslash.
func escapeJSON(s string) string {
	out := ""
	for _, r := range s {
		if r == '"' {
			out += `\"`
			continue
		}
		out += string(r)
	}
	return out
}
