package config

import (
	"strings"
	"testing"
)

func TestIconStyleRoundTrip(t *testing.T) {
	text := `format = "{mem}"
[icons]
mode = "always"
set = "ascii"
separator = ": "
[icons.overrides]
mem = "RAM"
face = ""
[style.mem]
fg = "#112233"
bg = "index:255"
bold = true
dim = true
italic = true
underline = true
reverse = true
`
	cfg, err := Parse("icons.toml", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := TOML(cfg)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse("again.toml", []byte(serialized))
	if err != nil {
		t.Fatal(err)
	}
	if again.Icons.Mode != "always" || again.Icons.Overrides["mem"] != "RAM" || again.Style["mem"] != cfg.Style["mem"] {
		t.Fatal(serialized)
	}
	for _, invalid := range []struct{ text, field string }{{"[icons]\nmode='bad'", "icons.mode"}, {"[icons]\nset='bad'", "icons.set"}, {"[icons.overrides]\nnope='x'", "icons.overrides.nope"}, {"[icons]\nunknown=true", "unknown"}, {"[style.mem]\nbg='#bad'", "style.mem.bg"}, {"[style.mem]\nfg='index:256'", "style.mem.fg"}} {
		if _, err := Parse("bad.toml", []byte(invalid.text)); err == nil || !strings.Contains(err.Error(), invalid.field) {
			t.Fatalf("%s: %v", invalid.text, err)
		}
	}
}
