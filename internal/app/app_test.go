package app

import (
	"bytes"
	"context"
	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/playback"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runCLI(t *testing.T, env map[string]string, tty bool, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	systemDirs := t.TempDir()
	code = Run(context.Background(), args, &out, &errb, Runtime{FS: os.DirFS("../../testdata/x86"), Getenv: func(k string) string {
		if k == "XDG_CONFIG_DIRS" && env[k] == "" {
			return systemDirs
		}
		return env[k]
	}, StdoutIsTTY: tty, Version: "test"})
	return code, out.String(), errb.String()
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVersionAndList(t *testing.T) {
	if code, out, _ := runCLI(t, nil, false, "--version"); code != 0 || !strings.HasPrefix(out, "smol ") {
		t.Errorf("--version: code=%d out=%q", code, out)
	}
	code, out, _ := runCLI(t, nil, false, "--list-modules")
	if code != 0 || !strings.Contains(out, "mem: Memory usage\n") || !strings.Contains(out, "uptime: System uptime\n") {
		t.Errorf("--list-modules: code=%d out=%q", code, out)
	}
}

func TestFormatFlag(t *testing.T) {
	code, out, _ := runCLI(t, map[string]string{"HOME": t.TempDir()}, false, "--format", "hi {{there}}")
	if code != 0 || out != "hi {there}\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestOutputIsExactlyOneLine(t *testing.T) {
	_, out, _ := runCLI(t, map[string]string{"HOME": t.TempDir()}, false)
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Errorf("want exactly one trailing newline, got %q", out)
	}
}

func TestFlagErrorsExitTwo(t *testing.T) {
	for _, args := range [][]string{
		{"--format", "{nope}"},
		{"--shell", "tcsh"},
		{"--color", "sometimes"},
		{"--no-such-flag"},
	} {
		if code, _, stderr := runCLI(t, nil, false, args...); code != 2 || stderr == "" {
			t.Errorf("%v: code=%d stderr=%q, want 2 and a message", args, code, stderr)
		}
	}
}

func TestBrokenConfigNeverBreaksThePrompt(t *testing.T) {
	cfg := writeConfig(t, "format = \"{nope}\"\n")
	code, out, stderr := runCLI(t, nil, false, "--config", cfg)
	if code != 0 || stderr != "" || strings.TrimSpace(out) == "" {
		t.Errorf("code=%d stderr=%q out=%q: want fallback line, no stderr, exit 0", code, stderr, out)
	}
}

func TestCheckConfig(t *testing.T) {
	good := writeConfig(t, "format = \"{os}\"\n")
	if code, out, _ := runCLI(t, nil, false, "--config", good, "--check-config"); code != 0 || !strings.Contains(out, "config ok") {
		t.Errorf("good: code=%d out=%q", code, out)
	}
	bad := writeConfig(t, "format = \"{os}\"\nbogus = 1\n")
	code, _, stderr := runCLI(t, nil, false, "--config", bad, "--check-config")
	if code != 1 || !strings.Contains(stderr, ":2:") {
		t.Errorf("bad: code=%d stderr=%q, want exit 1 with line number", code, stderr)
	}
	if code, _, _ := runCLI(t, nil, false, "--config", "/nonexistent/x.toml", "--check-config"); code != 1 {
		t.Errorf("explicit missing config: code=%d, want 1", code)
	}
}

func TestConfigFileIsUsedAndFlagOverridesIt(t *testing.T) {
	cfg := writeConfig(t, "format = \"from config\"\n")
	if _, out, _ := runCLI(t, nil, false, "--config", cfg); out != "from config\n" {
		t.Errorf("config: got %q", out)
	}
	if _, out, _ := runCLI(t, nil, false, "--config", cfg, "--format", "from flag"); out != "from flag\n" {
		t.Errorf("flag: got %q", out)
	}
}

func TestColorEndToEnd(t *testing.T) {
	cfg := writeConfig(t, "format = \"{os}\"\n[style]\nos = { fg = \"red\" }\n")
	_, plain, _ := runCLI(t, nil, false, "--config", cfg) // piped + auto
	if strings.Contains(plain, "\x1b") {
		t.Errorf("auto on a pipe must not emit color: %q", plain)
	}
	_, colored, _ := runCLI(t, nil, false, "--config", cfg, "--color", "always")
	if !strings.Contains(colored, "\x1b[31m") {
		t.Errorf("always must emit color: %q", colored)
	}
}

func TestUseColor(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	tests := []struct {
		name string
		mode string
		vars map[string]string
		tty  bool
		want bool
	}{
		{"auto tty", "auto", nil, true, true},
		{"auto pipe", "auto", nil, false, false},
		{"auto NO_COLOR", "auto", map[string]string{"NO_COLOR": "1"}, true, false},
		{"auto dumb term", "auto", map[string]string{"TERM": "dumb"}, true, false},
		{"always pipe", "always", nil, false, true},
		{"never tty", "never", nil, true, false},
	}
	for _, tt := range tests {
		if got := useColor(tt.mode, env(tt.vars), tt.tty); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAdditionalCLIInputs(t *testing.T) {
	for _, args := range [][]string{
		{"extra"}, {"--timeout", "0s"}, {"--timeout", "-1ms"},
		{"--timeout", "bad"}, {"--color="}, {"--format", "line\nbreak"},
	} {
		if code, _, diagnostic := runCLI(t, nil, false, args...); code != 2 || diagnostic == "" {
			t.Errorf("%v: code=%d diagnostic=%q", args, code, diagnostic)
		}
	}
	if code, out, diagnostic := runCLI(t, nil, false, "--format="); code != 0 || out != "\n" || diagnostic != "" {
		t.Errorf("empty format: code=%d out=%q diagnostic=%q", code, out, diagnostic)
	}
	if code, _, _ := runCLI(t, nil, false, "--help"); code != 0 {
		t.Errorf("help exit=%d", code)
	}
	if code, out, _ := runCLI(t, nil, false, "--print-default-config"); code != 0 || !strings.Contains(out, "[style]") {
		t.Errorf("default config: code=%d out=%q", code, out)
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestOutputFailure(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--list-modules"}, {"--print-default-config"}, {"--check-config"}, {"--format", "hello"}} {
		var diagnostic bytes.Buffer
		code := Run(context.Background(), args, failedWriter{}, &diagnostic, Runtime{
			FS: os.DirFS("../../testdata/x86"), Getenv: func(string) string { return "" }, Version: "test",
		})
		if code != 1 || !strings.Contains(diagnostic.String(), "write output") {
			t.Errorf("%v: exit=%d diagnostic=%q", args, code, diagnostic.String())
		}
	}
}

func TestFaceCLI(t *testing.T) {
	options := face.Options{State: "idle", Preset: "kitty"}
	animation, err := face.Build(options)
	if err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic := runCLI(t, nil, false, "--format", "{face}/{face} {host}", "--face-preset", "kitty")
	if code != 0 || diagnostic != "" || out != animation.Still+"/"+animation.Still+" thinkpad\n" {
		t.Fatalf("exit=%d out=%q diagnostic=%q", code, out, diagnostic)
	}
	for _, flag := range []string{"--list-face-presets", "--list-face-states"} {
		code, out, diagnostic = runCLI(t, nil, false, flag)
		if code != 0 || diagnostic != "" || len(strings.Fields(out)) == 0 {
			t.Fatalf("%s: %d %q %q", flag, code, out, diagnostic)
		}
	}
	_, names, _ := runCLI(t, nil, false, "--list-modules")
	if !strings.Contains(names, "face: Configured character\n") {
		t.Fatal(names)
	}
}

func TestFaceCLIValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--face-preset", "nope"}, {"--face-state", "nope"}, {"--face-state="}, {"--face-preset="},
		{"--face-preset", "kitty", "--face-seed", "seed"},
		{"--animate"}, {"--format", "{face}", "--animate", "--shell", "bash"},
		{"--format", "{face}", "--animate", "--shell", "zsh"},
		{"--format", "{face}", "--animate", "--shell", "fish"},
		{"--duration", "1s"}, {"--animate", "--duration", "0s"}, {"--animate", "--duration", "-1s"},
	} {
		code, _, diagnostic := runCLI(t, nil, false, args...)
		if code != 2 || diagnostic == "" {
			t.Errorf("%v: exit=%d diagnostic=%q", args, code, diagnostic)
		}
	}
}

func TestFaceConfigOverrides(t *testing.T) {
	for _, tc := range []struct {
		body    string
		args    []string
		options face.Options
	}{
		{`preset="kitty"`, []string{"--face-seed="}, face.Options{State: "idle", Seed: new("")}},
		{`seed="configured"`, []string{"--face-preset", "kitty"}, face.Options{State: "idle", Preset: "kitty"}},
		{`preset="kitty"`, []string{"--face-state", "happy"}, face.Options{State: "happy", Preset: "kitty"}},
	} {
		path := writeConfig(t, "format=\"{face}\"\n[face]\n"+tc.body)
		args := append([]string{"--config", path}, tc.args...)
		code, out, diagnostic := runCLI(t, nil, false, args...)
		animation, err := face.Build(tc.options)
		if err != nil {
			t.Fatal(err)
		}
		if code != 0 || diagnostic != "" || out != animation.Still+"\n" {
			t.Errorf("%v: %d %q %q", args, code, out, diagnostic)
		}
	}
	bad := writeConfig(t, "[face]\npreset=\"nope\"\n")
	if code, _, diagnostic := runCLI(t, nil, false, "--config", bad, "--check-config"); code != 1 || !strings.Contains(diagnostic, "nope") {
		t.Fatalf("%d %q", code, diagnostic)
	}
	if code, out, diagnostic := runCLI(t, nil, false, "--config", bad); code != 0 || out == "\n" || diagnostic != "" {
		t.Fatalf("fallback: %d %q %q", code, out, diagnostic)
	}
}

func TestAnimationFallbacks(t *testing.T) {
	for _, tc := range []struct {
		tty bool
		env map[string]string
	}{{false, nil}, {true, map[string]string{"TERM": "dumb"}}, {true, nil}} {
		code, out, diagnostic := runCLI(t, tc.env, tc.tty, "--format", "{face}", "--animate")
		if code != 0 || diagnostic != "" || strings.ContainsAny(out, "\x1b\r") || strings.Count(out, "\n") != 1 {
			t.Fatalf("fallback: %d %q %q", code, out, diagnostic)
		}
	}
}

func TestAnimationCLI(t *testing.T) {
	for _, mode := range []string{"duration", "cancel", "write failure", "single frame"} {
		t.Run(mode, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan time.Time, 1)
			calls := 0
			runtime := Runtime{
				FS: os.DirFS("../../testdata/x86"), Getenv: func(string) string { return "" }, StdoutIsTTY: true, Version: "test",
				Playback: playback.Options{Columns: func() int { return 120 }, After: func(d time.Duration) <-chan time.Time {
					calls++
					if d == time.Second {
						return finished
					}
					if mode == "cancel" {
						cancel()
					}
					if mode == "duration" {
						finished <- time.Time{}
					}
					return nil
				}},
			}
			args := []string{"--format", "{face} {host}", "--face-preset", "kitty", "--animate", "--color", "never"}
			var writer io.Writer = &out
			wantCode := 0
			switch mode {
			case "duration":
				args = append(args, "--duration", "1s")
			case "cancel":
				wantCode = 130
			case "write failure":
				writer = failedWriter{}
				wantCode = 1
			case "single frame":
				args = append(args, "--face-state", "dead")
			}
			code := Run(ctx, args, writer, &diagnostic, runtime)
			if code != wantCode {
				t.Fatalf("exit=%d want=%d diagnostic=%q", code, wantCode, diagnostic.String())
			}
			if mode == "write failure" {
				if !strings.Contains(diagnostic.String(), "write animation") {
					t.Fatal(diagnostic.String())
				}
				return
			}
			if !strings.Contains(out.String(), "thinkpad") || !strings.HasSuffix(out.String(), "\n") {
				t.Fatal(out.String())
			}
			if mode == "single frame" {
				if calls != 0 || strings.Contains(out.String(), "\x1b") {
					t.Fatal("single frame played animation")
				}
			} else if !strings.HasSuffix(out.String(), "\x1b[?25h\n") {
				t.Fatal("cursor not restored")
			}
		})
	}
}

func TestFaceShellStyling(t *testing.T) {
	path := writeConfig(t, "format=\"{face}\"\n[style]\nface={fg=\"red\",bold=true}\n")
	for _, shell := range []string{"none", "bash", "zsh", "fish"} {
		code, out, diagnostic := runCLI(t, nil, false, "--config", path, "--face-preset", "kitty", "--color", "always", "--shell", shell)
		if code != 0 || diagnostic != "" || !strings.Contains(out, "\x1b[1;31m") {
			t.Fatalf("%s: %d %q %q", shell, code, out, diagnostic)
		}
		if shell == "bash" && !strings.HasPrefix(out, `\[`) {
			t.Fatal(out)
		}
		if shell == "zsh" && !strings.HasPrefix(out, "%{") {
			t.Fatal(out)
		}
	}
}
