package modules

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func testEnv(files map[string]string, vars map[string]string) Env {
	tree := fstest.MapFS{}
	for p, v := range files {
		tree[p] = &fstest.MapFile{Data: []byte(v)}
	}
	defaults := DefaultOptions()
	return Env{FS: tree, Options: &defaults, Getenv: func(k string) string { return vars[k] }}
}
func TestExpandedModules(t *testing.T) {
	cases := []struct {
		name        string
		files, vars map[string]string
		want        string
		fields      map[string]string
	}{
		{"swap", map[string]string{"proc/meminfo": "SwapTotal: 2048 kB\nSwapFree: 1024 kB"}, nil, "1 MiB / 2 MiB", map[string]string{"used": "1 MiB", "free": "1 MiB", "total": "2 MiB", "percent": "50%", "bar": "[#####-----]"}},
		{"battery", map[string]string{"sys/class/power_supply/BAT0/type": "Battery", "sys/class/power_supply/BAT0/capacity": "82", "sys/class/power_supply/BAT0/status": "Discharging"}, nil, "82%", map[string]string{"percent": "82%", "status": "discharging"}},
		{"terminal", nil, map[string]string{"KITTY_PID": "1"}, "kitty", nil},
		{"desktop", nil, map[string]string{"XDG_CURRENT_DESKTOP": "GNOME", "WAYLAND_DISPLAY": "wayland-0", "SWAYSOCK": "s"}, "GNOME", map[string]string{"wm": "sway", "session": "wayland"}},
		{"virt", map[string]string{".dockerenv": ""}, nil, "docker", nil},
		{"processes", map[string]string{"proc/1/comm": "systemd", "proc/22/comm": "bash", "proc/notpid": "x"}, nil, "2", nil},
		{"init", map[string]string{"proc/1/comm": "systemd"}, nil, "systemd", nil},
		{"machine", map[string]string{"sys/class/dmi/id/product_name": "ThinkPad\x00", "sys/class/dmi/id/sys_vendor": "Lenovo", "sys/class/dmi/id/board_name": "board", "sys/class/dmi/id/bios_version": "bios"}, nil, "ThinkPad", map[string]string{"vendor": "Lenovo", "board": "board", "bios": "bios"}},
		{"packages", map[string]string{"var/lib/dpkg/status": "Package: a\nStatus: install ok installed\n\nPackage: b\nStatus: deinstall ok config-files"}, nil, "1", nil},
		{"os", map[string]string{"etc/lsb-release": "DISTRIB_DESCRIPTION='Legacy Linux'\nDISTRIB_ID=legacy\nDISTRIB_RELEASE=1"}, nil, "Legacy Linux", map[string]string{"id": "legacy", "version": "1"}},
		{"os", map[string]string{"etc/lsb-release": "DISTRIB_ID=Legacy\nDISTRIB_RELEASE=1"}, nil, "Legacy 1", nil},
		{"shell", map[string]string{"proc/self/stat": "1 (some name) S 100 0", "proc/100/comm": "fish"}, map[string]string{"SHELL": "/bin/bash"}, "fish", nil},
		{"mem", map[string]string{"proc/meminfo": "MemTotal: 1024 kB\nMemFree: 256 kB\nCached: 128 kB\nBuffers: 128 kB\nSReclaimable: 128 kB\nShmem: 128 kB"}, nil, "512 KiB / 1 MiB", map[string]string{"used": "512 KiB", "free": "256 KiB", "available": "512 KiB", "total": "1 MiB", "percent": "50%", "bar": "[#####-----]"}},
		{"uptime", map[string]string{"proc/uptime": "90061 0"}, nil, "1d 1h 1m", map[string]string{"days": "1", "hours": "1", "minutes": "1", "seconds": "1"}},
		{"kernel", map[string]string{"proc/sys/kernel/osrelease": "6.8", "proc/sys/kernel/ostype": "Linux"}, nil, "6.8", map[string]string{"release": "6.8", "name": "Linux"}},
		{"cpu", map[string]string{"proc/cpuinfo": "processor: 0\nmodel name: CPU\ncpu cores: 1\ncpu MHz: 2000", "sys/class/hwmon/hwmon0/name": "coretemp", "sys/class/hwmon/hwmon0/temp1_input": "42000"}, nil, "CPU (1)", map[string]string{"threads": "1", "cores": "1", "freq": "2.00 GHz", "temp": "42°C"}},
		{"load", map[string]string{"proc/loadavg": "0.5 0.4 0.3", "proc/cpuinfo": "processor: 0"}, nil, "0.5 0.4 0.3", map[string]string{"percent": "50%"}},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.want, func(t *testing.T) {
			m, _ := Get(tc.name)
			r, err := m.Fetch(context.Background(), testEnv(tc.files, tc.vars))
			if err != nil || r.Text != tc.want {
				t.Fatalf("%+v %v want %s", r, err, tc.want)
			}
			for k, want := range tc.fields {
				if r.Fields[k] != want {
					t.Errorf("%s=%q want %q", k, r.Fields[k], want)
				}
			}
		})
	}
}
func TestModuleHooks(t *testing.T) {
	env := testEnv(nil, nil)
	env.Now = func() time.Time { return time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC) }
	env.Statfs = func(p string) (DiskStats, error) {
		if p != "/" {
			t.Error(p)
		}
		return DiskStats{2048 * 1024, 1024 * 1024, 1024 * 1024}, nil
	}
	env.Interfaces = func() ([]Interface, error) {
		return []Interface{{Name: "lo", IPv4: []string{"127.0.0.1"}}, {Name: "eth0", IPv4: []string{"10.0.0.1"}, IPv6: []string{"::1"}}}, nil
	}
	for name, want := range map[string]string{"disk": "1 MiB / 2 MiB", "time": "15:04", "localip": "10.0.0.1"} {
		m, _ := Get(name)
		r, err := m.Fetch(context.Background(), env)
		if err != nil || r.Text != want {
			t.Fatal(name, r, err)
		}
	}
	env.Options.LocalIP.Family = "ipv6"
	m, _ := Get("localip")
	r, err := m.Fetch(context.Background(), env)
	if err != nil || r.Text != "::1" || r.Fields["interface"] != "eth0" {
		t.Fatal(r, err)
	}
	env = envFor(t, "x86", nil)
	env.Uname = func() (string, error) { return "x86_64", nil }
	m, _ = Get("kernel")
	r, err = m.Fetch(context.Background(), env)
	if err != nil || r.Fields["arch"] != "x86_64" {
		t.Fatal(r, err)
	}
}
func TestExpandedUnavailable(t *testing.T) {
	for _, name := range []string{"swap", "disk", "battery", "terminal", "desktop", "virt", "time", "processes", "init", "machine", "localip", "packages"} {
		m, _ := Get(name)
		if r, err := m.Fetch(context.Background(), testEnv(nil, nil)); err == nil {
			t.Errorf("%s: %+v", name, r)
		}
	}
}
func TestDetectionVariants(t *testing.T) {
	for key, want := range map[string]string{"TERM_PROGRAM": "custom", "WEZTERM_EXECUTABLE": "wezterm", "WT_SESSION": "Windows Terminal", "SSH_TTY": "ssh", "SSH_CONNECTION": "ssh", "TERM": "custom"} {
		if got := terminal(testEnv(nil, map[string]string{key: "custom"})); got != want {
			t.Fatal(key, got)
		}
	}
	for _, tc := range []struct {
		files, vars map[string]string
		want        string
	}{{nil, map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, "wsl"}, {map[string]string{"run/.containerenv": ""}, nil, "podman"}, {map[string]string{"proc/1/cgroup": "docker"}, nil, "docker"}, {map[string]string{"sys/class/dmi/id/product_name": "KVM"}, nil, "kvm"}, {map[string]string{"proc/cpuinfo": "flags: hypervisor"}, nil, "hypervisor"}} {
		if got := virtualization(testEnv(tc.files, tc.vars)); got != tc.want {
			t.Fatal(got)
		}
	}
	for _, name := range []string{"Default string", "Unknown", "N/A", "None", "To Be Filled By O.E.M."} {
		if got := machineValue(testEnv(map[string]string{"model": name}, nil), "model"); got != "" {
			t.Fatal(got)
		}
	}
	if _, err := desktop(testEnv(nil, map[string]string{"HYPRLAND_INSTANCE_SIGNATURE": "1", "SWAYSOCK": "1"})); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	r, err := desktop(testEnv(nil, map[string]string{"HYPRLAND_INSTANCE_SIGNATURE": "1"}))
	if err != nil || r.Text != "Hyprland" {
		t.Fatal(r, err)
	}
	for _, device := range []string{"..", "../BAT0", "/BAT0", "a/b"} {
		if ValidDevice(device) {
			t.Fatal(device)
		}
	}
}
func TestPackageDatabases(t *testing.T) {
	for _, tc := range []struct {
		manager string
		files   map[string]string
		want    string
	}{{"apk", map[string]string{"lib/apk/db/installed": "P:a\nV:1\n\nP:b\n"}, "2"}, {"pacman", map[string]string{"var/lib/pacman/local/a/desc": "a", "var/lib/pacman/local/b/desc": "b"}, "2"}} {
		env := testEnv(tc.files, nil)
		env.Options.Packages.Manager = tc.manager
		r, err := packages(context.Background(), env)
		if err != nil || r.Text != tc.want {
			t.Fatal(r, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := packages(ctx, env); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	env := testEnv(map[string]string{"var/lib/dpkg/status": strings.Repeat("x", 100000)}, nil)
	if _, err := packages(context.Background(), env); err == nil {
		t.Fatal("expected scanner error")
	}
	env.Options.Packages.Manager = "rpm"
	if _, err := packages(context.Background(), env); err == nil {
		t.Fatal("accepted rpm")
	}
}

func TestLegacyMemoryAndMalformedValues(t *testing.T) {
	m, _ := Get("mem")
	r, err := m.Fetch(context.Background(), envFor(t, "legacy", nil))
	if err != nil || r.Text != "512 KiB / 1 MiB" {
		t.Fatal(r, err)
	}
	for avail, want := range map[string]string{"0": "1 MiB / 1 MiB", "1024": "0 KiB / 1 MiB", "2048": "512 KiB / 1 MiB", "bad": "512 KiB / 1 MiB"} {
		env := testEnv(map[string]string{"proc/meminfo": "MemTotal: 1024 kB\nMemAvailable: " + avail + " kB\nMemFree: 512 kB"}, nil)
		r, err := m.Fetch(context.Background(), env)
		if err != nil || r.Text != want {
			t.Fatal(r, err, want)
		}
	}
	for _, tc := range []struct{ name, path, value string }{{"uptime", "proc/uptime", "NaN"}, {"uptime", "proc/uptime", "-1"}, {"load", "proc/loadavg", "NaN 0 0"}, {"swap", "proc/meminfo", "SwapTotal: 10 kB\nSwapFree: 20 kB"}, {"battery", "sys/class/power_supply/BAT0/capacity", "NaN"}} {
		env := testEnv(map[string]string{tc.path: tc.value, "sys/class/power_supply/BAT0/type": "Battery"}, nil)
		m, _ := Get(tc.name)
		if _, err := m.Fetch(context.Background(), env); err == nil {
			t.Fatal(tc)
		}
	}
}
func TestHookFailuresAndSelection(t *testing.T) {
	env := testEnv(nil, nil)
	failure := errors.New("hook failed")
	env.Statfs = func(string) (DiskStats, error) { return DiskStats{}, failure }
	env.Interfaces = func() ([]Interface, error) { return nil, failure }
	for _, name := range []string{"disk", "localip"} {
		m, _ := Get(name)
		if _, err := m.Fetch(context.Background(), env); !errors.Is(err, failure) {
			t.Fatal(name, err)
		}
	}
	env = testEnv(map[string]string{"sys/class/power_supply/BAT0/type": "Battery", "sys/class/power_supply/BAT0/capacity": "10", "sys/class/power_supply/BAT1/type": "Battery", "sys/class/power_supply/BAT1/capacity": "90"}, nil)
	env.Options.Battery.Device = "BAT1"
	r, err := battery(env)
	if err != nil || r.Text != "90%" || r.Fields["status"] != "unknown" {
		t.Fatal(r, err)
	}
	env = envFor(t, "x86", nil)
	env.Uname = func() (string, error) { return "", failure }
	m, _ := Get("kernel")
	r, err = m.Fetch(context.Background(), env)
	if err != nil || r.Fields["arch"] == "" {
		t.Fatal(r, err)
	}
}
func TestCPUFrequencySysfs(t *testing.T) {
	env := testEnv(map[string]string{"proc/cpuinfo": "model name: CPU\nprocessor: 0\ncpu MHz: 1000", "sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq": "3000000", "sys/devices/system/cpu/cpu1/cpufreq/cpuinfo_max_freq": "4000000"}, nil)
	m, _ := Get("cpu")
	r, err := m.Fetch(context.Background(), env)
	if err != nil || r.Fields["freq"] != "4.00 GHz" {
		t.Fatal(r, err)
	}
}

func TestShellParentWrappers(t *testing.T) {
	for _, wrapper := range []string{"sudo", "su", "sh", "env", "time", "strace", "gdb"} {
		env := testEnv(map[string]string{"proc/self/stat": "1 (smol) R 100 0", "proc/100/comm": wrapper, "proc/100/stat": "100 (" + wrapper + ") R 200 0", "proc/200/comm": "zsh"}, map[string]string{"SHELL": "/bin/bash"})
		r, err := (shellModule{}).Fetch(context.Background(), env)
		if err != nil || r.Text != "zsh" {
			t.Fatal(wrapper, r, err)
		}
	}
}

func TestDiskReservedBlocks(t *testing.T) {
	env := testEnv(nil, nil)
	env.Statfs = func(string) (DiskStats, error) { return DiskStats{100 * 1024, 20 * 1024, 15 * 1024}, nil }
	m, _ := Get("disk")
	r, err := m.Fetch(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"used": "80 KiB", "free": "20 KiB", "available": "15 KiB", "total": "100 KiB", "percent": "84%", "bar": "[########--]"} {
		if r.Fields[k] != want {
			t.Fatal(k, r.Fields[k], want)
		}
	}
	for _, stats := range []DiskStats{{}, {100, 100, 0}, {100, 101, 0}, {100, 20, 21}} {
		env.Statfs = func(string) (DiskStats, error) { return stats, nil }
		r, err := m.Fetch(context.Background(), env)
		if stats.Free > stats.Total || stats.Available > stats.Free {
			if err == nil {
				t.Fatal(stats)
			}
		} else if err != nil || r.Fields["percent"] != "" || r.Fields["total"] == "" {
			t.Fatal(r, err)
		}
	}
}
func TestMemoryFallbackArithmetic(t *testing.T) {
	for _, tc := range []struct {
		info      map[string]uint64
		available string
	}{{map[string]uint64{"MemTotal": 1024, "MemFree": 100, "Shmem": 200}, "0 KiB"}, {map[string]uint64{"MemTotal": 1024, "MemFree": ^uint64(0), "Buffers": 1, "Shmem": ^uint64(0)}, "1 KiB"}, {map[string]uint64{"MemTotal": 1024, "MemFree": ^uint64(0), "Buffers": ^uint64(0)}, "1 MiB"}} {
		r := memoryResult(tc.info, testEnv(nil, nil))
		if r.Fields["available"] != tc.available {
			t.Fatal(r, tc.available)
		}
	}
}
