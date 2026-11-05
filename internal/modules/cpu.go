package modules

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

type cpuModule struct{}

func (cpuModule) Fetch(_ context.Context, env Env) (string, error) {
	data, err := fs.ReadFile(env.FS, "proc/cpuinfo")
	if err != nil {
		return "", err
	}
	model, threads := parseCPUInfo(data)
	if model == "" {
		return "", errors.New("cpuinfo: no model name")
	}
	if threads > 0 {
		return fmt.Sprintf("%s (%d)", model, threads), nil
	}
	return model, nil
}

// parseCPUInfo returns the CPU model name and the number of logical CPUs.
// x86 uses "model name". Many ARM kernels have no such field, so we fall back
// to "Hardware" (the SoC, e.g. BCM2835) and then "Model" (the board). The SoC
// is preferred because it is the closest thing to a CPU name, and the choice
// must not depend on the order of lines in the file.
func parseCPUInfo(data []byte) (model string, threads int) {
	var hardware, board string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch k {
		case "processor":
			threads++
		case "model name":
			if model == "" {
				model = v
			}
		case "Hardware":
			if hardware == "" {
				hardware = v
			}
		case "Model":
			if board == "" {
				board = v
			}
		}
	}
	if model == "" {
		model = hardware
	}
	if model == "" {
		model = board
	}
	return cleanCPUName(model), threads
}

func cleanCPUName(s string) string {
	s = strings.NewReplacer("(R)", "", "(TM)", "", "(tm)", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}
