package render

import (
	"fmt"
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
	FG   string `toml:"fg"`
	Bold bool   `toml:"bold"`
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
	_, ok := colorCode(name)
	return ok
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
	if code, ok := colorCode(s.FG); ok {
		parts = append(parts, fmt.Sprint(code))
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
