// Command smol prints one line of Linux system information.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/rigerc/smol/internal/app"
	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/persistent"
	"github.com/rigerc/smol/internal/playback"
	"github.com/rigerc/smol/internal/setup"
	"github.com/xo/terminfo"
	"golang.org/x/sys/unix"
)

// version can be set with -ldflags '-X main.version=...'.
var version = "dev"

type interruption struct{ signal syscall.Signal }

func (i interruption) Error() string { return fmt.Sprintf("interrupted by %s", i.signal) }

func (i interruption) ExitCode() int { return 128 + int(i.signal) }

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
	if len(os.Args) > 1 && os.Args[1] == "init" && slices.Contains(os.Args[2:], "--help") {
		return app.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, app.Runtime{})
	}
	var setupInputs setup.Runtime
	if len(os.Args) > 1 && os.Args[1] == "init" {
		var err error
		setupInputs, err = setupRuntime()
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "smol init:", err)
			return 1
		}
	}

	code := app.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, system(app.Runtime{
		Setup:   setupInputs,
		Entropy: rand.Reader,
		FS:      os.DirFS("/"), Getenv: os.Getenv, StdoutIsTTY: terminalColumns(os.Stdout) > 0, Version: version,
		Persistent: func(ctx context.Context) (persistent.Terminal, error) { return detectPersistent(ctx, os.Stdout) },
		Playback:   playback.Options{Columns: func() int { return terminalColumns(os.Stdout) }, Resize: resized},
	}))
	var stopped interruption
	if (len(os.Args) < 2 || os.Args[1] != "init") && errors.As(context.Cause(ctx), &stopped) {
		return 128 + int(stopped.signal)
	}
	return code
}

func isTerminal(file *os.File) bool {
	_, err := unix.IoctlGetTermios(terminalFD(file), unix.TCGETS)
	return err == nil
}

func terminalFD(file *os.File) int { return int(file.Fd()) } //nolint:gosec // G115: process terminal descriptors are kernel int file descriptors.

func cursorCapable(term string) bool {
	if term == "" || term == "dumb" {
		return false
	}
	info, err := terminfo.Load(term)
	return err == nil && len(info.Strings[terminfo.CursorUp]) > 0 && len(info.Strings[terminfo.CursorAddress]) > 0
}

func setupSeed(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return face.RandomSeed(rand.Reader)
}

func setupRuntime() (setup.Runtime, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return setup.Runtime{}, err
	}
	width, rows := 80, 0
	size, _ := unix.IoctlGetWinsize(terminalFD(os.Stderr), unix.TIOCGWINSZ)
	if size != nil {
		if size.Col > 0 {
			width = int(size.Col)
		}
		rows = int(size.Row)
	}
	return setup.Runtime{Input: os.Stdin, Getenv: os.Getenv, Entropy: setupSeed, CWD: cwd, StdinTTY: isTerminal(os.Stdin), StderrTTY: isTerminal(os.Stderr), Width: width, Rows: rows, Basic: !cursorCapable(os.Getenv("TERM"))}, nil
}
