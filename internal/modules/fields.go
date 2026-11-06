package modules

import (
	"io/fs"
	"math"
	"math/bits"
	"runtime"
	"strconv"
	"strings"

	"github.com/rigerc/smol/internal/displayfmt"
)

func sizeResult(used, free, total uint64, env Env) Result {
	o := display(env)
	f := map[string]string{"used": displayfmt.Size(used, o.Size), "free": displayfmt.Size(free, o.Size), "total": displayfmt.Size(total, o.Size)}
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	f["percent"], f["bar"] = displayfmt.Percent(pct, o.Percent)
	return Result{Text: f["used"] + " / " + f["total"], Fields: f}
}
func coreResult(name, text string, data []byte, env Env) Result {
	r := Result{Text: text, Fields: map[string]string{}}
	o := display(env)
	switch name {
	case "mem":
		r = memoryResult(parseMeminfo(data), env)
	case "os":
		{
			kv := parseOSRelease(string(data))
			r.Fields["id"] = kv["ID"]
			r.Fields["version"] = kv["VERSION_ID"]
			if kv["DISTRIB_ID"] != "" {
				r.Fields["id"] = kv["DISTRIB_ID"]
				r.Fields["version"] = kv["DISTRIB_RELEASE"]
			}
		}
	case "kernel":
		r.Fields["release"] = text
		r.Fields["name"], _ = readTrim(env.FS, "proc/sys/kernel/ostype")
		r.Fields["arch"] = runtime.GOARCH
		if env.Uname != nil {
			if arch, err := env.Uname(); err == nil {
				r.Fields["arch"] = arch
			}
		}
	case "uptime":
		s := string(data)
		v, _, _ := strings.Cut(s, " ")
		secs, _ := strconv.ParseFloat(v, 64)
		n := int64(secs)
		r.Text = displayfmt.Duration(n, o.Duration)
		for k, v := range map[string]int64{"days": n / 86400, "hours": n % 86400 / 3600, "minutes": n % 3600 / 60, "seconds": n % 60} {
			r.Fields[k] = strconv.FormatInt(v, 10)
		}
	case "load":
		data, _ := fs.ReadFile(env.FS, "proc/cpuinfo")
		_, n := parseCPUInfo(data)
		v, _ := strconv.ParseFloat(strings.Fields(text)[0], 64)
		if n > 0 {
			r.Fields["percent"], _ = displayfmt.Percent(v/float64(n)*100, o.Percent)
		}
	case "cpu":
		_, n := parseCPUInfo(data)
		r.Fields["threads"] = strconv.Itoa(n)
		kv := parseOSRelease(strings.ReplaceAll(string(data), ":", "="))
		r.Fields["cores"] = kv["cpu cores"]
		freq := 0.0
		paths, _ := fs.Glob(env.FS, "sys/devices/system/cpu/cpu[0-9]*/cpufreq/cpuinfo_max_freq")
		for _, p := range paths {
			s, _ := readTrim(env.FS, p)
			v, _ := strconv.ParseFloat(s, 64)
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				freq = math.Max(freq, v/1000)
			}
		}
		if freq == 0 {
			freq, _ = strconv.ParseFloat(kv["cpu MHz"], 64)
		}
		if freq > 0 && !math.IsInf(freq, 0) {
			r.Fields["freq"] = strconv.FormatFloat(freq/1000, 'f', 2, 64) + " GHz"
		}
		if temp, ok := cpuTemperature(env); ok {
			r.Fields["temp"] = displayfmt.Temperature(temp, o.Temp)
		}
	}
	return r
}
func cpuTemperature(env Env) (float64, bool) {
	paths, _ := fs.Glob(env.FS, "sys/class/hwmon/hwmon*/temp*_input")
	highest := math.Inf(-1)
	for _, p := range paths {
		dir := p[:strings.LastIndex(p, "/")]
		name, _ := readTrim(env.FS, dir+"/name")
		label, _ := readTrim(env.FS, strings.TrimSuffix(p, "_input")+"_label")
		marker := strings.ToLower(name + " " + label)
		if !strings.Contains(marker, "core") && !strings.Contains(marker, "cpu") && !strings.Contains(marker, "k10temp") && !strings.Contains(marker, "zenpower") && !strings.Contains(marker, "package") {
			continue
		}
		s, _ := readTrim(env.FS, p)
		v, err := strconv.ParseFloat(s, 64)
		if err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
			highest = math.Max(highest, v/1000)
		}
	}
	return highest, !math.IsInf(highest, -1)
}

func memoryResult(info map[string]uint64, env Env) Result {
	o := display(env)
	total := info["MemTotal"]
	avail, ok := info["MemAvailable"]
	if !ok || avail > total {
		low, high := uint64(0), uint64(0)
		for _, key := range []string{"MemFree", "Buffers", "Cached", "SReclaimable"} {
			sum, carry := bits.Add64(low, info[key], 0)
			low = sum
			high += carry
		}
		difference, borrow := bits.Sub64(low, info["Shmem"], 0)
		switch {
		case high < borrow:
			avail = 0
		case high > borrow:
			avail = total
		default:
			avail = min(difference, total)
		}
	}
	avail = min(avail, total)
	r := sizeResult((total-avail)*1024, min(info["MemFree"], total)*1024, total*1024, env)
	r.Fields["available"] = displayfmt.Size(avail*1024, o.Size)
	return r
}
