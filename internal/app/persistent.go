package app

import (
	"context"
	"errors"
	"io"

	faceroll "github.com/rigerc/faceroll/go"

	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/persistent"
	"github.com/rigerc/smol/internal/render"
)

func runPersistent(ctx context.Context, out, stderr io.Writer, runtime Runtime, animation faceroll.Animation, toks []render.Token, values map[string]modules.Result, opts render.Options, getenv func(string) string) (bool, int) {
	// Multiplexer support needs placeholder placements and passthrough, rather
	// than direct image placements. Keep their output static for now.
	if runtime.Persistent == nil || len(animation.Frames) < 2 || getenv("TMUX") != "" || getenv("STY") != "" {
		return false, 0
	}
	style := opts.Styles["face"]
	if opts.Color && !persistent.PlainStyle(style.Bold, style.Dim, style.Italic, style.Underline, style.Reverse, style.BG) {
		return false, 0
	}
	terminal, err := runtime.Persistent(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return true, 130
		}
		if opts.Debug != nil {
			opts.Debug("persistent animation: " + err.Error())
		}
		return false, 0
	}
	ink := terminal.Foreground
	if opts.Color {
		ink = persistent.Foreground(style.FG, ink)
	}
	images, err := persistent.Prepare(animation, ink, terminal.CellWidth, terminal.CellHeight)
	if err != nil {
		if opts.Debug != nil {
			opts.Debug("persistent animation: " + err.Error())
		}
		return false, 0
	}
	layout := render.AssembleFaceLine(toks, values, opts, images.Columns)
	line := persistent.Line{Text: layout.Text, Columns: layout.Columns, Faces: layout.Faces}
	if !terminal.Fits(line) {
		return false, 0
	}
	if err := persistent.Display(ctx, out, images, line, terminal); err != nil {
		if errors.Is(err, context.Canceled) {
			return true, 130
		}
		report(stderr, "smol: "+err.Error())
		return true, 1
	}
	return true, 0
}
