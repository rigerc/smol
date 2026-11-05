// Package playback draws prepared terminal frames with checked writes and cleanup.
package playback

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Frame is a complete rendered line with its visible column width and dwell time.
type Frame struct {
	Text     string
	Width    int
	Duration time.Duration
}

// Options injects terminal capabilities and timers. Unknown width disables playback.
type Options struct {
	Columns  func() int
	Resize   <-chan os.Signal
	After    func(time.Duration) <-chan time.Time
	Duration time.Duration
}

// Run prints still when frames cannot fit; otherwise it loops until cancellation
// or duration expiry. All writes, including restoration, finish before it returns.
func Run(ctx context.Context, out io.Writer, frames []Frame, still string, opts Options) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	maxWidth := 0
	for _, frame := range frames {
		if frame.Duration <= 0 {
			return errors.New("animation frame duration must be positive")
		}
		maxWidth = max(maxWidth, frame.Width)
	}
	fits := func() bool { return opts.Columns != nil && opts.Columns() > maxWidth }
	if len(frames) == 0 || !fits() {
		return write(out, still+"\n")
	}
	if len(frames) == 1 {
		return write(out, frames[0].Text+"\n")
	}
	after := opts.After
	if after == nil {
		after = time.After
	}
	var finished <-chan time.Time
	if opts.Duration > 0 {
		finished = after(opts.Duration)
	}
	// Register cleanup before attempting to hide the cursor: a partial write may
	// have changed terminal state even when the writer returns an error.
	defer func() { err = errors.Join(err, write(out, "\x1b[?25h\n")) }()
	if err := write(out, "\x1b[?25l"); err != nil {
		return err
	}
	resize := opts.Resize
	for index := 0; ; index = (index + 1) % len(frames) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := write(out, "\r\x1b[2K"+frames[index].Text); err != nil {
			return err
		}
		next := after(frames[index].Duration)
		waiting := true
		for waiting {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-finished:
				return nil
			case _, ok := <-resize:
				if !ok {
					resize = nil
					continue
				}
				if !fits() {
					return nil
				}
			case <-next:
				waiting = false
			}
		}
	}
}

func write(out io.Writer, value string) error {
	n, err := io.WriteString(out, value)
	if err != nil {
		return fmt.Errorf("write animation: %w", err)
	}
	if n != len(value) {
		return fmt.Errorf("write animation: %w", io.ErrShortWrite)
	}
	return nil
}
