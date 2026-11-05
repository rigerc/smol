package render

import (
	"fmt"
	"strconv"
	"strings"
)

// Shell selects how color escapes are wrapped so the shell's line editor
// knows they take up no width.
type Shell string

// Supported prompt escape conventions.
const (
	ShellNone Shell = "none"
	ShellBash Shell = "bash"
	ShellZsh  Shell = "zsh"
	ShellFish Shell = "fish"
)

// ParseShell validates a --shell value.
func ParseShell(s string) (Shell, error) {
	switch Shell(s) {
	case ShellNone, ShellBash, ShellZsh, ShellFish:
		return Shell(s), nil
	}
	return "", fmt.Errorf("unknown shell %q (want none, bash, zsh or fish)", s)
}

// Style describes how a module's value is drawn.
type Style struct {
	FG        string `toml:"fg"`
	Bold      bool   `toml:"bold"`
	BG        string `toml:"bg,omitempty"`
	Dim       bool   `toml:"dim,omitempty"`
	Italic    bool   `toml:"italic,omitempty"`
	Underline bool   `toml:"underline,omitempty"`
	Reverse   bool   `toml:"reverse,omitempty"`
}

func colorCode(name string) (int, bool) {
	for i, color := range ColorNames() {
		if color == name {
			if i < 8 {
				return 30 + i, true
			}
			return 90 + i - 8, true
		}
	}
	return 0, false
}

// ValidColor reports whether name is a color smol knows. The empty string
// means "terminal default" and is valid.
func ValidColor(name string) bool {
	if name == "" {
		return true
	}
	return colorSGR(name, false) != ""
}

// colorSGR validates structured colors before constructing terminal escapes.
func colorSGR(value string, background bool) string {
	if code, ok := colorCode(value); ok {
		if background {
			code += 10
		}
		return strconv.Itoa(code)
	}
	channel := "38"
	if background {
		channel = "48"
	}
	if index, ok := strings.CutPrefix(value, "index:"); ok {
		n, err := strconv.Atoi(index)
		if err == nil && n >= 0 && n <= 255 && strconv.Itoa(n) == index {
			return channel + ";5;" + index
		}
	}
	if len(value) == 7 && value[0] == '#' {
		n, err := strconv.ParseUint(value[1:], 16, 24)
		if err == nil {
			return fmt.Sprintf("%s;2;%d;%d;%d", channel, n>>16, (n>>8)&255, n&255)
		}
	}
	return ""
}

// ColorNames lists the accepted color names, for error messages and docs.
func ColorNames() []string {
	return []string{
		"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
		"bright-black", "bright-red", "bright-green", "bright-yellow",
		"bright-blue", "bright-magenta", "bright-cyan", "bright-white",
	}
}

// sgr returns the opening escape sequence for s, or "" if s is a no-op.
func (s Style) sgr() string {
	var parts []string
	if s.Bold {
		parts = append(parts, "1")
	}
	for _, attr := range []struct {
		enabled bool
		code    string
	}{{s.Dim, "2"}, {s.Italic, "3"}, {s.Underline, "4"}, {s.Reverse, "7"}} {
		if attr.enabled {
			parts = append(parts, attr.code)
		}
	}
	if code := colorSGR(s.FG, false); code != "" {
		parts = append(parts, code)
	}
	if code := colorSGR(s.BG, true); code != "" {
		parts = append(parts, code)
	}
	if len(parts) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

const sgrReset = "\x1b[0m"

// wrapZeroWidth marks an escape sequence as non-printing for the given shell.
func wrapZeroWidth(esc string, sh Shell) string {
	switch sh {
	case ShellBash:
		return `\[` + esc + `\]`
	case ShellZsh:
		return "%{" + esc + "%}"
	default:
		return esc
	}
}

// escapeValue protects characters that the shell would otherwise interpret
// when it expands the prompt string.
func escapeValue(v string, sh Shell) string {
	switch sh {
	case ShellBash:
		return strings.ReplaceAll(v, `\`, `\\`)
	case ShellZsh:
		return strings.ReplaceAll(v, "%", "%%")
	default:
		return v
	}
}

// paint applies style to an already-escaped value.
func paint(v string, st Style, sh Shell) string {
	open := st.sgr()
	if open == "" {
		return v
	}
	return wrapZeroWidth(open, sh) + v + wrapZeroWidth(sgrReset, sh)
}
