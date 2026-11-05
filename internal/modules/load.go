package modules

import (
	"context"
	"errors"
	"strings"
)

type loadModule struct{}

func (loadModule) Fetch(_ context.Context, env Env) (string, error) {
	s, err := readTrim(env.FS, "proc/loadavg")
	if err != nil {
		return "", err
	}
	f := strings.Fields(s)
	if len(f) < 3 {
		return "", errors.New("loadavg: expected at least 3 fields")
	}
	return strings.Join(f[:3], " "), nil
}
