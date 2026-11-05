package app

import (
	"bytes"
	"context"
	"github.com/rigerc/smol/internal/playback"
	"os"
	"strings"
	"testing"
)

func TestIconCLI(t *testing.T) {
	for _, tc := range []struct {
		mode  string
		tty   bool
		term  string
		shown bool
	}{{"never", true, "xterm", false}, {"auto", true, "xterm", true}, {"auto", false, "xterm", false}, {"auto", true, "dumb", false}, {"always", false, "dumb", true}} {
		code, out, diag := runCLI(t, map[string]string{"HOME": t.TempDir(), "TERM": tc.term, "NO_COLOR": "1"}, tc.tty, "--format", "{mem:percent}", "--icons", tc.mode, "--icon-set", "ascii")
		if code != 0 || strings.HasPrefix(out, "MEM ") != tc.shown || strings.Contains(out, "\x1b") {
			t.Fatalf("%+v: %d %q %s", tc, code, out, diag)
		}
	}
	for _, args := range [][]string{{"--icons", "sometimes"}, {"--icon-set", "bad"}, {"--icons", ""}} {
		if code, _, _ := runCLI(t, map[string]string{"HOME": t.TempDir()}, false, args...); code != 2 {
			t.Fatal(args, code)
		}
	}
	path := writeConfig(t, "[icons]\nmode='always'\nset='nerd-font'\n")
	code, out, _ := runCLI(t, nil, false, "--config", path, "--format", "{os}", "--icons", "never")
	if code != 0 || out != "Ubuntu 24.04.1 LTS\n" {
		t.Fatal(code, out)
	}
}

func TestAnimationMeasuresIcons(t *testing.T) {
	path := writeConfig(t, "format='{face}'\n[icons]\nmode='always'\nset='ascii'\n[icons.overrides]\nface='"+strings.Repeat("X", 40)+"'\n")
	var out, diag bytes.Buffer
	code := Run(context.Background(), []string{"--config", path, "--animate", "--face-preset", "kitty"}, &out, &diag, Runtime{FS: os.DirFS("../../testdata/x86"), Getenv: func(string) string { return "" }, StdoutIsTTY: true, Playback: playback.Options{Columns: func() int { return 30 }}})
	if code != 0 || diag.Len() != 0 || strings.Contains(out.String(), "\x1b[?25l") || !strings.HasPrefix(out.String(), strings.Repeat("X", 40)+" ") {
		t.Fatalf("%d %q %s", code, out.String(), diag.String())
	}
}
