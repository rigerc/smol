// Package setup implements the inline configuration wizard.
package setup

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

type invocation struct {
	path string
	help bool
}

func parse(args []string) (invocation, error) {
	var inv invocation
	supplied := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			inv.help = true
		case arg == "--config" || strings.HasPrefix(arg, "--config="):
			if supplied {
				return inv, errors.New("duplicate --config")
			}
			supplied = true
			if arg == "--config" {
				i++
				if i == len(args) {
					return inv, errors.New("--config requires a path")
				}
				inv.path = args[i]
			} else {
				inv.path = strings.TrimPrefix(arg, "--config=")
			}
			if inv.path == "" {
				return inv, errors.New("--config must not be empty")
			}
		default:
			return inv, fmt.Errorf("unexpected argument %q", arg)
		}
	}
	return inv, nil
}

func destination(inv invocation, cwd string, getenv func(string) string) (string, error) {
	path := inv.path
	if path == "" {
		if x := getenv("XDG_CONFIG_HOME"); x != "" {
			path = filepath.Join(x, "smol", "config.toml")
		} else if h := getenv("HOME"); h != "" {
			path = filepath.Join(h, ".config", "smol", "config.toml")
		}
	}
	if path == "" || controls(path) {
		return "", errors.New("cannot resolve safe config path; pass --config PATH")
	}
	if !filepath.IsAbs(path) {
		if !filepath.IsAbs(cwd) {
			return "", errors.New("startup working directory must be absolute")
		}
		path = filepath.Join(cwd, path)
	}
	return filepath.Clean(path), nil
}
func controls(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return '?'
		}
		return r
	}, s)
}
