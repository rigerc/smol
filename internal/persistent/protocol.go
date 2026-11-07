package persistent

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/color"
	"io"
	"strings"
)

// Terminal describes the output terminal at detection time. Column is zero-based.
type Terminal struct {
	ImageID    uint32
	Columns    int
	Column     int
	CellWidth  int
	CellHeight int
	Foreground color.RGBA
}

// Line contains the rendered text and the column offsets of reserved face areas.
type Line struct {
	Text    string
	Columns int
	Faces   []int
}

// Fits leaves the final terminal column unused to avoid autowrap and scrolling.
func (t Terminal) Fits(line Line) bool {
	return t.ImageID != 0 && t.Columns > 0 && t.Column >= 0 && line.Columns > 0 && t.Column+line.Columns < t.Columns && len(line.Faces) > 0
}

// Display uploads and starts a looping image, then returns without a timer or
// background writer. Successful placements deliberately survive process exit.
func Display(ctx context.Context, out io.Writer, animation Animation, line Line, terminal Terminal) (err error) {
	if !terminal.Fits(line) || len(animation.Frames) < 2 || animation.Columns <= 0 {
		return errors.New("persistent animation does not fit terminal")
	}
	for _, offset := range line.Faces {
		if offset < 0 || offset+animation.Columns > line.Columns {
			return errors.New("invalid face placement")
		}
	}
	for _, frame := range animation.Frames {
		if len(frame.PNG) == 0 || frame.MS <= 0 {
			return errors.New("invalid persistent frame")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	id := terminal.ImageID
	defer func() {
		if err != nil {
			// CAN also aborts a partially written graphics command before deletion.
			err = errors.Join(err, checkedWrite(out, "\x18"+command(fmt.Sprintf("a=d,d=I,i=%d,q=2", id), "")))
		}
	}()
	for i, frame := range animation.Frames {
		if err := ctx.Err(); err != nil {
			return err
		}
		params := fmt.Sprintf("a=t,i=%d,f=100,q=2", id)
		if i > 0 {
			params = fmt.Sprintf("a=f,i=%d,f=100,z=%d,q=2", id, frame.MS)
		}
		if err := transmit(ctx, out, params, frame.PNG, i > 0); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var tail strings.Builder
	tail.WriteString(command(fmt.Sprintf("a=a,i=%d,r=1,z=%d,q=2", id, animation.Frames[0].MS), ""))
	tail.WriteString(line.Text)
	for i, offset := range line.Faces {
		tail.WriteString(cursorColumn(terminal.Column + offset))
		tail.WriteString(command(fmt.Sprintf("a=p,i=%d,p=%d,c=%d,r=1,C=1,q=2", id, i+1, animation.Columns), ""))
	}
	tail.WriteString(command(fmt.Sprintf("a=a,i=%d,s=3,v=1,q=2", id), ""))
	tail.WriteString(cursorColumn(terminal.Column + line.Columns))
	tail.WriteByte('\n')
	return checkedWrite(out, tail.String())
}

func cursorColumn(column int) string {
	if column == 0 {
		return "\r"
	}
	return fmt.Sprintf("\r\x1b[%dC", column)
}

func transmit(ctx context.Context, out io.Writer, params string, data []byte, frame bool) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	for len(encoded) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		size := min(4096, len(encoded))
		more := 0
		if size < len(encoded) {
			more = 1
		}
		if err := checkedWrite(out, command(fmt.Sprintf("%s,m=%d", params, more), encoded[:size])); err != nil {
			return err
		}
		encoded = encoded[size:]
		params = "q=2"
		if frame {
			params += ",a=f"
		}
	}
	return nil
}

func command(params, payload string) string {
	if payload != "" {
		params += ";" + payload
	}
	return "\x1b_G" + params + "\x1b\\"
}

func checkedWrite(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err != nil {
		return fmt.Errorf("write persistent animation: %w", err)
	}
	if n != len(text) {
		return fmt.Errorf("write persistent animation: %w", io.ErrShortWrite)
	}
	return nil
}
