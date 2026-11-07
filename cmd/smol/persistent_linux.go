//go:build linux

package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/rigerc/smol/internal/persistent"
)

func detectPersistent(ctx context.Context, out *os.File) (persistent.Terminal, error) {
	size, err := unix.IoctlGetWinsize(terminalFD(out), unix.TIOCGWINSZ)
	if err != nil {
		return persistent.Terminal{}, err
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return persistent.Terminal{}, err
	}
	defer func() { _ = tty.Close() }()
	outputDevice, err := unix.IoctlGetInt(terminalFD(out), unix.TIOCGDEV)
	if err != nil {
		return persistent.Terminal{}, err
	}
	ttyDevice, err := unix.IoctlGetInt(terminalFD(tty), unix.TIOCGDEV)
	if err != nil {
		return persistent.Terminal{}, err
	}
	// /dev/tty has a different device node, so compare the underlying TTY IDs.
	if outputDevice != ttyDevice {
		return persistent.Terminal{}, errors.New("stdout is not the controlling terminal")
	}
	var identity [4]byte
	if _, err := rand.Read(identity[:]); err != nil {
		return persistent.Terminal{}, err
	}
	terminal := persistent.Terminal{ImageID: binary.BigEndian.Uint32(identity[:]) | 0x80000000, Columns: int(size.Col), CellWidth: 20, CellHeight: 40}
	if size.Col > 0 && size.Row > 0 && size.Xpixel > 0 && size.Ypixel > 0 {
		terminal.CellWidth = int(size.Xpixel / size.Col)
		terminal.CellHeight = int(size.Ypixel / size.Row)
	}
	return persistent.Probe(ctx, terminal, func(ctx context.Context, query string) (string, error) {
		return exchangePersistent(ctx, tty, query)
	})
}

func exchangePersistent(ctx context.Context, tty *os.File, query string) (reply string, err error) {
	fd := terminalFD(tty)
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", err
	}
	mode := *old
	mode.Lflag &^= unix.ICANON | unix.ECHO
	mode.Cc[unix.VMIN], mode.Cc[unix.VTIME] = 1, 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &mode); err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, unix.IoctlSetTermios(fd, unix.TCSETS, old)) }()
	if n, err := tty.WriteString(query); err != nil || n != len(query) {
		return "", fmt.Errorf("write terminal probe: %w", errors.Join(err, errors.New("incomplete terminal probe")))
	}
	deadline := time.Now().Add(200 * time.Millisecond)
	var response []byte
	for time.Now().Before(deadline) && len(response) < 8192 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}} //nolint:gosec // G115: terminal descriptors are kernel int file descriptors.
		if _, err := unix.Poll(fds, 10); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return "", err
		}
		if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			return "", errors.New("terminal closed during probe")
		}
		if fds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		var chunk [512]byte
		n, err := unix.Read(fd, chunk[:])
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return "", err
		}
		response = append(response, chunk[:n]...)
		if persistent.ProbeComplete(string(response)) {
			return string(response), nil
		}
	}
	return string(response), nil
}
