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
	Disk     *DiskConfig     `toml:"disk,omitempty"`
	Time     *TimeConfig     `toml:"time,omitempty"`
	Battery  *BatteryConfig  `toml:"battery,omitempty"`
	LocalIP  *LocalIPConfig  `toml:"localip,omitempty"`
	Packages *PackagesConfig `toml:"packages,omitempty"`
}

// DiskConfig selects the filesystem mount.
type DiskConfig struct {
	Mount string `toml:"mount"`
}

// TimeConfig selects a Go time layout.
type TimeConfig struct {
	Format string `toml:"format"`
}

// BatteryConfig selects a power supply entry.
type BatteryConfig struct {
	Device string `toml:"device"`
}

// LocalIPConfig selects an interface and address family.
type LocalIPConfig struct {
	Interface string `toml:"interface"`
	Family    string `toml:"family"`
}

// PackagesConfig selects a file-based package database.
type PackagesConfig struct {
	Manager string `toml:"manager"`
}

// Options converts configuration to module inputs.
func (m ModulesConfig) Options() modules.Options {
	o := modules.Options{Mount: "/", TimeFormat: "15:04", Family: "ipv4", Manager: "auto"}
	if m.Disk != nil && m.Disk.Mount != "" {
		o.Mount = m.Disk.Mount
	}
	if m.Time != nil && m.Time.Format != "" {
		o.TimeFormat = m.Time.Format
	}
	if m.Battery != nil {
		o.BatteryDevice = m.Battery.Device
	}
	if m.LocalIP != nil {
		o.Interface = m.LocalIP.Interface
		if m.LocalIP.Family != "" {
			o.Family = m.LocalIP.Family
		}
	}
	if m.Packages != nil && m.Packages.Manager != "" {
		o.Manager = m.Packages.Manager
	}
	return o
}
func (m ModulesConfig) validate() error {
	o := m.Options()
	if !filepath.IsAbs(o.Mount) {
		return fmt.Errorf("modules.disk.mount: must be absolute")
	}
	if !modules.ValidDevice(o.BatteryDevice) {
		return fmt.Errorf("modules.battery.device: must name one power supply entry")
	}
	if o.Family != "ipv4" && o.Family != "ipv6" {
		return fmt.Errorf("modules.localip.family: expected ipv4 or ipv6")
	}
	switch o.Manager {
	case "auto", "dpkg", "pacman", "apk":
		return nil
	case "rpm":
		return fmt.Errorf("modules.packages.manager: rpm requires a database library and is unsupported")
	default:
		return fmt.Errorf("modules.packages.manager: unknown manager %q", o.Manager)
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
	defaults := ModulesConfig{}.Options()
	o := c.Modules.Options()
	if o.Mount == defaults.Mount {
		c.Modules.Disk = nil
	}
	if o.TimeFormat == defaults.TimeFormat {
		c.Modules.Time = nil
	}
	if o.BatteryDevice == "" {
		c.Modules.Battery = nil
	}
	if o.Interface == "" && o.Family == defaults.Family {
		c.Modules.LocalIP = nil
	}
	if o.Manager == defaults.Manager {
		c.Modules.Packages = nil
	}
	data, err := toml.Marshal(c)
	return strings.TrimSuffix(string(data), "\n"), err
}
