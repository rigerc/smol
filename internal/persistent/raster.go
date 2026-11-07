// Package persistent uploads text-face images for terminal-managed animation.
package persistent

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	faceroll "github.com/rigerc/faceroll/go"
)

//go:embed font/atlas.png font/chars.txt
var assets embed.FS

// Frame contains one complete PNG and its dwell time in milliseconds.
type Frame struct {
	PNG []byte
	MS  int
}

// Animation holds equally sized image frames and their reserved terminal width.
type Animation struct {
	Frames  []Frame
	Columns int
}

// Prepare rasterizes every frame before any terminal output. Unknown glyphs
// return an error so the caller can preserve the original static text instead.
func Prepare(animation faceroll.Animation, foreground color.RGBA, cellWidth, cellHeight int) (Animation, error) {
	if len(animation.Frames) < 2 || len(animation.Frames) > 128 {
		return Animation{}, errors.New("persistent animation needs 2–128 frames")
	}
	if cellWidth <= 0 || cellHeight <= 0 || cellWidth > 128 || cellHeight > 256 {
		return Animation{}, errors.New("invalid terminal cell dimensions")
	}
	data, err := assets.ReadFile("font/atlas.png")
	if err != nil {
		return Animation{}, err
	}
	atlas, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return Animation{}, err
	}
	chars, err := assets.ReadFile("font/chars.txt")
	if err != nil {
		return Animation{}, err
	}
	glyphs := make(map[rune]int)
	for i, r := range []rune(string(chars)) {
		glyphs[r] = i
	}
	columns := 0
	for _, frame := range animation.Frames {
		if frame.MS <= 0 {
			return Animation{}, errors.New("animation frame duration must be positive")
		}
		columns = max(columns, faceroll.CellWidth(frame.Text))
		for _, r := range frame.Text {
			if _, ok := glyphs[r]; !ok {
				return Animation{}, fmt.Errorf("no bundled glyph for %U", r)
			}
		}
	}
	if columns == 0 || columns > 256 {
		return Animation{}, errors.New("invalid face width")
	}
	result := Animation{Columns: columns}
	for _, frame := range animation.Frames {
		canvas := image.NewNRGBA(image.Rect(0, 0, columns*cellWidth, cellHeight))
		column := 0
		for _, r := range frame.Text {
			width := faceroll.CellWidth(string(r))
			if width == 0 {
				continue
			}
			paintGlyph(canvas, atlas, glyphs[r], column, width, cellWidth, cellHeight, foreground)
			column += width
		}
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, canvas); err != nil {
			return Animation{}, err
		}
		result.Frames = append(result.Frames, Frame{PNG: encoded.Bytes(), MS: frame.MS})
	}
	return result, nil
}

func paintGlyph(dst *image.NRGBA, atlas image.Image, index, column, width, cellWidth, cellHeight int, ink color.RGBA) {
	tile := image.Rect((index%16)*40, (index/16)*40, (index%16+1)*40, (index/16+1)*40)
	// Fit the glyph into its cell while preserving its source aspect ratio.
	bounds := image.Rectangle{}
	for y := tile.Min.Y; y < tile.Max.Y; y++ {
		for x := tile.Min.X; x < tile.Max.X; x++ {
			if _, _, _, a := atlas.At(x, y).RGBA(); a != 0 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if bounds.Empty() {
		return
	}
	scale := min(float64(cellWidth*width)/float64(max(20*width, bounds.Dx())), float64(cellHeight)/40)
	w, h := max(1, int(float64(bounds.Dx())*scale)), max(1, int(float64(bounds.Dy())*scale))
	left, top := column*cellWidth+(width*cellWidth-w)/2, (cellHeight-h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_, _, _, a := atlas.At(bounds.Min.X+x*bounds.Dx()/w, bounds.Min.Y+y*bounds.Dy()/h).RGBA()
			dst.SetNRGBA(left+x, top+y, color.NRGBA{R: ink.R, G: ink.G, B: ink.B, A: uint8(((a >> 8) * uint32(ink.A) / 255) & 255)})
		}
	}
}

// PlainStyle reports whether a face style can be reproduced by the glyph atlas.
func PlainStyle(bold, dim, italic, underline, reverse bool, background string) bool {
	return !bold && !dim && !italic && !underline && !reverse && strings.TrimSpace(background) == ""
}
