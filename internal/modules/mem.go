package modules

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strconv"
	"strings"
)

type memModule struct{}

func (memModule) Fetch(_ context.Context, env Env) (string, error) {
	data, err := fs.ReadFile(env.FS, "proc/meminfo")
	if err != nil {
		return "", err
	}
	info := parseMeminfo(data)
	total, ok := info["MemTotal"]
	if !ok || total == 0 {
		return "", errors.New("meminfo: MemTotal missing")
	}
	avail, ok := info["MemAvailable"]
	if !ok {
		// Kernels before 3.14 have no MemAvailable.
		avail = info["MemFree"] + info["Buffers"] + info["Cached"]
	}
	if avail > total {
		avail = total
	}
	return formatKiB(total-avail) + " / " + formatKiB(total), nil
}

// parseMeminfo returns /proc/meminfo values in KiB, keyed by field name.
func parseMeminfo(data []byte) map[string]uint64 {
	out := map[string]uint64{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		k, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		out[k] = n
	}
	return out
}
