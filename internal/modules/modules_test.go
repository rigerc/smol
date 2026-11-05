package modules

import (
	"context"
	"errors"
	"os"
	"testing"
	"testing/fstest"
)

func envFor(t *testing.T, fixture string, vars map[string]string) Env {
	t.Helper()
	return Env{
		FS:     os.DirFS("../../testdata/" + fixture),
		Getenv: func(k string) string { return vars[k] },
	}
}

func TestFetch(t *testing.T) {
	tests := []struct {
		name    string
		module  string
		fixture string
		vars    map[string]string
		want    string
	}{
		{"os pretty name", "os", "x86", nil, "Ubuntu 24.04.1 LTS"},
		{"os fallback to usr/lib and NAME", "os", "arm", nil, "Alpine Linux"},
		{"kernel", "kernel", "x86", nil, "6.8.0-48-generic"},
		{"host", "host", "x86", nil, "thinkpad"},
		{"uptime days", "uptime", "x86", nil, "3d 4h 4m"},
		{"uptime under a minute", "uptime", "arm", nil, "1m"},
		{"load", "load", "x86", nil, "0.52 0.48 0.45"},
		{"mem with MemAvailable", "mem", "x86", nil, "4.6 GiB / 15.6 GiB"},
		{"mem without MemAvailable", "mem", "arm", nil, "438 MiB / 926 MiB"},
		{"cpu x86", "cpu", "x86", nil, "Intel Core i7-1165G7 @ 2.80GHz (4)"},
		{"cpu arm prefers Hardware over Model", "cpu", "arm", nil, "BCM2835 (2)"},
		{"shell", "shell", "x86", map[string]string{"SHELL": "/usr/bin/fish"}, "fish"},
		{"user from USER", "user", "x86", map[string]string{"USER": "ada"}, "ada"},
		{"user from LOGNAME", "user", "x86", map[string]string{"LOGNAME": "grace"}, "grace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, ok := Get(tt.module)
			if !ok {
				t.Fatalf("module %q not registered", tt.module)
			}
			got, err := m.Fetch(context.Background(), envFor(t, tt.fixture, tt.vars))
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUnavailable(t *testing.T) {
	for _, name := range []string{"shell", "user"} {
		m, _ := Get(name)
		_, err := m.Fetch(context.Background(), envFor(t, "x86", nil))
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: got err %v, want ErrUnavailable", name, err)
		}
	}
}

func TestMissingFilesError(t *testing.T) {
	empty := Env{FS: fstest.MapFS{}, Getenv: func(string) string { return "" }}
	for _, name := range []string{"os", "kernel", "host", "uptime", "mem", "cpu", "load"} {
		m, _ := Get(name)
		if _, err := m.Fetch(context.Background(), empty); err == nil {
			t.Errorf("%s: expected error on empty filesystem", name)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := map[int64]string{
		0: "<1m", 59: "<1m", 60: "1m", 3599: "59m",
		3600: "1h 0m", 86399: "23h 59m", 86400: "1d 0h 0m", 273845: "3d 4h 4m",
	}
	for in, want := range tests {
		if got := formatUptime(in); got != want {
			t.Errorf("formatUptime(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatKiB(t *testing.T) {
	tests := map[uint64]string{
		512: "512 KiB", 1024: "1 MiB", 1048576: "1.0 GiB", 16318712: "15.6 GiB",
	}
	for in, want := range tests {
		if got := formatKiB(in); got != want {
			t.Errorf("formatKiB(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestParseOSReleaseQuoting(t *testing.T) {
	kv := parseOSRelease("A=plain\nB=\"dq \\\"x\\\"\"\nC='sq'\n# comment\n\nbroken line\n")
	want := map[string]string{"A": "plain", "B": `dq "x"`, "C": "sq"}
	for k, v := range want {
		if kv[k] != v {
			t.Errorf("%s = %q, want %q", k, kv[k], v)
		}
	}
}

func TestNamesSortedAndComplete(t *testing.T) {
	want := []string{"cpu", "host", "kernel", "load", "mem", "os", "shell", "uptime", "user"}
	got := Names()
	if len(got) != len(want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Names() = %v, want %v", got, want)
		}
	}
}

func TestParseCPUInfoFallbackOrder(t *testing.T) {
	boardFirst := []byte("Model\t: Some Board\nHardware\t: SoC-1\nprocessor\t: 0\n")
	if got, n := parseCPUInfo(boardFirst); got != "SoC-1" || n != 1 {
		t.Errorf("board first: got (%q, %d), want (SoC-1, 1)", got, n)
	}
	boardOnly := []byte("Model\t: Some Board\nprocessor\t: 0\nprocessor\t: 1\n")
	if got, n := parseCPUInfo(boardOnly); got != "Some Board" || n != 2 {
		t.Errorf("board only: got (%q, %d), want (Some Board, 2)", got, n)
	}
}
