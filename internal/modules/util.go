package modules

import (
	"github.com/rigerc/smol/internal/format"
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

// formatKiB preserves the original default size formatting.
func formatKiB(kib uint64) string { return format.Size(kib*1024, format.Default().Size) }
