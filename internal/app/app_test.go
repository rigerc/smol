package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, env map[string]string, tty bool, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(context.Background(), args, &out, &errb, Runtime{FS: os.DirFS("../../testdata/x86"), Getenv: func(k string) string { return env[k] }, StdoutIsTTY: tty, Version: "test"})
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
	if code != 0 || !strings.Contains(out, "mem\n") || !strings.Contains(out, "uptime\n") {
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
