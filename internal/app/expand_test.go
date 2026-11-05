package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rigerc/smol/internal/config"
)

func TestExpansionCLI(t *testing.T) {
	for _, args := range [][]string{{"--list-fields"}, {"--list-config-paths"}, {"--print-config"}, {"--print-config", "--format", "{mem:percent}", "--color", "never"}} {
		code, out, diagnostic := runCLI(t, map[string]string{"HOME": t.TempDir()}, false, args...)
		if code != 0 || out == "" || diagnostic != "" {
			t.Fatal(args, code, out, diagnostic)
		}
		if args[0] == "--print-config" {
			if _, err := config.Parse("printed", []byte(out)); err != nil {
				t.Fatal(err)
			}
		}
	}
	code, out, diagnostic := runCLI(t, nil, false, "--format", "{mem:percent}", "--debug")
	if code != 0 || out == "" || !strings.Contains(diagnostic, "smol: mem ") || !strings.Contains(diagnostic, "smol: total ") {
		t.Fatal(code, out, diagnostic)
	}
	code, _, diagnostic = runCLI(t, nil, false, "--animate", "--debug")
	if code != 2 || !strings.Contains(diagnostic, "mutually exclusive") {
		t.Fatal(code, diagnostic)
	}
	p := writeConfig(t, `format="{host}"`)
	code, out, diagnostic = runCLI(t, nil, false, "--config", p, "--list-config-paths")
	if code != 0 || out != p+" (found)\n" || diagnostic != "" {
		t.Fatal(code, out, diagnostic)
	}
	code, _, diagnostic = runCLI(t, nil, false, "--config", p, "--check-config", "--debug")
	if code != 0 || !strings.Contains(diagnostic, p) {
		t.Fatal(code, diagnostic)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "smol")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`format="{host}"`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic = runCLI(t, map[string]string{"HOME": t.TempDir(), "XDG_CONFIG_DIRS": root}, false)
	if code != 0 || out != "thinkpad\n" || diagnostic != "" {
		t.Fatal(code, out, diagnostic)
	}
}
func TestSystemHooks(t *testing.T) {
	r := System(Runtime{})
	if r.Now().IsZero() {
		t.Fatal("zero time")
	}
	total, free, err := r.Statfs("/")
	if err != nil || total < free || total == 0 {
		t.Fatal(total, free, err)
	}
	if _, _, err := r.Statfs(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing mount error")
	}
	arch, err := r.Uname()
	if err != nil || arch == "" {
		t.Fatal(arch, err)
	}
	if _, err := r.Interfaces(); err != nil {
		t.Fatal(err)
	}
	var out, diagnostic bytes.Buffer
	r.FS = os.DirFS("../../testdata/x86")
	r.Getenv = func(string) string { return "" }
	code := Run(context.Background(), []string{"--format", "{time}", "--debug"}, &out, &diagnostic, r)
	if code != 0 || out.Len() == 0 {
		t.Fatal(code, out.String(), diagnostic.String())
	}
}

func TestDefaultOutputGolden(t *testing.T) {
	code, out, diagnostic := runCLI(t, map[string]string{"HOME": t.TempDir()}, false)
	const want = "Ubuntu 24.04.1 LTS · 6.8.0-48-generic · 3d 4h 4m · 4.6 GiB / 15.6 GiB\n"
	if code != 0 || out != want || diagnostic != "" {
		t.Fatalf("%d %q %q", code, out, diagnostic)
	}
}
