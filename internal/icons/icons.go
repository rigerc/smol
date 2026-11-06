// Package icons resolves optional module decorations without terminal I/O.
package icons

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lrstanley/go-nf/glyphs/fa"
	"github.com/lrstanley/go-nf/glyphs/linux"
)

// Options describes the icon policy and literal per-module overrides.
type Options struct {
	Mode      string            `toml:"mode"`
	Set       string            `toml:"set"`
	Separator string            `toml:"separator"`
	Overrides map[string]string `toml:"overrides,omitempty"`
}

// Default leaves existing output undecorated.
func Default() Options { return Options{Mode: "never", Set: "nerd-font", Separator: " "} }

// Validate rejects invalid policies and unsafe terminal text.
func (o Options) Validate(known func(string) bool) error {
	switch o.Mode {
	case "auto", "always", "never":
	default:
		return fmt.Errorf("icons.mode: expected auto, always or never")
	}
	if o.Set != "nerd-font" && o.Set != "ascii" {
		return fmt.Errorf("icons.set: expected nerd-font or ascii")
	}
	if !validText(o.Separator) {
		return fmt.Errorf("icons.separator: controls and invalid UTF-8 are not allowed")
	}
	names := make([]string, 0, len(o.Overrides))
	for name := range o.Overrides {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !known(name) {
			return fmt.Errorf("icons.overrides.%s: unknown module", name)
		}
		if !validText(o.Overrides[name]) {
			return fmt.Errorf("icons.overrides.%s: controls and invalid UTF-8 are not allowed", name)
		}
	}
	return nil
}

func validText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Enabled resolves visibility independently of the color policy.
func (o Options) Enabled(isTTY bool, term string) bool {
	return o.Mode == "always" || o.Mode == "auto" && isTTY && term != "dumb"
}

// Prefix returns the glyph and separator, or nothing when suppressed.
// osID comes from the already-fetched OS result, never a fresh system read.
func (o Options) Prefix(module, osID string) string {
	glyph, overridden := o.Overrides[module]
	if !overridden {
		if o.Set == "ascii" {
			glyph = ascii(module)
		} else {
			glyph = nerd(module, osID)
		}
	}
	if glyph == "" {
		return ""
	}
	return glyph + o.Separator
}

func ascii(name string) string {
	switch name {
	case "colors":
		return "CLR"
	case "battery":
		return "BAT"
	case "cpu":
		return "CPU"
	case "desktop":
		return "DE"
	case "disk":
		return "DSK"
	case "host":
		return "HOST"
	case "init":
		return "INIT"
	case "kernel":
		return "KRN"
	case "load":
		return "LOAD"
	case "localip":
		return "IP"
	case "machine":
		return "HW"
	case "mem":
		return "MEM"
	case "os":
		return "OS"
	case "packages":
		return "PKG"
	case "processes":
		return "PROC"
	case "shell":
		return "SH"
	case "swap":
		return "SWAP"
	case "terminal":
		return "TTY"
	case "time":
		return "TIME"
	case "uptime":
		return "UP"
	case "user":
		return "USR"
	case "virt":
		return "VM"
	default:
		return ""
	}
}

func nerd(name, osID string) string {
	switch name {
	case "colors":
		return fa.Palette.String()
	case "battery":
		return fa.BatteryHalf.String()
	case "cpu":
		return fa.Microchip.String()
	case "desktop", "machine":
		return fa.Desktop.String()
	case "disk":
		return fa.HddO.String()
	case "host":
		return fa.Server.String()
	case "init", "kernel":
		return fa.Cog.String()
	case "load":
		return fa.Tachometer.String()
	case "localip":
		return fa.NetworkWired.String()
	case "mem":
		return fa.Memory.String()
	case "os":
		id := strings.ToLower(osID)
		if id == "arch" {
			id = "archlinux"
		}
		if id == "darwin" || id == "macos" {
			return fa.Apple.String()
		}
		if id == "windows" {
			return fa.Windows.String()
		}
		if glyph := linux.ByID(id); !glyph.IsZero() {
			return glyph.String()
		}
		return fa.Linux.String()
	case "packages":
		return fa.Cubes.String()
	case "processes":
		return fa.Tasks.String()
	case "shell", "terminal":
		return fa.Terminal.String()
	case "swap":
		return fa.Exchange.String()
	case "time", "uptime":
		return fa.ClockO.String()
	case "user":
		return fa.User.String()
	case "virt":
		return fa.Cube.String()
	default:
		return ""
	}
}
