package setup

import (
	"fmt"
	"strings"

	"github.com/lsferreira42/figlet-go/figlet"
)

// welcome prints once to scrollback, outside the transient prompt renderer.
func (t *terminal) welcome() error {
	art := "smol"
	if t.runtime.Width >= 40 && (t.runtime.Rows == 0 || t.runtime.Rows >= 16) {
		parser := "terminal-color"
		if t.runtime.Basic || t.runtime.Getenv("NO_COLOR") != "" {
			parser = "terminal"
		}
		// Sunset stops from the supplied banner palette. Keep the font stable so
		// the header remains compact and readable on every wizard invocation.
		rendered, err := figlet.Render("smol",
			figlet.WithFont("small"),
			figlet.WithWidth(t.runtime.Width),
			figlet.WithJustification(0),
			figlet.WithColors(
				figlet.TrueColor{R: 255, G: 78, B: 80},
				figlet.TrueColor{R: 249, G: 132, B: 91},
				figlet.TrueColor{R: 252, G: 145, B: 58},
				figlet.TrueColor{R: 245, G: 208, B: 99},
				figlet.TrueColor{R: 254, G: 202, B: 87},
				figlet.TrueColor{R: 255, G: 179, B: 198},
				figlet.TrueColor{R: 255, G: 159, B: 243},
			),
			// WithColors selects the colored parser, so apply the explicit
			// parser last to honor NO_COLOR and numbered-input mode.
			figlet.WithParser(parser),
		)
		if err != nil {
			return fmt.Errorf("render setup banner: %w", err)
		}
		art = strings.TrimRight(rendered, "\n")
	}
	// Banner ANSI escapes are generated locally; say sanitizes user content.
	if _, err := fmt.Fprintln(t.out, "\n"+art); err != nil {
		return err
	}
	return t.say("\nSet up your smol configuration.\n")
}
