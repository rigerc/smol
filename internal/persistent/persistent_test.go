package persistent

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	faceroll "github.com/rigerc/faceroll/go"
)

func testTerminal() Terminal {
	return Terminal{ImageID: 42, Columns: 80, Column: 0, CellWidth: 12, CellHeight: 24, Foreground: color.RGBA{R: 12, G: 34, B: 56, A: 255}}
}

func textAnimation() faceroll.Animation {
	return faceroll.Animation{Frames: []faceroll.Frame{{Text: "(^.^)", MS: 250}, {Text: "(-.-)", MS: 75}}}
}

func TestPrepare(t *testing.T) {
	animation, err := Prepare(textAnimation(), testTerminal().Foreground, 12, 24)
	if err != nil {
		t.Fatal(err)
	}
	if animation.Columns != 5 || len(animation.Frames) != 2 {
		t.Fatalf("unexpected animation: %+v", animation)
	}
	for _, frame := range animation.Frames {
		img, err := png.Decode(bytes.NewReader(frame.PNG))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 60 || img.Bounds().Dy() != 24 {
			t.Fatal(img.Bounds())
		}
		ink, transparent := false, false
		for y := 0; y < 24; y++ {
			for x := 0; x < 60; x++ {
				rgba, ok := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				if !ok {
					t.Fatal("NRGBAModel did not produce NRGBA")
				}
				if rgba.A == 0 {
					transparent = true
				} else {
					ink = true
					if rgba.R != 12 || rgba.G != 34 || rgba.B != 56 {
						t.Fatalf("wrong ink: %+v", rgba)
					}
				}
			}
		}
		if !ink || !transparent {
			t.Fatal("frame must contain ink and transparent background")
		}
	}
}

func TestBundledGlyphCoverage(t *testing.T) {
	chars, err := assets.ReadFile("font/chars.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, preset := range faceroll.FacePresets {
		character, err := faceroll.CreatePresetCharacter(string(preset.ID), faceroll.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, animation := range character.Animations {
			for _, frame := range animation.Frames {
				for _, r := range frame.Text {
					if !strings.ContainsRune(string(chars), r) {
						t.Fatalf("preset %s: missing %U", preset.ID, r)
					}
				}
			}
		}
	}
	// Every literal used by the pinned generator was included in the atlas.
	for _, state := range faceroll.StateNames() {
		seed := "smol"
		character, err := faceroll.Roll(faceroll.RollOptions{Seed: &seed})
		if err != nil {
			t.Fatal(err)
		}
		animation := character.Animations[state]
		if len(animation.Frames) > 1 {
			if _, err := Prepare(animation, testTerminal().Foreground, 12, 24); err != nil {
				t.Fatal(state, err)
			}
		}
	}
}

func TestPrepareRejectsInvalidFrames(t *testing.T) {
	for _, tc := range []struct {
		animation faceroll.Animation
		w, h      int
	}{
		{faceroll.Animation{}, 12, 24},
		{textAnimation(), 0, 24}, {textAnimation(), 12, 0}, {textAnimation(), 129, 24}, {textAnimation(), 12, 257},
		{faceroll.Animation{Frames: []faceroll.Frame{{Text: "x", MS: 1}, {Text: "x", MS: 0}}}, 12, 24},
		{faceroll.Animation{Frames: []faceroll.Frame{{Text: "x", MS: 1}, {Text: "🦕", MS: 1}}}, 12, 24},
		{faceroll.Animation{Frames: []faceroll.Frame{{Text: "", MS: 1}, {Text: "", MS: 1}}}, 12, 24},
		{faceroll.Animation{Frames: []faceroll.Frame{{Text: strings.Repeat("x", 257), MS: 1}, {Text: "x", MS: 1}}}, 12, 24},
	} {
		if _, err := Prepare(tc.animation, color.RGBA{A: 255}, tc.w, tc.h); err == nil {
			t.Fatalf("accepted invalid animation: %+v", tc)
		}
	}
}

func TestVariableWidthFrames(t *testing.T) {
	a := faceroll.Animation{Frames: []faceroll.Frame{{Text: " (x) ", MS: 100}, {Text: "【x】", MS: 50}}}
	images, err := Prepare(a, color.RGBA{R: 255, A: 255}, 20, 40)
	if err != nil || images.Columns != 5 {
		t.Fatal(images.Columns, err)
	}
	for _, frame := range images.Frames {
		img, err := png.Decode(bytes.NewReader(frame.PNG))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 100 {
			t.Fatal(img.Bounds())
		}
	}
}

func TestDisplayProtocol(t *testing.T) {
	animation, err := Prepare(textAnimation(), testTerminal().Foreground, 12, 24)
	if err != nil {
		t.Fatal(err)
	}
	line := Line{Text: "left       right       end", Columns: 26, Faces: []int{5, 17}}
	var out bytes.Buffer
	if err := Display(context.Background(), &out, animation, line, testTerminal()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, part := range []string{"a=t,i=42,f=100,q=2", "a=f,i=42,f=100,z=75,q=2", "a=a,i=42,r=1,z=250,q=2", "\r\x1b[5C\x1b_Ga=p,i=42,p=1,c=5,r=1,C=1,q=2", "p=2,c=5", "a=a,i=42,s=3,v=1,q=2", "\r\x1b[26C\n"} {
		if !strings.Contains(text, part) {
			t.Fatalf("missing %q", part)
		}
	}
	if strings.Contains(text, "a=d") || strings.Contains(text, "?25") {
		t.Fatal("successful display must preserve image and cursor visibility")
	}
	// Both uploaded payloads must be complete PNGs, rather than GIF containers.
	for _, frame := range animation.Frames {
		if !strings.Contains(text, base64.StdEncoding.EncodeToString(frame.PNG)) {
			t.Fatal("missing PNG payload")
		}
	}
}

func TestDisplayValidation(t *testing.T) {
	valid := Animation{Columns: 3, Frames: []Frame{{PNG: []byte("png"), MS: 1}, {PNG: []byte("png"), MS: 2}}}
	for _, line := range []Line{
		{Text: "x", Columns: 80, Faces: []int{0}}, {Text: "x", Columns: 3}, {Text: "x", Columns: 3, Faces: []int{-1}}, {Text: "x", Columns: 3, Faces: []int{1}},
	} {
		var out bytes.Buffer
		if err := Display(context.Background(), &out, valid, line, testTerminal()); err == nil || out.Len() != 0 {
			t.Fatal("invalid placement accepted", err, out.String())
		}
	}
	line := Line{Text: "   ", Columns: 3, Faces: []int{0}}
	for _, animation := range []Animation{{}, {Columns: 3, Frames: []Frame{{MS: 1}, {PNG: []byte("x"), MS: 1}}}, {Columns: 3, Frames: []Frame{{PNG: []byte("x"), MS: 0}, {PNG: []byte("x"), MS: 1}}}} {
		if err := Display(context.Background(), io.Discard, animation, line, testTerminal()); err == nil {
			t.Fatal("invalid animation accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if err := Display(ctx, &out, valid, line, testTerminal()); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal(err, out.String())
	}
	if err := Display(context.Background(), &out, valid, line, testTerminal()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\r\x1b_Ga=p") || strings.Contains(out.String(), "[0C") {
		t.Fatal("left-edge placement moved right")
	}
}

type failingWriter struct {
	bytes.Buffer
	writes int
	fail   int
	short  bool
	cancel context.CancelFunc
}

func (w *failingWriter) WriteString(s string) (int, error) {
	w.writes++
	if w.cancel != nil {
		w.cancel()
	}
	if w.writes == w.fail {
		if w.short {
			return 0, nil
		}
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.WriteString(s)
}

func TestDisplayFailureCleanup(t *testing.T) {
	a := Animation{Columns: 3, Frames: []Frame{{PNG: []byte("a"), MS: 1}, {PNG: []byte("b"), MS: 1}}}
	line := Line{Text: "   ", Columns: 3, Faces: []int{0}}
	for fail := 1; fail <= 3; fail++ {
		for _, short := range []bool{false, true} {
			out := &failingWriter{fail: fail, short: short}
			err := Display(context.Background(), out, a, line, testTerminal())
			expected := io.ErrClosedPipe
			if short {
				expected = io.ErrShortWrite
			}
			if !errors.Is(err, expected) || !strings.Contains(out.String(), "\x18\x1b_Ga=d,d=I,i=42,q=2") {
				t.Fatal(fail, err, out.String())
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &failingWriter{cancel: cancel}
	if err := Display(ctx, out, a, line, testTerminal()); !errors.Is(err, context.Canceled) || !strings.Contains(out.String(), "a=d") {
		t.Fatal(err, out.String())
	}
}

func TestChunkedTransmission(t *testing.T) {
	for _, frame := range []bool{false, true} {
		data := bytes.Repeat([]byte{0, 1, 2}, 4000)
		var out bytes.Buffer
		if err := transmit(context.Background(), &out, "a=t,q=2", data, frame); err != nil {
			t.Fatal(err)
		}
		var encoded strings.Builder
		chunks := strings.Split(out.String(), "\x1b\\")
		for i, chunk := range chunks[:len(chunks)-1] {
			header, payload, ok := strings.Cut(chunk, ";")
			if !ok || len(payload) > 4096 {
				t.Fatal("invalid chunk")
			}
			if i > 0 && frame && !strings.Contains(header, "a=f") {
				t.Fatal("missing frame continuation action")
			}
			encoded.WriteString(payload)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded.String())
		if err != nil || !bytes.Equal(decoded, data) || !strings.Contains(out.String(), "m=0") {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	out := &failingWriter{cancel: cancel}
	if err := transmit(ctx, out, "a=t", make([]byte, 9000), false); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
