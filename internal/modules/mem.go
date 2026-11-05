package modules

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"math"
	"strconv"
	"strings"
)

type memModule struct{}

func (memModule) Fetch(_ context.Context, env Env) (Result, error) {
	data, err := fs.ReadFile(env.FS, "proc/meminfo")
	if err != nil {
		return Result{}, err
	}
	info := parseMeminfo(data)
	total, ok := info["MemTotal"]
	if !ok || total == 0 || total > math.MaxUint64/1024 {
		return Result{}, errors.New("meminfo: MemTotal missing")
	}
	return memoryResult(info, env), nil
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
