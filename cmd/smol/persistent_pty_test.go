//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestPersistentChild(_ *testing.T) {
	if os.Getenv("SMOL_PERSISTENT_TEST_CHILD") != "1" {
		return
	}
	os.Args = []string{"smol", "--config", os.Getenv("SMOL_PERSISTENT_TEST_CONFIG"), "--format", "left {face} right", "--face-preset", "kitty", "--face-state", "working", "--animate-persist", "--color", "never"}
	os.Exit(run())
}

func persistentPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(fd), "ptmx") //nolint:gosec // G115: fd is a successful, nonnegative unix.Open result.
	t.Cleanup(func() { _ = master.Close() })
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
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
	t.Cleanup(func() { _ = slave.Close() })
	if err := unix.IoctlSetWinsize(terminalFD(slave), unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 100, Xpixel: 1000, Ypixel: 576}); err != nil {
		t.Fatal(err)
	}
	return master, slave
}

func TestPersistentTerminal(t *testing.T) {
	for _, mode := range []string{"supported", "unsupported", "timeout", "interrupted"} {
		t.Run(mode, func(t *testing.T) {
			master, slave := persistentPTY(t)
			before, err := unix.IoctlGetTermios(terminalFD(slave), unix.TCGETS)
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(executable, "-test.run=^TestPersistentChild$") //nolint:gosec // G204: executable is this test binary with a fixed helper test.
			command.Env = append(os.Environ(), "SMOL_PERSISTENT_TEST_CHILD=1", "SMOL_PERSISTENT_TEST_CONFIG="+t.TempDir()+"/missing.toml", "TERM=xterm-ghostty", "TMUX=", "STY=")
			command.Stdin, command.Stdout, command.Stderr = slave, slave, slave
			command.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			exited := false
			defer func() {
				if !exited {
					_ = command.Process.Kill()
					<-done
				}
			}()
			deadline := time.Now().Add(5 * time.Second)
			var output strings.Builder
			responded := false
			var childErr error
			for time.Now().Before(deadline) {
				poll := []unix.PollFd{{Fd: int32(terminalFD(master)), Events: unix.POLLIN}} //nolint:gosec // G115: master is an open kernel file descriptor.
				if _, err := unix.Poll(poll, 10); err != nil && !errors.Is(err, unix.EINTR) {
					t.Fatal(err)
				}
				if poll[0].Revents&unix.POLLIN != 0 {
					var buf [8192]byte
					n, err := master.Read(buf[:])
					if err != nil {
						t.Fatal(err)
					}
					output.Write(buf[:n])
				}
				if !responded && strings.Contains(output.String(), "\x1b[c") {
					responded = true
					id := regexp.MustCompile(`a=f,i=([0-9]+)`).FindStringSubmatch(output.String())
					if len(id) != 2 {
						t.Fatal("missing frame probe", output.String())
					}
					switch mode {
					case "supported":
						_, err = fmt.Fprintf(master, "\x1b_Gi=%s;OK\x1b\\\x1b[1;1R\x1b]10;rgb:ffff/ffff/ffff\x1b\\\x1b[?1;2c", id[1])
					case "unsupported":
						_, err = fmt.Fprintf(master, "\x1b_Gi=%s;EINVAL\x1b\\\x1b[1;1R\x1b[?1;2c", id[1])
					case "interrupted":
						err = command.Process.Signal(os.Interrupt)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				select {
				case childErr = <-done:
					exited = true
				default:
				}
				if exited {
					// Drain bytes written immediately before the process exited.
					poll[0].Revents = 0
					if _, err := unix.Poll(poll, 0); err != nil {
						t.Fatal(err)
					}
					if poll[0].Revents&unix.POLLIN == 0 {
						break
					}
				}
			}
			if !exited || !responded {
				t.Fatal("child did not finish terminal probe", output.String())
			}
			if mode == "interrupted" {
				var exit *exec.ExitError
				if !errors.As(childErr, &exit) || exit.ExitCode() != 130 {
					t.Fatal("wrong interrupt result", childErr)
				}
			} else if childErr != nil {
				t.Fatal(childErr, output.String())
			}
			after, err := unix.IoctlGetTermios(terminalFD(slave), unix.TCGETS)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("terminal input settings were not restored")
			}
			animated := strings.Contains(output.String(), "s=3,v=1,q=2")
			if animated != (mode == "supported") {
				t.Fatal(mode, "incorrect animation result", output.String())
			}
			if mode == "supported" && !strings.Contains(output.String(), "C=1,q=2") {
				t.Fatal("placement moves cursor")
			}
		})
	}
}
