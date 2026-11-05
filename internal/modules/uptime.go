package modules

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type uptimeModule struct{}

func (uptimeModule) Fetch(_ context.Context, env Env) (string, error) {
	s, err := readTrim(env.FS, "proc/uptime")
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(s, " ")
	secs, err := strconv.ParseFloat(first, 64)
	if err != nil {
		return "", fmt.Errorf("parse /proc/uptime: %w", err)
	}
	return formatUptime(int64(secs)), nil
}

// formatUptime renders seconds as "3d 4h 12m", dropping leading zero units.
func formatUptime(secs int64) string {
	if secs < 60 {
		return "<1m"
	}
	d := secs / 86400
	h := (secs % 86400) / 3600
	m := (secs % 3600) / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
