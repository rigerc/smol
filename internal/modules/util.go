package modules

import (
	"fmt"
	"io/fs"
	"strings"
)

// readTrim reads a file and trims surrounding whitespace.
func readTrim(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// formatKiB renders a size given in KiB as a short human-readable string.
func formatKiB(kib uint64) string {
	const (
		mib = 1024
		gib = 1024 * 1024
	)
	switch {
	case kib >= gib:
		return fmt.Sprintf("%.1f GiB", float64(kib)/gib)
	case kib >= mib:
		return fmt.Sprintf("%.0f MiB", float64(kib)/mib)
	default:
		return fmt.Sprintf("%d KiB", kib)
	}
}
