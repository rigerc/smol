//go:build !linux

package main

import (
	"context"
	"errors"
	"os"

	"github.com/rigerc/smol/internal/persistent"
)

func detectPersistent(context.Context, *os.File) (persistent.Terminal, error) {
	return persistent.Terminal{}, errors.New("persistent animation detection is unavailable on this platform")
}
