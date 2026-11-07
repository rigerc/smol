package persistent

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"
)

// Probe checks frame loading using an undisplayed one-pixel image. The image is
// deleted in the same request even when the terminal never answers. Exchange
// owns bounded terminal I/O and must restore input settings before returning.
func Probe(ctx context.Context, terminal Terminal, exchange func(context.Context, string) (string, error)) (Terminal, error) {
	if terminal.ImageID == 0 || terminal.Columns <= 0 {
		return Terminal{}, errors.New("invalid terminal probe")
	}
	id := terminal.ImageID
	query := command(fmt.Sprintf("a=t,i=%d,f=32,s=1,v=1,q=2", id), "AAAAAA==") +
		command(fmt.Sprintf("a=f,i=%d,f=32,s=1,v=1", id), "AAAAAA==") +
		command(fmt.Sprintf("a=d,d=I,i=%d,q=2", id), "") + "\x1b[6n\x1b]10;?\x1b\\\x1b[c"
	reply, err := exchange(ctx, query)
	if err != nil {
		return Terminal{}, err
	}
	ok := false
	for _, match := range regexp.MustCompile("\x1b_G([^;]*);OK\x1b\\\\").FindAllStringSubmatch(reply, -1) {
		for _, param := range strings.Split(match[1], ",") {
			if param == fmt.Sprintf("i=%d", id) {
				ok = true
			}
		}
	}
	cursor := regexp.MustCompile("\x1b\\[[0-9]+;([0-9]+)R").FindStringSubmatch(reply)
	if !ok || len(cursor) != 2 {
		return Terminal{}, errors.New("terminal does not support graphics animation")
	}
	column, err := strconv.Atoi(cursor[1])
	if err != nil || column < 1 || column > terminal.Columns {
		return Terminal{}, errors.New("invalid terminal cursor position")
	}
	terminal.Column = column - 1
	terminal.Foreground = color.RGBA{R: 224, G: 224, B: 224, A: 255}
	match := regexp.MustCompile("\x1b]10;rgb:([0-9a-fA-F]{1,4})/([0-9a-fA-F]{1,4})/([0-9a-fA-F]{1,4})(?:\x07|\x1b\\\\)").FindStringSubmatch(reply)
	if len(match) == 4 {
		terminal.Foreground = color.RGBA{R: colorComponent(match[1]), G: colorComponent(match[2]), B: colorComponent(match[3]), A: 255}
	}
	return terminal, nil
}

func colorComponent(hex string) uint8 {
	value, err := strconv.ParseUint(hex, 16, 16)
	if err != nil {
		return 0
	}
	maximum := (uint64(1) << (4 * len(hex))) - 1
	return uint8(value * 255 / maximum) //nolint:gosec // G115: a parsed channel normalized by its maximum is in 0–255.
}

// ProbeComplete reports the final primary-device-attributes reply, which
// follows the graphics, cursor and foreground responses on supported terminals.
func ProbeComplete(reply string) bool {
	return regexp.MustCompile("\x1b\\[[?>]?[0-9;]*c").MatchString(reply)
}
