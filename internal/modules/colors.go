package modules

import (
	"context"
	"fmt"
	"strings"
)

// colors returns plain swatches; the renderer applies trusted terminal colors.
func colors(_ context.Context, env Env) (Result, error) {
	shape := options(env).Colors.Shape
	var glyph string
	switch shape {
	case "blocks":
		glyph = "██"
	case "circles":
		glyph = "●"
	default:
		return Result{}, fmt.Errorf("colors: unknown shape %q", shape)
	}
	return Result{Text: strings.TrimSuffix(strings.Repeat(glyph+" ", 16), " ")}, nil
}
