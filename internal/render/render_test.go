package render

import (
	"context"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rigerc/smol/internal/modules"
)

func fixtureEnv(vars map[string]string) modules.Env {
	return modules.Env{
		FS:     os.DirFS("../../testdata/x86"),
		Getenv: func(k string) string { return vars[k] },
	}
}

func mustParse(t *testing.T, f string) []Token {
	t.Helper()
	toks, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse(%q): %v", f, err)
	}
	return toks
}

func TestParse(t *testing.T) {
	toks := mustParse(t, "a {os} · {{x}} {mem}")
	want := []Token{{Literal: "a "}, {Module: "os"}, {Literal: " · {x} "}, {Module: "mem"}}
	if len(toks) != len(want) {
		t.Fatalf("got %+v, want %+v", toks, want)
	}
	for i := range want {
		if toks[i] != want[i] {
			t.Errorf("token %d = %+v, want %+v", i, toks[i], want[i])
		}
	}
	if got := mustParse(t, "{ os }"); got[0].Module != "os" {
		t.Errorf("whitespace inside braces should be trimmed, got %+v", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, f := range []string{"{os", "{}", "}", "a } b", "{a b}", "{ {os} }"} {
		if _, err := Parse(f); err == nil {
			t.Errorf("Parse(%q): expected error", f)
		}
	}
}

func TestRenderPlain(t *testing.T) {
	toks := mustParse(t, "{os} · {kernel} · {uptime} · {mem}")
	got := Render(context.Background(), toks, fixtureEnv(nil), Options{})
	want := "Ubuntu 24.04.1 LTS · 6.8.0-48-generic · 3d 4h 4m · 4.6 GiB / 15.6 GiB"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestRenderDropsMissingModuleWithItsSeparator(t *testing.T) {
	// shell and user are unavailable because the env has no SHELL/USER.
	tests := []struct{ name, format, want string }{
		{"middle", "{os} · {shell} · {kernel}", "Ubuntu 24.04.1 LTS · 6.8.0-48-generic"},
		{"first", "{shell} · {os} · {kernel}", "Ubuntu 24.04.1 LTS · 6.8.0-48-generic"},
		{"last", "{os} · {kernel} · {shell}", "Ubuntu 24.04.1 LTS · 6.8.0-48-generic"},
		{"two in a row", "{os} · {shell} · {user} · {kernel}", "Ubuntu 24.04.1 LTS · 6.8.0-48-generic"},
		{"user@host", "{user}@{host}", "thinkpad"},
		{"all missing", "{shell} {user}", ""},
		{"leading and trailing literals kept", "[{shell} · {os}]", "[Ubuntu 24.04.1 LTS]"},
		{"unknown module is dropped", "{os} · {nope} · {kernel}", "Ubuntu 24.04.1 LTS · 6.8.0-48-generic"},
		{"no modules", "just text {{ok}}", "just text {ok}"},
		{"adjacent modules, no separator", "{os}{kernel}", "Ubuntu 24.04.1 LTS6.8.0-48-generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(context.Background(), mustParse(t, tt.format), fixtureEnv(nil), Options{})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderColorAndShellWrapping(t *testing.T) {
	toks := mustParse(t, "{os}")
	styles := map[string]Style{"os": {FG: "cyan", Bold: true}}
	tests := []struct {
		name  string
		shell Shell
		want  string
	}{
		{"none", ShellNone, "\x1b[1;36mUbuntu 24.04.1 LTS\x1b[0m"},
		{"fish", ShellFish, "\x1b[1;36mUbuntu 24.04.1 LTS\x1b[0m"},
		{"bash", ShellBash, `\[` + "\x1b[1;36m" + `\]Ubuntu 24.04.1 LTS\[` + "\x1b[0m" + `\]`},
		{"zsh", ShellZsh, "%{\x1b[1;36m%}Ubuntu 24.04.1 LTS%{\x1b[0m%}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Render(context.Background(), toks, fixtureEnv(nil),
				Options{Color: true, Shell: tt.shell, Styles: styles})
			if got != tt.want {
				t.Errorf("got %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestRenderNoColorMeansNoEscapes(t *testing.T) {
	got := Render(context.Background(), mustParse(t, "{os}"), fixtureEnv(nil),
		Options{Color: false, Shell: ShellBash, Styles: map[string]Style{"os": {FG: "red"}}})
	if strings.ContainsAny(got, "\x1b\\%") {
		t.Errorf("unexpected escapes in %q", got)
	}
}

func TestRenderUnstyledModuleHasNoEscapes(t *testing.T) {
	got := Render(context.Background(), mustParse(t, "{os}"), fixtureEnv(nil),
		Options{Color: true, Shell: ShellBash})
	if got != "Ubuntu 24.04.1 LTS" {
		t.Errorf("got %q", got)
	}
}

func TestEscapeValue(t *testing.T) {
	tests := []struct {
		in   string
		sh   Shell
		want string
	}{
		{`a\hb`, ShellBash, `a\\hb`},
		{"100%", ShellZsh, "100%%"},
		{`a\b%c`, ShellFish, `a\b%c`},
		{`a\b%c`, ShellNone, `a\b%c`},
		{"100%", ShellBash, "100%"},
		{`a\hb`, ShellZsh, `a\hb`},
	}
	for _, tt := range tests {
		if got := escapeValue(tt.in, tt.sh); got != tt.want {
			t.Errorf("escapeValue(%q, %s) = %q, want %q", tt.in, tt.sh, got, tt.want)
		}
	}
}

// slowModule never returns within any reasonable timeout.
type slowModule struct{ release chan struct{} }

func (slowModule) Name() string { return "slow" }
func (s slowModule) Fetch(_ context.Context, _ modules.Env) (string, error) {
	<-s.release
	return "late", nil
}

func TestRenderTimeoutDropsSlowModule(t *testing.T) {
	slow := slowModule{release: make(chan struct{})}
	defer close(slow.release)
	lookup := func(n string) (modules.Module, bool) {
		if n == "slow" {
			return modules.Module{Fetch: slow.Fetch}, true
		}
		return modules.Get(n)
	}

	toks := mustParse(t, "{os} · {slow} · {kernel}")
	start := time.Now()
	got := Render(context.Background(), toks, fixtureEnv(nil), Options{Timeout: 30 * time.Millisecond, Lookup: lookup})
	elapsed := time.Since(start)

	if want := "Ubuntu 24.04.1 LTS · 6.8.0-48-generic"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Render took %v; it should return soon after the timeout", elapsed)
	}
}

func TestRenderEmptyFilesystem(t *testing.T) {
	env := modules.Env{FS: fstest.MapFS{}, Getenv: func(string) string { return "" }}
	got := Render(context.Background(), mustParse(t, "{os} · {mem}"), env, Options{})
	if got != "" {
		t.Errorf("got %q, want empty line", got)
	}
}

func TestParseShell(t *testing.T) {
	for _, s := range []string{"none", "bash", "zsh", "fish"} {
		if _, err := ParseShell(s); err != nil {
			t.Errorf("ParseShell(%q): %v", s, err)
		}
	}
	if _, err := ParseShell("tcsh"); err == nil {
		t.Error("expected error for tcsh")
	}
}

func TestRenderRemovesControlCharacters(t *testing.T) {
	got := Render(context.Background(), mustParse(t, "{user}"), fixtureEnv(map[string]string{
		"USER": "ada\n\r\t\x1b[31m\x00\x7f\u009b",
	}), Options{})
	if got != "ada[31m" {
		t.Errorf("got %q", got)
	}
}

func TestParseRejectsControlCharacters(t *testing.T) {
	for _, format := range []string{"\n", "\r", "\t", "\x1b[0m", "\u009b", "{us\ner}"} {
		if _, err := Parse(format); err == nil {
			t.Errorf("accepted %q", format)
		}
	}
}

func TestRenderDropsValuesEmptyAfterCleaning(t *testing.T) {
	got := Render(context.Background(), mustParse(t, "{host} · {user}"), fixtureEnv(map[string]string{
		"USER": "\n\r\x00",
	}), Options{})
	if got != "thinkpad" {
		t.Errorf("got %q, want host without dangling separator", got)
	}
}
