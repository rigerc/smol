package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// terminalColumns uses the same ioctl for TTY detection and resize handling.
// A zero width means the output is not a terminal or its size is unknown.
func terminalColumns(file *os.File) int {
	var size struct{ Rows, Columns, X, Y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size))) //nolint:gosec // G103: TIOCGWINSZ writes exactly this four-uint16 winsize buffer.
	runtime.KeepAlive(file)
	if errno != 0 {
		return 0
	}
	return int(size.Columns)
}
