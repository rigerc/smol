package setup

import (
	"fmt"

	"github.com/rigerc/smol/internal/brand"
)

// welcome prints once to scrollback, outside the persistent prompt renderer.
func (t *terminal) welcome() error {
	art := "smol"
	if t.runtime.Width >= 40 && (t.runtime.Rows == 0 || t.runtime.Rows >= 24) {
		rendered, err := brand.Art()
		if err != nil {
			return fmt.Errorf("render setup banner: %w", err)
		}
		art = rendered
		if !t.runtime.Basic && t.runtime.Getenv("NO_COLOR") == "" && t.runtime.Getenv("TERM") != "dumb" {
			art = brand.Gradient(art, brand.LightBackground(t.runtime.Getenv("COLORFGBG")))
		}
	}
	if _, err := fmt.Fprintln(t.out, "\n"+art); err != nil {
		return err
	}
	return t.say("\nSet up your smol configuration.\n")
}

// header reuses the banner palette in the compact persistent wizard header.
func header(runtime Runtime) string {
	text := "smol · setup"
	if runtime.Basic || runtime.Getenv("NO_COLOR") != "" || runtime.Getenv("TERM") == "dumb" {
		return text
	}
	return brand.Gradient("smol", brand.LightBackground(runtime.Getenv("COLORFGBG"))) + " · setup"
}
