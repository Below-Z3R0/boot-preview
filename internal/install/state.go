package install

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
)

// watched son los paquetes de los que se guarda la versión, para detectar
// que el sistema se actualizó.
var watched = []string{
	"qemu-desktop", "libisoburn", "edk2-ovmf", "imagemagick", "socat",
	"libvips", "ffmpeg", "xdotool", "xorg-xhost",
	"plymouth", "limine", "xorg-server-xvfb",
}

// state es el snapshot que escribe install.
type state struct {
	Version    string            `json:"version"`
	UpdatedAt  string            `json:"updated_at"`
	Components []string          `json:"components"`
	Deps       map[string]string `json:"deps"`
	Repo       string            `json:"repo"`
}

func writeState(paths cli.ToolPaths, what string) error {
	cli.Step("4/4  Estado")

	deps := map[string]string{}
	for _, pkg := range watched {
		if v := cli.PackageVersion(pkg); v != "" {
			deps[pkg] = v
		}
	}

	st := state{
		Version:    Version(),
		UpdatedAt:  today(),
		Components: []string{what},
		Deps:       deps,
		Repo:       paths.Repo,
	}
	if what == "all" {
		st.Components = []string{"animation", "loader"}
	}

	// Se conserva lo que ya había: el snapshot es acumulativo.
	if data, err := os.ReadFile(filepath.Join(paths.Config, "state.json")); err == nil {
		var old state
		if json.Unmarshal(data, &old) == nil {
			for _, c := range old.Components {
				if !cli.Contains(st.Components, c) {
					st.Components = append(st.Components, c)
				}
			}
			if old.Repo != "" {
				st.Repo = old.Repo
			}
		}
	}
	sortStrings(st.Components)

	encoded, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	dest := filepath.Join(paths.Config, "state.json")
	if err := os.WriteFile(dest, encoded, 0o644); err != nil {
		return err
	}
	cli.Ok("%s (%d dependencias registradas)", dest, len(deps))
	return nil
}
