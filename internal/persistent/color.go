package persistent

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/rigerc/smol/internal/render"
)

// Foreground maps configured colors to pixels. Named/indexed colors use the
// standard xterm palette; terminal-default foreground comes from OSC 10.
func Foreground(value string, fallback color.RGBA) color.RGBA {
	for i, name := range render.ColorNames() {
		if name == value {
			return indexedColor(i)
		}
	}
	if hex, ok := strings.CutPrefix(value, "#"); ok && len(hex) == 6 {
		if n, err := strconv.ParseUint(hex, 16, 24); err == nil {
			return color.RGBA{R: uint8(n >> 16), G: uint8((n >> 8) & 255), B: uint8(n & 255), A: 255}
		}
	}
	if index, ok := strings.CutPrefix(value, "index:"); ok {
		if n, err := strconv.Atoi(index); err == nil && n >= 0 && n <= 255 {
			return indexedColor(n)
		}
	}
	return fallback
}

func indexedColor(index int) color.RGBA {
	if index < 0 || index > 255 {
		return color.RGBA{}
	}
	if index < 16 {
		palette := [16]color.RGBA{
			{R: 0, G: 0, B: 0, A: 255}, {R: 128, G: 0, B: 0, A: 255}, {R: 0, G: 128, B: 0, A: 255}, {R: 128, G: 128, B: 0, A: 255},
			{R: 0, G: 0, B: 128, A: 255}, {R: 128, G: 0, B: 128, A: 255}, {R: 0, G: 128, B: 128, A: 255}, {R: 192, G: 192, B: 192, A: 255},
			{R: 128, G: 128, B: 128, A: 255}, {R: 255, G: 0, B: 0, A: 255}, {R: 0, G: 255, B: 0, A: 255}, {R: 255, G: 255, B: 0, A: 255},
			{R: 0, G: 0, B: 255, A: 255}, {R: 255, G: 0, B: 255, A: 255}, {R: 0, G: 255, B: 255, A: 255}, {R: 255, G: 255, B: 255, A: 255},
		}
		return palette[index]
	}
	if index >= 232 {
		v := uint8(8 + (index-232)*10)
		return color.RGBA{R: v, G: v, B: v, A: 255}
	}
	levels := [6]uint8{0, 95, 135, 175, 215, 255}
	index -= 16
	return color.RGBA{R: levels[index/36], G: levels[(index/6)%6], B: levels[index%6], A: 255}
}
