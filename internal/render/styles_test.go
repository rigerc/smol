package render

import (
	"strings"
	"testing"

	"github.com/rigerc/smol/internal/icons"
	"github.com/rigerc/smol/internal/modules"
)

func TestExtendedColorsAndAttributes(t *testing.T) {
	for _, tc := range []struct{ color, fg, bg string }{{"red", "31", "41"}, {"bright-cyan", "96", "106"}, {"index:0", "38;5;0", "48;5;0"}, {"index:255", "38;5;255", "48;5;255"}, {"#a6E301", "38;2;166;227;1", "48;2;166;227;1"}} {
		if !ValidColor(tc.color) || colorSGR(tc.color, false) != tc.fg || colorSGR(tc.color, true) != tc.bg {
			t.Fatal(tc)
		}
	}
	for _, c := range []string{"index:-1", "index:256", "index:01", "index:+1", "index:", "#fff", "#12345g", "#1234567", "red\x1b[0m"} {
		if ValidColor(c) {
			t.Fatalf("accepted %q", c)
		}
	}
	st := Style{FG: "#010203", BG: "index:16", Bold: true, Dim: true, Italic: true, Underline: true, Reverse: true}
	if got := st.sgr(); got != "\x1b[1;2;3;4;7;38;2;1;2;3;48;5;16m" {
		t.Fatal(got)
	}
}
func TestIconsFollowTokenAvailabilityAndShellEscaping(t *testing.T) {
	vals := map[string]modules.Result{"mem": {Text: "RAM", Fields: map[string]string{"percent": "42%"}}, "os": {Text: "Ubuntu", Fields: map[string]string{"id": "ubuntu"}}}
	opts := Options{ShowIcons: true, Icons: icons.Options{Mode: "always", Set: "ascii", Separator: " ", Overrides: map[string]string{"mem": "M%\\"}}, Styles: map[string]Style{"mem": {BG: "blue", Underline: true}}, Color: true, Shell: ShellZsh}
	got := Assemble(mustParse(t, "{shell} · {mem:percent} · {mem:absent} · {mem}"), vals, opts)
	want := "\x1b[4;44m"
	if !strings.Contains(got, want) || strings.Count(got, "M%%\\") != 2 || strings.Contains(got, "· ·") || strings.Count(got, "\x1b[0m") != 2 {
		t.Fatalf("%q", got)
	}
	opts.Color = false
	if got = Assemble(mustParse(t, "{mem}"), vals, opts); got != "M%%\\ RAM" {
		t.Fatalf("%q", got)
	}
	opts.Shell = ShellBash
	if got = Assemble(mustParse(t, "{mem}"), vals, opts); got != "M%\\\\ RAM" {
		t.Fatalf("%q", got)
	}
	opts.Icons.Overrides["mem"] = ""
	if got = Assemble(mustParse(t, "{mem}"), vals, opts); got != "RAM" {
		t.Fatal(got)
	}
}
