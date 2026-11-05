// Package format provides shared display options and value formatting.
package format //nolint:revive // Shared display formatting; distinct from go/format.

import (
	"fmt"
	"math"
	"strings"
)

// Options controls human-readable values.
type Options struct {
	Size     SizeOptions     `toml:"size"`
	Duration DurationOptions `toml:"duration"`
	Percent  PercentOptions  `toml:"percent"`
	Temp     TempOptions     `toml:"temp"`
}

// SizeOptions controls size units and precision.
type SizeOptions struct {
	Unit   string `toml:"unit"`
	Digits int    `toml:"digits"`
}

// DurationOptions controls duration units and rounding.
type DurationOptions struct {
	Abbreviation    bool `toml:"abbreviation"`
	SpaceBeforeUnit bool `toml:"space-before-unit"`
	Compact         bool `toml:"compact"`
	Round           bool `toml:"round"`
}

// PercentOptions controls numbers and ASCII bars.
type PercentOptions struct {
	Digits int `toml:"digits"`
	Width  int `toml:"width"`
}

// TempOptions controls temperature units and precision.
type TempOptions struct {
	Unit   string `toml:"unit"`
	Digits int    `toml:"digits"`
}

// Default preserves the original output.
func Default() Options {
	return Options{Size: SizeOptions{"iec", -1}, Duration: DurationOptions{Abbreviation: true}, Percent: PercentOptions{Width: 10}, Temp: TempOptions{Unit: "c"}}
}

// Validate rejects unsupported units and unbounded precision.
func (o Options) Validate() error {
	if o.Size.Unit != "iec" && o.Size.Unit != "si" {
		return fmt.Errorf("display.size.unit: expected iec or si")
	}
	if o.Temp.Unit != "c" && o.Temp.Unit != "f" && o.Temp.Unit != "k" {
		return fmt.Errorf("display.temp.unit: expected c, f or k")
	}
	if o.Size.Digits < -1 || o.Size.Digits > 6 || o.Percent.Digits < 0 || o.Percent.Digits > 6 || o.Temp.Digits < 0 || o.Temp.Digits > 6 || o.Percent.Width < 1 || o.Percent.Width > 100 {
		return fmt.Errorf("display: digits out of range (-1..6 for size, 0..6 otherwise), or percent width outside 1..100")
	}
	return nil
}

// Size formats bytes; the original precision rule uses KiB, MiB and GiB.
func Size(bytes uint64, o SizeOptions) string {
	base := 1024.0
	units := []string{"KiB", "MiB", "GiB"}
	if o.Unit == "si" {
		base = 1000
		units = []string{"kB", "MB", "GB"}
	}
	value := float64(bytes) / base
	i := 0
	for value >= base && i < 2 {
		value /= base
		i++
	}
	digits := o.Digits
	if digits < 0 {
		digits = 0
		if i == 2 {
			digits = 1
		}
	}
	return fmt.Sprintf("%.*f %s", digits, value, units[i])
}

// Duration formats seconds with leading zero units omitted.
func Duration(seconds int64, o DurationOptions) string {
	if o.Round {
		seconds += 30
	}
	if seconds < 60 {
		if !o.Abbreviation {
			return "<1 minute"
		}
		return "<1m"
	}
	values := []int64{seconds / 86400, seconds % 86400 / 3600, seconds % 3600 / 60}
	short := []string{"d", "h", "m"}
	long := []string{"day", "hour", "minute"}
	var out []string
	started := false
	for i, v := range values {
		if !started && v == 0 {
			continue
		}
		started = true
		if o.Compact && v == 0 {
			continue
		}
		unit := short[i]
		space := ""
		if o.SpaceBeforeUnit {
			space = " "
		}
		if !o.Abbreviation {
			unit = long[i]
			if v != 1 {
				unit += "s"
			}
			space = " "
		}
		out = append(out, fmt.Sprintf("%d%s%s", v, space, unit))
	}
	return strings.Join(out, " ")
}

// Percent formats a ratio as a percentage and a bounded ASCII bar.
func Percent(value float64, o PercentOptions) (string, string) {
	value = math.Max(0, value)
	filled := int(math.Round(math.Min(100, value) * float64(o.Width) / 100))
	return fmt.Sprintf("%.*f%%", o.Digits, value), "[" + strings.Repeat("#", filled) + strings.Repeat("-", o.Width-filled) + "]"
}

// Temperature formats Celsius in the selected unit.
func Temperature(celsius float64, o TempOptions) string {
	unit := "°C"
	if o.Unit == "f" {
		celsius = celsius*9/5 + 32
		unit = "°F"
	}
	if o.Unit == "k" {
		celsius += 273.15
		unit = "K"
	}
	return fmt.Sprintf("%.*f%s", o.Digits, celsius, unit)
}
