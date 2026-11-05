// Package config loads and validates smol's TOML configuration.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/format"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/render"
)

// DefaultFormat is used when no format is configured.
const DefaultFormat = "{os} · {kernel} · {uptime} · {mem}"

// Config is the on-disk configuration.
type Config struct {
	Display format.Options          `toml:"display"`
	Modules ModulesConfig           `toml:"modules"`
	Face    face.Options            `toml:"face"`
	Format  string                  `toml:"format"`
	Color   string                  `toml:"color"` // auto | always | never
	Style   map[string]render.Style `toml:"style"`
}

// Default returns the configuration used when no file exists. The default
// palette is applied first, so entries in a user's [style] table override it
// per module.
func Default() Config {
	cfg := Config{
		Format:  DefaultFormat,
		Display: format.Default(),
		Modules: defaultModules(),
		Face:    face.Options{State: "idle"},
		Color:   "auto",
		Style: map[string]render.Style{
			"os":     {FG: "cyan", Bold: true},
			"kernel": {FG: "blue"},
			"uptime": {FG: "green"},
			"mem":    {FG: "yellow"},
			"cpu":    {FG: "magenta"},
			"load":   {FG: "yellow"},
			"user":   {FG: "green", Bold: true},
			"host":   {FG: "cyan", Bold: true},
			"shell":  {FG: "white"},
		},
	}
	for _, name := range modules.Names() {
		if _, ok := cfg.Style[name]; !ok && name != "face" {
			cfg.Style[name] = render.Style{FG: "cyan"}
		}
	}
	return cfg
}

// DefaultPath returns the config file location, honoring XDG_CONFIG_HOME.
func DefaultPath(getenv func(string) string) string {
	if x := getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "smol", "config.toml")
	}
	if h := getenv("HOME"); h != "" {
		return filepath.Join(h, ".config", "smol", "config.toml")
	}
	return ""
}

// Load reads path and returns the validated configuration. Missing fields
// take their defaults. If the file does not exist, Load returns Default()
// and no error, unless mustExist is set (used when the user passed --config).
func Load(path string, mustExist bool) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	// The path is explicitly selected by the local user or their XDG/HOME settings.
	data, err := os.ReadFile(path) //nolint:gosec // G304: reading the chosen config file is intentional.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !mustExist {
			return cfg, nil
		}
		return Default(), err
	}
	return Parse(path, data)
}

// Parse decodes and validates config data. name is used in error messages.
func Parse(name string, data []byte) (Config, error) {
	cfg := Default()
	dec := toml.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Default(), decodeError(name, err)
	}
	if err := cfg.Validate(); err != nil {
		return Default(), fmt.Errorf("%s: %w", name, err)
	}
	return cfg, nil
}

func decodeError(name string, err error) error {
	var derr *toml.DecodeError
	if errors.As(err, &derr) {
		row, _ := derr.Position()
		return fmt.Errorf("%s:%d: %s", name, row, derr.Error())
	}
	var serr *toml.StrictMissingError
	if errors.As(err, &serr) {
		if len(serr.Errors) > 0 {
			row, _ := serr.Errors[0].Position()
			key := strings.Join(serr.Errors[0].Key(), ".")
			return fmt.Errorf("%s:%d: unknown key %q", name, row, key)
		}
	}
	return fmt.Errorf("%s: %w", name, err)
}

// Validate checks values that decoding alone cannot.
func (c Config) Validate() error {
	if err := c.Display.Validate(); err != nil {
		return err
	}
	if err := c.Modules.validate(); err != nil {
		return err
	}
	if err := c.Face.Validate(); err != nil {
		return fmt.Errorf("face: %w", err)
	}
	switch c.Color {
	case "auto", "always", "never":
	default:
		return fmt.Errorf("color: %q is not one of auto, always, never", c.Color)
	}
	if err := ValidateFormat(c.Format); err != nil {
		return err
	}
	names := make([]string, 0, len(c.Style))
	for n := range c.Style {
		names = append(names, n)
	}
	sort.Strings(names) // deterministic error order
	for _, n := range names {
		if !modules.Known(n) {
			return fmt.Errorf("style.%s: unknown module %q", n, n)
		}
		if fg := c.Style[n].FG; !render.ValidColor(fg) {
			return fmt.Errorf("style.%s.fg: unknown color %q (try one of: %s)",
				n, fg, strings.Join(render.ColorNames(), ", "))
		}
	}
	return nil
}

// ValidateFormat checks that format parses and only names known modules.
func ValidateFormat(format string) error {
	toks, err := render.Parse(format)
	if err != nil {
		return err
	}
	for _, n := range render.ModuleNames(toks) {
		if !modules.Known(n) {
			return fmt.Errorf("format: unknown module %q (see `smol --list-modules`)", n)
		}
	}
	for _, t := range toks {
		if t.Field == "" {
			continue
		}
		m, _ := modules.Get(t.Module)
		found := false
		for _, f := range m.Fields {
			if f.Name == t.Field {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("format: unknown field %q for module %q (see `smol --list-fields`)", t.Field, t.Module)
		}
	}
	return nil
}

// DefaultTOML is printed by `smol --print-default-config`.
const DefaultTOML = `# smol configuration
# Place at ~/.config/smol/config.toml

# Text between {module} references is kept; if a module has nothing to show,
# it is dropped together with the separator in front of it.
# Use {{ and }} for literal braces. Fields use {mem:percent}; face has no fields.
format = "{os} · {kernel} · {uptime} · {mem}"

# auto | always | never. "auto" disables color when output is not a terminal
# or NO_COLOR is set.
color = "auto"

# Optional character. Add {face} to format to show it.
[face]
state = "idle"
# preset = "kitty"
# Or choose a deterministic seed (do not combine with preset):
# seed = "smol"
# Or generate a fresh identity each run (excludes seed/preset):
# random = true

# Animation is enabled by --animate, and cannot be combined with --debug.

# Optional display controls (defaults shown).
# [display.size]
# unit = "iec" # iec | si
# digits = -1 # automatic precision, or 0..6
# [display.duration]
# abbreviation = true
# space-before-unit = false
# compact = false
# round = false
# [display.percent]
# digits = 0
# width = 10
# [display.temp]
# unit = "c" # c | f | k
# digits = 0

# Module options only apply when referenced in format.
# [modules.disk]
# mount = "/"
# [modules.time]
# format = "15:04" # Go layout; also "15:04:05" or "Mon 15:04"
# [modules.battery]
# device = "" # first battery, or e.g. BAT1
# [modules.localip]
# interface = ""
# family = "ipv4" # ipv4 | ipv6
# [modules.packages]
# manager = "auto" # auto | dpkg | pacman | apk; try --timeout 100ms

# Per-module styling. Entries here override smol's built-in palette for that
# module only. Colors: black red green yellow blue magenta cyan white
# (and bright-* variants). Use fg = "" for the terminal's default color.
[style]
os  = { fg = "cyan", bold = true }
mem = { fg = "yellow" }
`
