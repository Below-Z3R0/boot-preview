package animation

import (
	"fmt"

	"github.com/Below-Z3R0/boot-preview/internal/cli"
	"github.com/Below-Z3R0/boot-preview/internal/steam"
)

// ── Steam ────────────────────────────────────────────────────────────────────

// ── Steam ───────────────────────────────────────────────────────────────────

// cmdSteamSource lista las animaciones de Steam.
func cmdSteamSource() error {
	home := cli.Home()
	movies := steam.Movies(home)
	active := steam.ActiveVideo(home)

	cli.Step("Animaciones de inicio de Steam")

	if len(movies) > 0 {
		cli.Say("Compradas en la tienda:")
		for _, m := range movies {
			if !m.Purchased {
				continue
			}
			if m.Active {
				fmt.Printf("    %s %-32s (%s)  ← la activa\n", cli.Green("*"), m.Name, m.ID)
			} else {
				fmt.Printf("      %-32s (%s)\n", m.Name, m.ID)
			}
		}
		fmt.Println()
	}

	cli.Say("Del cliente (Steam Big Picture / Deck):")
	for _, m := range movies {
		if m.Purchased {
			continue
		}
		if m.Active {
			fmt.Printf("    %s %-44s ← la activa\n", cli.Green("*"), m.Name)
		} else {
			fmt.Printf("      %-44s\n", m.Name)
		}
	}
	fmt.Println()

	cli.Say("Activa: %s", orNone(active))
	cli.Say("Steam las guarda en config/communityitemscache/startupmovies/")
	return nil
}

// orNone devuelve "ninguna" si el string está vacío.
func orNone(s string) string {
	if s == "" {
		return "ninguna"
	}
	return s
}
