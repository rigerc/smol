// Command smol prints one line of Linux system information.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rigerc/smol/internal/app"
	"github.com/rigerc/smol/internal/playback"
)

// version can be set with -ldflags '-X main.version=...'.
var version = "dev"

type interruption struct{ signal syscall.Signal }

func (i interruption) Error() string { return fmt.Sprintf("interrupted by %s", i.signal) }

func main() { os.Exit(run()) }

func run() int {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		select {
		case sig := <-signals:
			if sig == syscall.SIGTERM {
				cancel(interruption{syscall.SIGTERM})
			} else {
				cancel(interruption{syscall.SIGINT})
			}
		case <-ctx.Done():
		}
	}()
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	code := app.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, system(app.Runtime{
		FS: os.DirFS("/"), Getenv: os.Getenv, StdoutIsTTY: terminalColumns(os.Stdout) > 0, Version: version,
		Playback: playback.Options{Columns: func() int { return terminalColumns(os.Stdout) }, Resize: resized},
	}))
	var stopped interruption
	if errors.As(context.Cause(ctx), &stopped) {
		return 128 + int(stopped.signal)
	}
	return code
}
