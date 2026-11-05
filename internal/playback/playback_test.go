package playback

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func frames() []Frame {
	return []Frame{{Text: "long", Width: 4, Duration: 10 * time.Millisecond}, {Text: "x", Width: 1, Duration: 20 * time.Millisecond}}
}

func TestFallback(t *testing.T) {
	for _, columns := range []func() int{nil, func() int { return 0 }, func() int { return 4 }} {
		var out bytes.Buffer
		if err := Run(context.Background(), &out, frames(), "still", Options{Columns: columns}); err != nil {
			t.Fatal(err)
		}
		if out.String() != "still\n" {
			t.Fatalf("unexpected controls: %q", out.String())
		}
	}
	var out bytes.Buffer
	if err := Run(context.Background(), &out, nil, "still", Options{Columns: func() int { return 80 }}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "still\n" {
		t.Fatal(out.String())
	}
}

func TestTimingAndDuration(t *testing.T) {
	var out bytes.Buffer
	finished := make(chan time.Time, 1)
	ticks := make(chan time.Time, 1)
	ticks <- time.Time{}
	calls := 0
	after := func(d time.Duration) <-chan time.Time {
		calls++
		switch calls {
		case 1:
			if d != time.Second {
				t.Fatalf("duration=%v", d)
			}
			return finished
		case 2:
			if d != 10*time.Millisecond {
				t.Fatalf("first dwell=%v", d)
			}
			return ticks
		case 3:
			if d != 20*time.Millisecond {
				t.Fatalf("second dwell=%v", d)
			}
			finished <- time.Time{}
			return nil
		default:
			t.Fatal("playback did not stop")
			return nil
		}
	}
	err := Run(context.Background(), &out, frames(), "still", Options{Columns: func() int { return 80 }, After: after, Duration: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	want := "\x1b[?25l\r\x1b[2Klong\r\x1b[2Kx\x1b[?25h\n"
	if out.String() != want {
		t.Fatalf("got %q want %q", out.String(), want)
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	err := Run(ctx, &out, frames(), "still", Options{Columns: func() int { return 80 }, After: func(time.Duration) <-chan time.Time { cancel(); return nil }})
	if !errors.Is(err, context.Canceled) || !strings.HasSuffix(out.String(), "\x1b[?25h\n") {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
	before := out.Len()
	if err := Run(ctx, &out, frames(), "still", Options{}); !errors.Is(err, context.Canceled) || out.Len() != before {
		t.Fatalf("pre-canceled run: %v", err)
	}
}

func TestResize(t *testing.T) {
	var out bytes.Buffer
	columns := 80
	resized := make(chan os.Signal, 1)
	err := Run(context.Background(), &out, frames(), "still", Options{
		Columns: func() int { return columns }, Resize: resized,
		After: func(time.Duration) <-chan time.Time { columns = 2; resized <- syscall.SIGWINCH; return nil },
	})
	if err != nil || !strings.HasSuffix(out.String(), "\x1b[?25h\n") || strings.Contains(out.String(), "\r\x1b[2Kx") {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
}

func TestSingleFrameAndInvalidFrame(t *testing.T) {
	var out bytes.Buffer
	err := Run(context.Background(), &out, frames()[:1], "still", Options{Columns: func() int { return 80 }, After: func(time.Duration) <-chan time.Time { t.Fatal("single frame scheduled timer"); return nil }})
	if err != nil || out.String() != "long\n" {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
	if err := Run(context.Background(), &out, []Frame{{Text: "bad", Width: 3}}, "still", Options{}); err == nil {
		t.Fatal("accepted nonpositive frame duration")
	}
}

type failingWriter struct {
	calls  int
	failAt int
	short  bool
	data   bytes.Buffer
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		if w.short {
			return 0, nil
		}
		return 0, io.ErrClosedPipe
	}
	return w.data.Write(p)
}

func TestWriteFailures(t *testing.T) {
	for _, failAt := range []int{1, 2, 3} {
		for _, short := range []bool{false, true} {
			out := &failingWriter{failAt: failAt, short: short}
			done := make(chan time.Time, 1)
			done <- time.Time{}
			err := Run(context.Background(), out, frames(), "still", Options{Columns: func() int { return 80 }, Duration: time.Second, After: func(d time.Duration) <-chan time.Time {
				if d == time.Second {
					return done
				}
				return nil
			}})
			want := io.ErrClosedPipe
			if short {
				want = io.ErrShortWrite
			}
			if !errors.Is(err, want) {
				t.Fatalf("failAt=%d short=%v err=%v", failAt, short, err)
			}
			if failAt < 3 && !strings.HasSuffix(out.data.String(), "\x1b[?25h\n") {
				t.Fatalf("cleanup missing: %q", out.data.String())
			}
		}
	}
}
