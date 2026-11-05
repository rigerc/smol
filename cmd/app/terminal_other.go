//go:build !linux

package main

import "os"

func terminalColumns(_ *os.File) int { return 0 }
