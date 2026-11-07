package app

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rigerc/smol/internal/persistent"
)

func TestPersistentCLI(t *testing.T) {
	for _, mode := range []string{"success", "midline", "not-tty", "dumb", "tmux", "screen", "unsupported", "narrow", "unknown-size", "single", "style", "no-probe", "write-failure", "debug"} {
		t.Run(mode, func(t *testing.T) {
			env := map[string]string{"HOME": t.TempDir(), "TERM": "xterm-ghostty"}
			args := []string{"--format", "left {face} / {face} right", "--face-preset", "kitty", "--face-state", "working", "--animate-persist", "--color", "never"}
			var stdout, stderr bytes.Buffer
			probes := 0
			runtime := Runtime{FS: os.DirFS("../../testdata/x86"), StdoutIsTTY: true, Getenv: func(k string) string { return env[k] }, Persistent: func(context.Context) (persistent.Terminal, error) {
				probes++
				if mode == "unsupported" || mode == "debug" {
					return persistent.Terminal{}, errors.New("unsupported terminal")
				}
				terminal := persistent.Terminal{ImageID: 42, Columns: 80, CellWidth: 10, CellHeight: 20, Foreground: color.RGBA{R: 255, G: 255, B: 255, A: 255}}
				if mode == "narrow" {
					terminal.Columns = 2
				}
				if mode == "unknown-size" {
					terminal.CellWidth = 0
				}
				if mode == "midline" {
					terminal.Column = 3
				}
				return terminal, nil
			}}
			switch mode {
			case "not-tty":
				runtime.StdoutIsTTY = false
			case "dumb":
				env["TERM"] = "dumb"
			case "tmux":
				env["TMUX"] = "session"
			case "screen":
				env["STY"] = "session"
			case "single":
				args = append(args, "--face-state", "dead")
			case "no-probe":
				runtime.Persistent = nil
			case "style":
				args = append(args, "--config", writeConfig(t, "[style]\nface={bold=true}\n"), "--color", "always")
			case "debug":
				args = append(args, "--debug")
			}
			var out io.Writer = &stdout
			if mode == "write-failure" {
				out = failedWriter{}
			}
			code := Run(context.Background(), args, out, &stderr, runtime)
			if mode == "write-failure" {
				if code != 1 || !strings.Contains(stderr.String(), "write persistent animation") {
					t.Fatal(code, stderr.String())
				}
				return
			}
			if code != 0 {
				t.Fatal(code, stderr.String())
			}
			animated := mode == "success" || mode == "midline"
			if strings.Contains(stdout.String(), "a=a,i=42,s=3") != animated {
				t.Fatal(mode, stdout.String(), stderr.String())
			}
			if !strings.HasSuffix(stdout.String(), "\n") {
				t.Fatal("missing final newline")
			}
			if !animated && mode != "style" && strings.Contains(stdout.String(), "\x1b") {
				t.Fatal("fallback contains terminal control sequences")
			}
			if mode == "debug" && !strings.Contains(stderr.String(), "unsupported terminal") {
				t.Fatal("missing fallback diagnostic")
			}
			if (mode == "not-tty" || mode == "dumb" || mode == "tmux" || mode == "screen" || mode == "single" || mode == "style" || mode == "no-probe") && probes != 0 {
				t.Fatal("unnecessary terminal probe")
			}
		})
	}
}

func TestPersistentUsage(t *testing.T) {
	for _, args := range [][]string{
		{"--animate-persist"},
		{"--format", "{face}", "--animate-persist", "--animate"},
		{"--format", "{face}", "--animate-persist", "--duration", "0s"},
		{"--format", "{face}", "--animate-persist", "--duration", "1s"},
		{"--format", "{face}", "--animate-persist", "--shell", "bash"},
	} {
		code, _, diag := runCLI(t, map[string]string{"HOME": t.TempDir()}, true, args...)
		if code != 2 || diag == "" {
			t.Fatal(args, code, diag)
		}
	}
}

func TestSavedPersistentPreference(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		enabled, tty, animated bool
		args                   []string
	}{
		{name: "saved on", enabled: true, tty: true, animated: true},
		{name: "saved off", tty: true},
		{name: "explicit disable", enabled: true, tty: true, args: []string{"--animate-persist=false"}},
		{name: "explicit enable", tty: true, animated: true, args: []string{"--animate-persist"}},
		{name: "foreground override", enabled: true, tty: true, args: []string{"--animate", "--duration", "1ms"}},
		{name: "bash prompt", enabled: true, tty: true, args: []string{"--shell", "bash"}},
		{name: "zsh prompt", enabled: true, tty: true, args: []string{"--shell", "zsh"}},
		{name: "fish prompt", enabled: true, tty: true, args: []string{"--shell", "fish"}},
		{name: "no face", enabled: true, tty: true, args: []string{"--format", "{host}"}},
		{name: "pipe", enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := "false"
			if tc.enabled {
				value = "true"
			}
			path := writeConfig(t, "format='{face} {host}'\ncolor='never'\n[face]\npreset='kitty'\nstate='working'\nanimate-persist="+value+"\n")
			calls := 0
			runtime := Runtime{FS: os.DirFS("../../testdata/x86"), StdoutIsTTY: tc.tty, Getenv: func(string) string { return "" }, Persistent: func(context.Context) (persistent.Terminal, error) {
				calls++
				return persistent.Terminal{ImageID: 42, Columns: 80, CellWidth: 10, CellHeight: 20, Foreground: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, nil
			}}
			var out, diag bytes.Buffer
			args := append([]string{"--config", path}, tc.args...)
			code := Run(context.Background(), args, &out, &diag, runtime)
			if code != 0 || diag.Len() != 0 {
				t.Fatal(code, diag.String())
			}
			if strings.Contains(out.String(), "a=a,i=42,s=3") != tc.animated || (calls > 0) != tc.animated {
				t.Fatal("saved preference or override ignored", calls, out.String())
			}
		})
	}
}
