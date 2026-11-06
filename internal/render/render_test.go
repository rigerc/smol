package render

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rigerc/smol/internal/modules"
)

func TestRenderPalette(t *testing.T) {
	for _, shape := range []string{"blocks", "circles"} {
		for _, shell := range []Shell{ShellNone, ShellBash, ShellZsh, ShellFish} {
			t.Run(shape+"/"+string(shell), func(t *testing.T) {
				env := fixtureEnv(nil)
				moduleOpts := modules.DefaultOptions()
				moduleOpts.Colors.Shape = shape
				env.Options = &moduleOpts
				opts := Options{Color: true, Shell: shell, Styles: map[string]Style{"colors": {FG: "red", BG: "blue", Bold: true}}}
				glyph := "██"
				if shape == "circles" {
					glyph = "●"
				}
				var want []string
				for i := range 16 {
					code := 30 + i
					if i >= 8 {
						code = 90 + i - 8
					}
					open, reset := fmt.Sprintf("\x1b[%dm", code), "\x1b[0m"
					switch shell {
					case ShellBash:
						open, reset = `\[`+open+`\]`, `\[`+reset+`\]`
					case ShellZsh:
						open, reset = "%{"+open+"%}", "%{"+reset+"%}"
					}
					want = append(want, open+glyph+reset)
				}
				toks := mustParse(t, "palette {colors} done")
				if got := Render(context.Background(), toks, env, opts); got != "palette "+strings.Join(want, " ")+" done" {
					t.Fatalf("palette: %q", got)
				}
				opts.Color = false
				if got, want := Render(context.Background(), toks, env, opts), "palette "+strings.TrimSuffix(strings.Repeat(glyph+" ", 16), " ")+" done"; got != want {
					t.Fatalf("plain palette: %q, want %q", got, want)
				}
			})
		}
	}
}

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
func (s slowModule) Fetch(_ context.Context, _ modules.Env) (modules.Result, error) {
	<-s.release
	return modules.Result{Text: "late"}, nil
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

func TestAssembleFaceValues(t *testing.T) {
	toks := mustParse(t, "{face}/{face} · {host}")
	// Braces in generated faces are data, never nested format references.
	values := map[string]modules.Result{"face": {Text: `{\_%_\}`}, "host": {Text: "host"}}
	got := Assemble(toks, values, Options{})
	if want := `{\_%_\}/{\_%_\} · host`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got = Assemble(toks, values, Options{Shell: ShellZsh})
	if !strings.Contains(got, "%%") {
		t.Fatal(got)
	}
	got = Assemble(toks, values, Options{Shell: ShellBash})
	if !strings.Contains(got, `\\`) {
		t.Fatal(got)
	}
}

func TestFetchOnceForManyFrames(t *testing.T) {
	reads := 0
	opts := Options{Lookup: func(name string) (modules.Module, bool) {
		return modules.Module{Fetch: func(context.Context, modules.Env) (modules.Result, error) {
			reads++
			return modules.Result{Text: "snapshot"}, nil
		}}, name == "host"
	}}
	toks := mustParse(t, "{face} {host} {host}")
	values := Fetch(context.Background(), toks, fixtureEnv(nil), opts)
	for _, face := range []string{"a", "b", "c"} {
		values["face"] = modules.Result{Text: face}
		if got := Assemble(toks, values, opts); got != face+" snapshot snapshot" {
			t.Fatal(got)
		}
	}
	if reads != 1 {
		t.Fatalf("fetched %d times", reads)
	}
}

func TestFieldReferences(t *testing.T) {
	calls := 0
	opts := Options{Lookup: func(name string) (modules.Module, bool) {
		return modules.Module{Fetch: func(context.Context, modules.Env) (modules.Result, error) {
			calls++
			return modules.Result{Text: "text", Fields: map[string]string{"percent": "50%", "bar": "[#####-----]", "empty": "\n"}}, nil
		}}, name == "mem"
	}}
	toks := mustParse(t, "{mem} · {mem:percent} · {mem:missing} · {mem:bar} · {mem:empty}")
	if got := Render(context.Background(), toks, fixtureEnv(nil), opts); got != "text · 50% · [#####-----]" {
		t.Fatal(got)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	for _, bad := range []string{"{mem:}", "{mem:percent:extra}", "{mem:Upper}", "{mem:has_space}"} {
		if _, err := Parse(bad); err == nil {
			t.Fatal(bad)
		}
	}
}
func TestDebugTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var messages []string
	opts := Options{Timeout: time.Millisecond, Debug: func(s string) { messages = append(messages, s) }, Lookup: func(name string) (modules.Module, bool) {
		if name == "unknown" {
			return modules.Module{}, false
		}
		return modules.Module{Fetch: func(context.Context, modules.Env) (modules.Result, error) { <-release; return modules.Result{}, nil }}, true
	}}
	Render(context.Background(), mustParse(t, "{slow} {unknown}"), fixtureEnv(nil), opts)
	text := strings.Join(messages, "\n")
	if !strings.Contains(text, "slow ") || !strings.Contains(text, "timeout") || !strings.Contains(text, "unknown unknown") || !strings.Contains(text, "total ") {
		t.Fatal(text)
	}
}
