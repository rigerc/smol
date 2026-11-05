package icons

import (
	"strings"
	"testing"

	"github.com/lrstanley/go-nf/glyphs/fa"
	"github.com/lrstanley/go-nf/glyphs/linux"
	"github.com/rigerc/smol/internal/modules"
)

func TestDefaultsAndVisibility(t *testing.T) {
	o := Default()
	if err := o.Validate(modules.Known); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"never", "auto", "always"} {
		o.Mode = mode
		for _, tty := range []bool{false, true} {
			for _, term := range []string{"dumb", "xterm", ""} {
				want := mode == "always" || mode == "auto" && tty && term != "dumb"
				if got := o.Enabled(tty, term); got != want {
					t.Fatalf("%s %t %s: %t", mode, tty, term, got)
				}
			}
		}
	}
}
func TestMappingsAndOverrides(t *testing.T) {
	o := Default()
	for _, name := range modules.Names() {
		for _, set := range []string{"ascii", "nerd-font"} {
			o.Set = set
			prefix := o.Prefix(name, "")
			if name == "face" {
				if prefix != "" {
					t.Fatal(prefix)
				}
				continue
			}
			if prefix == "" || !strings.HasSuffix(prefix, " ") {
				t.Fatalf("missing %s %s", set, name)
			}
			if set == "ascii" {
				for _, r := range prefix {
					if r > 127 {
						t.Fatal(prefix)
					}
				}
			}
		}
	}
	o.Set = "nerd-font"
	for _, tc := range []struct{ id, want string }{{"ubuntu", linux.Ubuntu.String()}, {"arch", linux.Archlinux.String()}, {"darwin", fa.Apple.String()}, {"macos", fa.Apple.String()}, {"windows", fa.Windows.String()}, {"unknown", fa.Linux.String()}} {
		if got := o.Prefix("os", tc.id); got != tc.want+" " {
			t.Fatalf("%s: %q", tc.id, got)
		}
	}
	o.Overrides = map[string]string{"os": "", "mem": "RAM", "face": "@"}
	if o.Prefix("os", "ubuntu") != "" || o.Prefix("mem", "") != "RAM " || o.Prefix("face", "") != "@ " || o.Prefix("unknown", "") != "" {
		t.Fatal("override resolution")
	}
}
func TestValidation(t *testing.T) {
	for _, change := range []func(*Options){
		func(o *Options) { o.Mode = "sometimes" }, func(o *Options) { o.Set = "emoji" }, func(o *Options) { o.Separator = "\x1b[0m" },
		func(o *Options) { o.Overrides = map[string]string{"nope": "x"} }, func(o *Options) { o.Overrides = map[string]string{"mem": "a\nb"} }, func(o *Options) { o.Overrides = map[string]string{"mem": string([]byte{255})} },
	} {
		o := Default()
		change(&o)
		if err := o.Validate(modules.Known); err == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
}
