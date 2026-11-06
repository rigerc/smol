package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/rigerc/smol/internal/modules"
)

// ModulesConfig uses explicit tables so unknown options remain errors.
type ModulesConfig struct {
	Colors   *ColorsConfig   `toml:"colors,omitempty"`
	Disk     *DiskConfig     `toml:"disk,omitempty"`
	Time     *TimeConfig     `toml:"time,omitempty"`
	Battery  *BatteryConfig  `toml:"battery,omitempty"`
	LocalIP  *LocalIPConfig  `toml:"localip,omitempty"`
	Packages *PackagesConfig `toml:"packages,omitempty"`
}

// ColorsConfig selects the palette swatch shape.
type ColorsConfig = modules.ColorsOptions

// DiskConfig selects the filesystem mount.
type DiskConfig = modules.DiskOptions

// TimeConfig selects a Go time layout.
type TimeConfig = modules.TimeOptions

// BatteryConfig selects a power supply entry.
type BatteryConfig = modules.BatteryOptions

// LocalIPConfig selects an interface and address family.
type LocalIPConfig = modules.LocalIPOptions

// PackagesConfig selects a file-based package database.
type PackagesConfig = modules.PackagesOptions

func defaultModules() ModulesConfig {
	o := modules.DefaultOptions()
	return ModulesConfig{Colors: &o.Colors, Disk: &o.Disk, Time: &o.Time, Battery: &o.Battery, LocalIP: &o.LocalIP, Packages: &o.Packages}
}

// Options copies configuration to independent module inputs.
func (m ModulesConfig) Options() modules.Options {
	o := modules.DefaultOptions()
	if m.Colors != nil {
		o.Colors = *m.Colors
	}
	if m.Disk != nil {
		o.Disk = *m.Disk
	}
	if m.Time != nil {
		o.Time = *m.Time
	}
	if m.Battery != nil {
		o.Battery = *m.Battery
	}
	if m.LocalIP != nil {
		o.LocalIP = *m.LocalIP
	}
	if m.Packages != nil {
		o.Packages = *m.Packages
	}
	return o
}
func (m ModulesConfig) validate() error {
	o := m.Options()
	if o.Colors.Shape != "blocks" && o.Colors.Shape != "circles" {
		return fmt.Errorf("modules.colors.shape: expected blocks or circles")
	}
	if !filepath.IsAbs(o.Disk.Mount) {
		return fmt.Errorf("modules.disk.mount: must be absolute")
	}
	if o.Time.Format == "" {
		return fmt.Errorf("modules.time.format: must not be empty")
	}
	if !modules.ValidDevice(o.Battery.Device) {
		return fmt.Errorf("modules.battery.device: must name one power supply entry")
	}
	if o.LocalIP.Family != "ipv4" && o.LocalIP.Family != "ipv6" {
		return fmt.Errorf("modules.localip.family: expected ipv4 or ipv6")
	}
	switch o.Packages.Manager {
	case "auto", "dpkg", "pacman", "apk":
		return nil
	case "rpm":
		return fmt.Errorf("modules.packages.manager: rpm requires a database library and is unsupported")
	default:
		return fmt.Errorf("modules.packages.manager: unknown manager %q", o.Packages.Manager)
	}
}

// Candidate is one configuration search location.
type Candidate struct {
	Path  string
	Found bool
}

// Search returns ordered candidates without layering.
func Search(getenv func(string) string, exists func(string) bool) []Candidate {
	var out []Candidate
	add := func(p string) {
		if p != "" {
			out = append(out, Candidate{p, exists(p)})
		}
	}
	add(DefaultPath(getenv))
	dirs := getenv("XDG_CONFIG_DIRS")
	if dirs == "" {
		dirs = "/etc/xdg"
	}
	for _, dir := range filepath.SplitList(dirs) {
		if filepath.IsAbs(dir) {
			add(filepath.Join(dir, "smol", "config.toml"))
		}
	}
	return out
}

// TOML returns an effective configuration in deterministic key order.
func TOML(c Config) (string, error) {
	defaults := modules.DefaultOptions()
	o := c.Modules.Options()
	if o.Colors == defaults.Colors {
		c.Modules.Colors = nil
	}
	if o.Disk == defaults.Disk {
		c.Modules.Disk = nil
	}
	if o.Time == defaults.Time {
		c.Modules.Time = nil
	}
	if o.Battery == defaults.Battery {
		c.Modules.Battery = nil
	}
	if o.LocalIP == defaults.LocalIP {
		c.Modules.LocalIP = nil
	}
	if o.Packages == defaults.Packages {
		c.Modules.Packages = nil
	}
	data, err := toml.Marshal(c)
	return strings.TrimSuffix(string(data), "\n"), err
}
