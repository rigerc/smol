package app

import (
	"bytes"
	"context"
	"github.com/rigerc/smol/internal/modules"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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
func TestDefaultOutputGolden(t *testing.T) {
	code, out, diagnostic := runCLI(t, map[string]string{"HOME": t.TempDir()}, false)
	const want = "Ubuntu 24.04.1 LTS · 6.8.0-48-generic · 3d 4h 4m · 4.6 GiB / 15.6 GiB\n"
	if code != 0 || out != want || diagnostic != "" {
		t.Fatalf("%d %q %q", code, out, diagnostic)
	}
}

func TestModuleOptionsReachFetches(t *testing.T) {
	tree := fstest.MapFS{
		"sys/class/power_supply/BAT0/type": {Data: []byte("Battery")}, "sys/class/power_supply/BAT0/capacity": {Data: []byte("10")},
		"sys/class/power_supply/BAT1/type": {Data: []byte("Battery")}, "sys/class/power_supply/BAT1/capacity": {Data: []byte("90")},
		"var/lib/dpkg/status": {Data: []byte("Status: install ok installed\n")}, "lib/apk/db/installed": {Data: []byte("P:a\nP:b\n")},
	}
	custom := writeConfig(t, `format="{disk:percent} {time} {battery} {localip} {packages}"
[modules.disk]
mount="/custom"
[modules.time]
format="Mon 15:04:05"
[modules.battery]
device="BAT1"
[modules.localip]
interface="eth1"
family="ipv6"
[modules.packages]
manager="apk"
`)
	defaults := writeConfig(t, `format="{disk:percent} {time} {battery} {localip} {packages}"`)
	runtime := Runtime{FS: tree, Getenv: func(string) string { return "" }, Now: func() time.Time { return time.Date(2026, 11, 5, 15, 4, 5, 0, time.UTC) }, Interfaces: func() ([]modules.Interface, error) {
		return []modules.Interface{{Name: "eth0", IPv4: []string{"192.0.2.1"}}, {Name: "eth1", IPv6: []string{"2001:db8::1"}}}, nil
	}}
	for _, tc := range []struct{ path, mount, want string }{{custom, "/custom", "84% Thu 15:04:05 90% 2001:db8::1 2\n"}, {defaults, "/", "84% 15:04 10% 192.0.2.1 1\n"}} {
		runtime.Statfs = func(mount string) (modules.DiskStats, error) {
			if mount != tc.mount {
				t.Errorf("mount %s want %s", mount, tc.mount)
			}
			return modules.DiskStats{Total: 100, Free: 20, Available: 15}, nil
		}
		var out, diagnostic bytes.Buffer
		code := Run(context.Background(), []string{"--config", tc.path}, &out, &diagnostic, runtime)
		if code != 0 || out.String() != tc.want || diagnostic.Len() != 0 {
			t.Fatal(code, out.String(), diagnostic.String())
		}
	}
}

func TestRandomFaceInvocation(t *testing.T) {
	path := writeConfig(t, "format = \"{face}/{face}\"\n[face]\nrandom = true\n")
	runtime := Runtime{FS: os.DirFS("../../testdata/x86"), Getenv: func(string) string { return "" }, Entropy: strings.NewReader(strings.Repeat("a", 32))}
	var out, diag bytes.Buffer
	if code := Run(context.Background(), []string{"--config", path}, &out, &diag, runtime); code != 0 {
		t.Fatalf("%d %s", code, diag.String())
	}
	parts := strings.Split(strings.TrimSpace(out.String()), "/")
	if len(parts) != 2 || parts[0] == "" || parts[0] != parts[1] {
		t.Fatalf("different characters %q", out.String())
	}
	entropy, ok := runtime.Entropy.(*strings.Reader)
	if !ok || entropy.Len() != 16 {
		t.Fatalf("wrong entropy consumption: %v", ok)
	}
	for _, args := range [][]string{{"--check-config"}, {"--print-config"}, {"--format", "{host}"}, {"--face-preset", "kitty"}, {"--face-seed", "fixed"}} {
		runtime.Entropy = strings.NewReader("")
		out.Reset()
		diag.Reset()
		if code := Run(context.Background(), append([]string{"--config", path}, args...), &out, &diag, runtime); code != 0 {
			t.Fatalf("%v: %d %s", args, code, diag.String())
		}
	}
	out.Reset()
	diag.Reset()
	runtime.Entropy = strings.NewReader("")
	if code := Run(context.Background(), []string{"--config", path, "--format", "{face} · {host}", "--debug"}, &out, &diag, runtime); code != 0 || !strings.Contains(diag.String(), "face:") {
		t.Fatalf("%d %q %s", code, out.String(), diag.String())
	}
}
