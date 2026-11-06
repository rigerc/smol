//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSetupChild(_ *testing.T) {
	if os.Getenv("SMOL_SETUP_TEST_CHILD") != "1" {
		return
	}
	os.Args = []string{"smol", "init", "--config", os.Getenv("SMOL_SETUP_TEST_CONFIG")}
	os.Exit(run())
}

func TestSetupTerminal(t *testing.T) {
	for _, mode := range []string{"basic-save", "rich-save", "rich-fallback-save", "rich-cancel", "rich-term", "narrow-cancel"} {
		t.Run(mode, func(t *testing.T) {
			fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
			if err != nil {
				t.Fatal(err)
			}
			master := os.NewFile(uintptr(fd), "ptmx") //nolint:gosec // G115: fd is a successful, nonnegative unix.Open result.
			defer func() { _ = master.Close() }()
			if err = unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
				t.Fatal(err)
			}
			number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
			if err != nil {
				t.Fatal(err)
			}
			slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = slave.Close() }()
			columns := uint16(80)
			if mode == "narrow-cancel" {
				columns = 19
			}
			rows := uint16(24)
			if mode == "rich-fallback-save" {
				rows = 10
			}
			if err = unix.IoctlSetWinsize(terminalFD(slave), unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: columns}); err != nil {
				t.Fatal(err)
			}
			before, err := unix.IoctlGetTermios(terminalFD(slave), unix.TCGETS)
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "nested", "config.toml")
			command := exec.Command(executable, "-test.run=^TestSetupChild$") //nolint:gosec // G204: executable is this test binary, with a fixed helper test.
			term := "xterm-256color"
			if mode == "basic-save" {
				term = "dumb"
			}
			command.Env = append(os.Environ(), "SMOL_SETUP_TEST_CHILD=1", "SMOL_SETUP_TEST_CONFIG="+path, "TERM="+term, "NO_COLOR=1")
			command.Stdin = slave
			command.Stderr = slave
			var stdout bytes.Buffer
			command.Stdout = &stdout
			command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
			if err = command.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			defer func() {
				if !waited {
					_ = command.Process.Kill()
					_ = command.Wait()
				}
			}()
			chunks := make(chan []byte, 32)
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				defer close(chunks)
				buf := make([]byte, 4096)
				for {
					n, e := master.Read(buf)
					if n > 0 {
						data := bytes.Clone(buf[:n])
						select {
						case chunks <- data:
						case <-stop:
							return
						}
					}
					if e != nil {
						return
					}
				}
			}()
			var transcript strings.Builder
			deadline := time.NewTimer(10 * time.Second)
			defer deadline.Stop()
			ready := false
			for !ready {
				select {
				case data, ok := <-chunks:
					if !ok {
						t.Fatal("PTY closed before prompt")
					}
					transcript.Write(data)
					ready = strings.Contains(transcript.String(), "Continue")
				case <-deadline.C:
					t.Fatal("timed out waiting for prompt: " + transcript.String())
				}
			}
			switch mode {
			case "rich-save", "rich-fallback-save":
				_, err = io.WriteString(master, "\r")
				// Bubble Tea may retain the unchanged "field" suffix when replacing uptime with mem.
				prompts := []string{"Configuration preset", "Face mode", "System modules", "os field", "kernel field", "uptime field", "mem ", "Module order", "Output icons", "Icon set", "Output color policy", "Module styles", "Print shell startup instructions", "Review configuration", "Create "}
				offset := transcript.Len()
				for i, prompt := range prompts {
					found := false
					for !found {
						select {
						case data, ok := <-chunks:
							if !ok {
								t.Fatal("PTY closed")
							}
							transcript.Write(data)
							found = strings.Contains(transcript.String()[offset:], prompt)
						case <-deadline.C:
							t.Fatal("waiting for " + prompt + ": " + transcript.String())
						}
					}
					offset = transcript.Len()
					answer := "\r"
					if i == len(prompts)-1 {
						answer = "\x1b[B\r"
						if mode == "rich-fallback-save" {
							answer = "2\n"
						}
					}
					if _, err = io.WriteString(master, answer); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "rich-save" && strings.Contains(transcript.String(), ":back / :cancel") {
					t.Fatal("80x24 wizard switched to numbered forms")
				}
			case "basic-save":
				_, err = io.WriteString(master, strings.Repeat("\n", 15)+"2\n")
			case "rich-term":
				err = command.Process.Signal(syscall.SIGTERM)
			case "narrow-cancel":
				_, err = io.WriteString(master, ":cancel\n")
			default:
				_, err = master.Write([]byte{3})
			}
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			completed := false
			for !completed {
				select {
				case data := <-chunks:
					transcript.Write(data)
				case err = <-done:
					completed = true
					waited = true
				case <-deadline.C:
					t.Fatal("timed out finishing setup: " + transcript.String())
				}
			}
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			want := 130
			if mode == "basic-save" || mode == "rich-save" || mode == "rich-fallback-save" {
				want = 0
			}
			if mode == "rich-term" {
				want = 143
			}
			if code != want {
				t.Fatalf("exit %d want %d: %s", code, want, transcript.String())
			}
			expected := ""
			if want == 0 {
				expected = path + "\n"
			}
			if (mode == "rich-save" || mode == "rich-fallback-save") && !strings.Contains(transcript.String(), "Saved ") {
				t.Fatal("missing save completion: " + transcript.String())
			}
			if stdout.String() != expected {
				t.Fatalf("stdout %q want %q", stdout.String(), expected)
			}
			after, e := unix.IoctlGetTermios(terminalFD(slave), unix.TCGETS)
			if e != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("terminal not restored: %v", e)
			}
			for _, sequence := range []string{"\x1b[?1049h", "\x1b[2J"} {
				if strings.Contains(transcript.String(), sequence) {
					t.Fatal("setup cleared the terminal")
				}
			}
			if want != 0 {
				if _, e = os.Stat(filepath.Dir(path)); !os.IsNotExist(e) {
					t.Fatalf("cancel created destination parent: %v", e)
				}
			}
		})
	}
}
