// Package brand owns smol's FIGlet Standard wordmark and theme palettes.
package brand

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/lsferreira42/figlet-go/figlet"
)

// Art returns the authoritative FIGlet Standard lettering without ANSI escapes.
func Art() (string, error) {
	text, err := figlet.Render("smol", figlet.WithFont("standard"), figlet.WithWidth(120), figlet.WithJustification(0), figlet.WithParser("terminal"))
	return strings.TrimRight(text, "\n"), err
}

// LightBackground uses an existing terminal hint without querying or blocking.
// COLORFGBG's final field is the background palette index; unknown means dark.
func LightBackground(hint string) bool {
	parts := strings.Split(hint, ";")
	n, err := strconv.Atoi(parts[len(parts)-1])
	return err == nil && (n == 7 || n == 15)
}

// Gradient applies a horizontal Phoenix gradient to the fixed ASCII lettering.
func Gradient(art string, light bool) string {
	stops := Stops(light)
	colors := make([]color.RGBA, len(stops))
	for i, stop := range stops {
		n, _ := strconv.ParseUint(stop[1:], 16, 24)
		colors[i] = color.RGBA{R: uint8(n >> 16), G: uint8((n >> 8) & 255), B: uint8(n & 255), A: 255}
	}
	lines := strings.Split(art, "\n")
	width := 1
	for _, line := range lines {
		width = max(width, len(line))
	}
	var out strings.Builder
	for row, line := range lines {
		if row > 0 {
			out.WriteByte('\n')
		}
		for x, r := range line {
			if r == ' ' {
				out.WriteRune(r)
				continue
			}
			position := float64(x) * float64(len(colors)-1) / float64(max(1, width-1))
			segment := min(int(position), len(colors)-2)
			fraction := position - float64(segment)
			from, to := colors[segment], colors[segment+1]
			blend := func(a, b uint8) int { return int(float64(a) + (float64(b)-float64(a))*fraction) }
			fmt.Fprintf(&out, "\x1b[38;2;%d;%d;%dm%c", blend(from.R, to.R), blend(from.G, to.G), blend(from.B, to.B), r)
		}
		out.WriteString("\x1b[0m")
	}
	return out.String()
}
