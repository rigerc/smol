// Command smol prints one line of Linux system information.
package main

import (
	"context"
	"github.com/rigerc/smol/internal/app"
	"os"
)

// version can be set with -ldflags '-X main.version=...'.
var version = "dev"

func main() {
	os.Exit(app.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, app.Runtime{
		FS: os.DirFS("/"), Getenv: os.Getenv, StdoutIsTTY: isTerminal(os.Stdout), Version: version,
	}))
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
