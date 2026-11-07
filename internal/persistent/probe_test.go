package persistent

import (
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"
)

func TestProbe(t *testing.T) {
	for _, reply := range []string{
		"\x1b_Gi=42;OK\x1b\\\x1b[2;7R\x1b]10;rgb:ffff/8080/0000\x1b\\\x1b[?1;2c",
		"\x1b_GI=0,i=42;OK\x1b\\\x1b[2;7R\x1b]10;rgb:f/8/0\x07\x1b[?1;2c",
	} {
		terminal, err := Probe(context.Background(), testTerminal(), func(_ context.Context, query string) (string, error) {
			for _, part := range []string{"a=t,i=42,f=32,s=1,v=1,q=2", "a=f,i=42,f=32,s=1,v=1;AAAAAA==", "a=d,d=I,i=42,q=2", "\x1b[6n", "\x1b]10;?", "\x1b[c"} {
				if !strings.Contains(query, part) {
					t.Fatal("missing probe command", part)
				}
			}
			return reply, nil
		})
		if err != nil || terminal.Column != 6 || terminal.Foreground.R != 255 || terminal.Foreground.G < 128 || terminal.Foreground.G > 136 || terminal.Foreground.B != 0 {
			t.Fatal(terminal, err)
		}
		if !ProbeComplete(reply) {
			t.Fatal("did not recognize final reply")
		}
	}
	terminal, err := Probe(context.Background(), testTerminal(), func(context.Context, string) (string, error) { return "\x1b_Gi=42;OK\x1b\\\x1b[1;1R", nil })
	if err != nil || terminal.Foreground != (color.RGBA{R: 224, G: 224, B: 224, A: 255}) {
		t.Fatal(terminal, err)
	}
}

func TestProbeFallback(t *testing.T) {
	for _, reply := range []string{
		"", "\x1b[?1;2c", "\x1b_Gi=42;EINVAL\x1b\\\x1b[1;1R", "\x1b_Gi=43;OK\x1b\\\x1b[1;1R",
		"\x1b_Gi=42;OK\x1b\\", "\x1b_Gi=42;OK\x1b\\\x1b[1;0R", "\x1b_Gi=42;OK\x1b\\\x1b[1;81R",
		"\x1b_Gi=42;OK\x1b\\\x1b[1;999999999999999999999999999R",
	} {
		if _, err := Probe(context.Background(), testTerminal(), func(context.Context, string) (string, error) { return reply, nil }); err == nil {
			t.Fatalf("accepted %q", reply)
		}
	}
	for _, terminal := range []Terminal{{}, {ImageID: 42, Columns: 0}} {
		if _, err := Probe(context.Background(), terminal, nil); err == nil {
			t.Fatal("accepted invalid terminal")
		}
	}
	expected := errors.New("terminal unavailable")
	if _, err := Probe(context.Background(), testTerminal(), func(context.Context, string) (string, error) { return "", expected }); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	if ProbeComplete("\x1b[1;1R") || colorComponent("bad-input") != 0 {
		t.Fatal("accepted malformed reply")
	}
}

func TestForeground(t *testing.T) {
	fallback := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	for _, tc := range []struct {
		value string
		want  color.RGBA
	}{
		{"", fallback}, {"unknown", fallback}, {"#badbadbad", fallback}, {"#zzzzzz", fallback}, {"index:-1", fallback}, {"index:256", fallback},
		{"red", color.RGBA{R: 128, A: 255}}, {"bright-cyan", color.RGBA{G: 255, B: 255, A: 255}},
		{"#1234ab", color.RGBA{R: 18, G: 52, B: 171, A: 255}},
		{"index:16", color.RGBA{A: 255}}, {"index:231", color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"index:232", color.RGBA{R: 8, G: 8, B: 8, A: 255}}, {"index:255", color.RGBA{R: 238, G: 238, B: 238, A: 255}},
	} {
		if got := Foreground(tc.value, fallback); got != tc.want {
			t.Fatal(tc.value, got, tc.want)
		}
	}
	if indexedColor(-1) != (color.RGBA{}) || indexedColor(256) != (color.RGBA{}) {
		t.Fatal("invalid palette index")
	}
	if !PlainStyle(false, false, false, false, false, "") {
		t.Fatal("plain style rejected")
	}
	for _, bits := range [][5]bool{{true, false, false, false, false}, {false, true, false, false, false}, {false, false, true, false, false}, {false, false, false, true, false}, {false, false, false, false, true}} {
		if PlainStyle(bits[0], bits[1], bits[2], bits[3], bits[4], "") {
			t.Fatal("unsupported style accepted")
		}
	}
	if PlainStyle(false, false, false, false, false, "red") {
		t.Fatal("background accepted")
	}
}
