package modules

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/rigerc/smol/internal/format"
)

func extraSpecs() []Module {
	specs := []struct{ name, desc, fields string }{
		{"swap", "Swap usage", "used free total percent bar"}, {"disk", "Filesystem usage", "used free available total percent bar"}, {"battery", "Battery charge", "percent status"}, {"terminal", "Terminal emulator", ""}, {"desktop", "Desktop session", "wm session"}, {"virt", "Virtualization environment", ""}, {"time", "Local time", ""}, {"processes", "Process count", ""}, {"init", "Init process", ""}, {"machine", "Machine model", "vendor board bios"}, {"localip", "Local IP address", "ipv6 interface"}, {"packages", "Installed package count", ""},
	}
	out := make([]Module, 0, len(specs))
	for _, s := range specs {
		m := Module{Name: s.name, Description: s.desc}
		for _, f := range strings.Fields(s.fields) {
			m.Fields = append(m.Fields, Field{f, fieldDescription(s.name, f)})
		}
		m.Fetch = func(ctx context.Context, env Env) (Result, error) { return extraFetch(ctx, s.name, env) }
		out = append(out, m)
	}
	return out
}
func extraFetch(ctx context.Context, name string, env Env) (Result, error) {
	switch name {
	case "swap":
		data, err := fs.ReadFile(env.FS, "proc/meminfo")
		if err != nil {
			return Result{}, err
		}
		kv := parseMeminfo(data)
		total := kv["SwapTotal"]
		free, hasFree := kv["SwapFree"]
		if total == 0 || !hasFree || free > total {
			return Result{}, ErrUnavailable
		}

		return sizeResult((total-free)*1024, free*1024, total*1024, env), nil
	case "disk":
		if env.Statfs == nil {
			return Result{}, errors.New("disk: missing Statfs hook")
		}
		stats, err := env.Statfs(options(env).Disk.Mount)
		if err != nil {
			return Result{}, err
		}
		if stats.Free > stats.Total || stats.Available > stats.Free {
			return Result{}, errors.New("disk: inconsistent filesystem stats")
		}
		used := stats.Total - stats.Free
		r := sizeResult(used, stats.Free, stats.Total, env)
		r.Fields["available"] = format.Size(stats.Available, display(env).Size)
		denominator := used + stats.Available
		if denominator == 0 {
			delete(r.Fields, "percent")
			delete(r.Fields, "bar")
		} else {
			r.Fields["percent"], r.Fields["bar"] = format.Percent(float64(used)/float64(denominator)*100, display(env).Percent)
		}
		return r, nil
	case "battery":
		return battery(env)
	case "terminal":
		return textResult(terminal(env))
	case "desktop":
		return desktop(env)
	case "virt":
		return textResult(virtualization(env))
	case "time":
		if env.Now == nil {
			return Result{}, errors.New("time: missing Now hook")
		}
		layout := options(env).Time.Format

		return textResult(env.Now().Format(layout))
	case "processes":
		entries, err := fs.ReadDir(env.FS, "proc")
		if err != nil {
			return Result{}, err
		}
		count := 0
		for _, e := range entries {
			if _, err := strconv.ParseUint(e.Name(), 10, 64); err == nil && e.IsDir() {
				count++
			}
		}
		return textResult(strconv.Itoa(count))
	case "init":
		s, err := readTrim(env.FS, "proc/1/comm")
		return Result{Text: s}, err
	case "machine":
		return machine(env)
	case "localip":
		return localIP(env)
	case "packages":
		return packages(ctx, env)
	}
	return Result{}, ErrUnavailable
}
func textResult(text string) (Result, error) {
	if text == "" {
		return Result{}, ErrUnavailable
	}
	return Result{Text: text}, nil
}
func battery(env Env) (Result, error) {
	paths := []string{}
	if options(env).Battery.Device != "" {
		paths = append(paths, "sys/class/power_supply/"+options(env).Battery.Device)
	} else {
		paths, _ = fs.Glob(env.FS, "sys/class/power_supply/*")
	}
	for _, p := range paths {
		kind, _ := readTrim(env.FS, p+"/type")
		if kind != "Battery" {
			continue
		}
		s, err := readTrim(env.FS, p+"/capacity")
		if err != nil {
			return Result{}, err
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return Result{}, ErrUnavailable
		}
		pct, _ := format.Percent(v, display(env).Percent)
		status, _ := readTrim(env.FS, p+"/status")
		status = strings.ToLower(status)
		switch status {
		case "charging", "discharging", "full":
		default:
			status = "unknown"
		}
		return Result{Text: pct, Fields: map[string]string{"percent": pct, "status": status}}, nil
	}
	return Result{}, ErrUnavailable
}
func terminal(env Env) string {
	for _, kv := range [][2]string{{"TERM_PROGRAM", ""}, {"KITTY_PID", "kitty"}, {"WEZTERM_EXECUTABLE", "wezterm"}, {"WT_SESSION", "Windows Terminal"}, {"SSH_TTY", "ssh"}, {"SSH_CONNECTION", "ssh"}, {"TERM", ""}} {
		if v := env.Getenv(kv[0]); v != "" {
			if kv[1] != "" {
				return kv[1]
			}
			return v
		}
	}
	return ""
}
func desktop(env Env) (Result, error) {
	name := env.Getenv("XDG_CURRENT_DESKTOP")
	if name == "" {
		name = env.Getenv("DESKTOP_SESSION")
	}
	wm := ""
	if env.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		wm = "Hyprland"
	}
	if env.Getenv("SWAYSOCK") != "" {
		if wm != "" {
			return Result{}, ErrUnavailable
		}
		wm = "sway"
	}
	if name == "" {
		name = wm
	}
	if name == "" {
		return Result{}, ErrUnavailable
	}
	session := ""
	if env.Getenv("WAYLAND_DISPLAY") != "" {
		session = "wayland"
	}
	return Result{Text: name, Fields: map[string]string{"wm": wm, "session": session}}, nil
}
func virtualization(env Env) string {
	if env.Getenv("WSL_DISTRO_NAME") != "" {
		return "wsl"
	}
	for _, kv := range [][2]string{{".dockerenv", "docker"}, {"run/.containerenv", "podman"}} {
		if _, err := fs.Stat(env.FS, kv[0]); err == nil {
			return kv[1]
		}
	}
	cgroup, _ := readTrim(env.FS, "proc/1/cgroup")
	for _, v := range []string{"docker", "podman", "lxc"} {
		if strings.Contains(cgroup, v) {
			return v
		}
	}
	product, _ := readTrim(env.FS, "sys/class/dmi/id/product_name")
	lower := strings.ToLower(product)
	for _, kv := range [][2]string{{"kvm", "kvm"}, {"virtualbox", "virtualbox"}, {"vmware", "vmware"}, {"virtual machine", "hyperv"}, {"qemu", "qemu"}} {
		if strings.Contains(lower, kv[0]) {
			return kv[1]
		}
	}
	cpu, _ := readTrim(env.FS, "proc/cpuinfo")
	for _, word := range strings.Fields(cpu) {
		if word == "hypervisor" {
			return "hypervisor"
		}
	}
	return ""
}
func machineValue(env Env, p string) string {
	v, _ := readTrim(env.FS, p)
	v = strings.TrimSpace(strings.Trim(v, "\x00"))
	switch strings.ToLower(v) {
	case "to be filled by o.e.m.", "to be filled by oem", "default string", "system product name", "system manufacturer", "system version", "none", "unknown", "n/a", "not applicable", "not specified", "invalid", "o.e.m.":
		return ""
	}
	return v
}
func machine(env Env) (Result, error) {
	name := machineValue(env, "sys/class/dmi/id/product_name")
	if name == "" {
		name = machineValue(env, "sys/firmware/devicetree/base/model")
	}
	if name == "" {
		return Result{}, ErrUnavailable
	}
	return Result{Text: name, Fields: map[string]string{"vendor": machineValue(env, "sys/class/dmi/id/sys_vendor"), "board": machineValue(env, "sys/class/dmi/id/board_name"), "bios": machineValue(env, "sys/class/dmi/id/bios_version")}}, nil
}
func localIP(env Env) (Result, error) {
	if env.Interfaces == nil {
		return Result{}, errors.New("localip: missing Interfaces hook")
	}
	entries, err := env.Interfaces()
	if err != nil {
		return Result{}, err
	}
	for _, e := range entries {
		if e.Name == "lo" || options(env).LocalIP.Interface != "" && options(env).LocalIP.Interface != e.Name {
			continue
		}
		ip := ""
		if len(e.IPv4) > 0 {
			ip = e.IPv4[0]
		}
		v6 := ""
		if len(e.IPv6) > 0 {
			v6 = e.IPv6[0]
		}
		if options(env).LocalIP.Family == "ipv6" {
			ip = v6
		}
		if ip != "" {
			return Result{Text: ip, Fields: map[string]string{"ipv6": v6, "interface": e.Name}}, nil
		}
	}
	return Result{}, ErrUnavailable
}
func packages(ctx context.Context, env Env) (Result, error) {
	managers := []string{options(env).Packages.Manager}
	if managers[0] == "auto" {
		managers = []string{"dpkg", "pacman", "apk"}
	}
	for _, manager := range managers {
		p := ""
		switch manager {
		case "dpkg":
			p = "var/lib/dpkg/status"
		case "pacman":
			p = "var/lib/pacman/local"
		case "apk":
			p = "lib/apk/db/installed"
		default:
			return Result{}, fmt.Errorf("unsupported package manager %q", manager)
		}
		if _, err := fs.Stat(env.FS, p); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return Result{}, err
		}
		count := 0
		if manager == "pacman" {
			entries, err := fs.ReadDir(env.FS, p)
			if err != nil {
				return Result{}, err
			}
			for _, e := range entries {
				if err := ctx.Err(); err != nil {
					return Result{}, err
				}
				if e.IsDir() && e.Name() != "ALPM_DB_VERSION" {
					count++
				}
			}
		} else {
			f, err := env.FS.Open(p)
			if err != nil {
				return Result{}, err
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if err := ctx.Err(); err != nil {
					_ = f.Close()
					return Result{}, err
				}
				line := scanner.Text()
				if manager == "dpkg" && line == "Status: install ok installed" || manager == "apk" && strings.HasPrefix(line, "P:") {
					count++
				}
			}
			scanErr := scanner.Err()
			closeErr := f.Close()
			if err := errors.Join(scanErr, closeErr); err != nil {
				return Result{}, err
			}
		}
		return textResult(strconv.Itoa(count))
	}
	return Result{}, ErrUnavailable
}

// ValidDevice restricts selectors to a single sysfs entry.
func ValidDevice(device string) bool {
	return device == "" || device != "." && device != ".." && path.Base(device) == device && !strings.ContainsAny(device, "/\\\x00")
}
