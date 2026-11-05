package modules

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
)

type loadModule struct{}

func (loadModule) Fetch(_ context.Context, env Env) (Result, error) {
	s, err := readTrim(env.FS, "proc/loadavg")
	if err != nil {
		return Result{}, err
	}
	f := strings.Fields(s)
	if len(f) < 3 {
		return Result{}, errors.New("loadavg: expected at least 3 fields")
	}
	for _, value := range f[:3] {
		n, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return Result{}, errors.New("loadavg: invalid load")
		}
	}
	return coreResult("load", strings.Join(f[:3], " "), nil, env), nil
}
