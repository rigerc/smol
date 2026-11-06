package modules

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rigerc/smol/internal/displayfmt"
)

type uptimeModule struct{}

func (uptimeModule) Fetch(_ context.Context, env Env) (Result, error) {
	s, err := readTrim(env.FS, "proc/uptime")
	if err != nil {
		return Result{}, err
	}
	first, _, _ := strings.Cut(s, " ")
	secs, err := strconv.ParseFloat(first, 64)
	if err != nil {
		return Result{}, fmt.Errorf("parse /proc/uptime: %w", err)
	}
	if math.IsNaN(secs) || math.IsInf(secs, 0) || secs < 0 || secs >= float64(math.MaxInt64) {
		return Result{}, errors.New("uptime: invalid seconds")
	}
	return coreResult("uptime", formatUptime(int64(secs)), []byte(s), env), nil
}

// formatUptime preserves the original duration formatting.
func formatUptime(secs int64) string { return displayfmt.Duration(secs, displayfmt.Default().Duration) }
