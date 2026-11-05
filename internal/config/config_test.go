package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	cfg, err := Parse("c.toml", []byte(`
format = "{os} / {mem}"
color = "always"

[style]
os = { fg = "red", bold = false }
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Format != "{os} / {mem}" || cfg.Color != "always" {
		t.Errorf("got %+v", cfg)
	}
	if got := cfg.Style["os"]; got.FG != "red" || got.Bold {
		t.Errorf("user style should override default for os, got %+v", got)
	}
	if got := cfg.Style["mem"]; got.FG != "yellow" {
		t.Errorf("default style for untouched modules should survive, got %+v", got)
	}
}

func TestParseEmptyFileGivesDefaults(t *testing.T) {
	cfg, err := Parse("c.toml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Format != DefaultFormat || cfg.Color != "auto" {
		t.Errorf("got %+v", cfg)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, toml, wantSub string
	}{
		{"syntax error has line", "format = \"ok\"\ncolor = \n", "c.toml:2:"},
		{"unknown key has line", "format = \"{os}\"\n\nbogus = 1\n", "c.toml:3: unknown key \"bogus\""},
		{"bad color mode", `color = "sometimes"`, "color:"},
		{"unknown module in format", `format = "{os} {memm}"`, `unknown module "memm"`},
		{"unclosed brace in format", `format = "{os"`, "unclosed"},
		{"unknown module in style", "[style]\nnope = { fg = \"red\" }\n", "style.nope"},
		{"bad style color", "[style]\nos = { fg = \"chartreuse\" }\n", "style.os.fg"},
		{"wrong type", "format = 5\n", "c.toml:1:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("c.toml", []byte(tt.toml))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err, tt.wantSub)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.toml")
	cfg, err := Load(missing, false)
	if err != nil || cfg.Format != DefaultFormat {
		t.Errorf("optional missing file: got (%+v, %v)", cfg, err)
	}
	if _, err := Load(missing, true); err == nil {
		t.Error("explicit missing file should be an error")
	}
	if cfg, err := Load("", false); err != nil || cfg.Format != DefaultFormat {
		t.Errorf("empty path: got (%+v, %v)", cfg, err)
	}
}

func TestLoadFromDisk(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(`format = "{host}"`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p, true)
	if err != nil || cfg.Format != "{host}" {
		t.Errorf("got (%+v, %v)", cfg, err)
	}
}

func TestDefaultPath(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if got := DefaultPath(env(map[string]string{"XDG_CONFIG_HOME": "/x", "HOME": "/h"})); got != "/x/smol/config.toml" {
		t.Errorf("XDG: got %q", got)
	}
	if got := DefaultPath(env(map[string]string{"HOME": "/h"})); got != "/h/.config/smol/config.toml" {
		t.Errorf("HOME: got %q", got)
	}
	if got := DefaultPath(env(nil)); got != "" {
		t.Errorf("neither: got %q", got)
	}
}

// The text printed by --print-default-config must itself be a valid config.
func TestDefaultTOMLIsValid(t *testing.T) {
	if _, err := Parse("default.toml", []byte(DefaultTOML)); err != nil {
		t.Fatalf("DefaultTOML does not validate: %v", err)
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}
