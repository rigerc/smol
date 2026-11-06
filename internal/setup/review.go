package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/render"
)

func (s *session) prepare(ctx context.Context) error {
	var preview string
	err := s.term.work(ctx, "Preparing review", func() error { var err error; preview, err = s.buildReview(ctx); return err })
	if err != nil {
		return err
	}
	if s.term.bridge != nil {
		return nil // The inline form owns the summary and scrollable TOML view.
	}
	verb := "Create"
	if s.snap.exists {
		verb = "REPLACE"
	}
	if err = s.term.say(verb + " " + s.path + "\nExact TOML to save:\n" + s.review); err != nil {
		return err
	}
	if _, err = fmt.Fprintln(s.term.out, "Preview: "+preview); err != nil {
		return err
	}
	if s.draft.Face.Random {
		if err = s.term.say("Face sample changes next run; no sample seed will be saved."); err != nil {
			return err
		}
	}
	return s.term.say(s.instructions)
}

func (s *session) reviewSummary() string {
	width := max(1, s.term.runtime.Width)
	modules := "Modules: " + strings.Join(s.selected, ", ")
	appearance := "Face: " + s.faceMode + " · Color: " + s.draft.Color + " · Icons: " + s.draft.Icons.Mode
	return ansi.Truncate(safe(modules), width, "…") + "\n" + ansi.Truncate(safe(appearance), width, "…") + "\nShell instructions: " + s.shell
}

func (s *session) buildReview(ctx context.Context) (string, error) {
	if err := s.draft.Validate(); err != nil {
		return "", err
	}
	text, err := config.TOML(s.draft)
	if err != nil {
		return "", err
	}
	s.review = text + "\n"
	opts := s.draft.Face
	if opts.Random {
		opts.Random = false
		seed := s.sampleSeed
		opts.Seed = &seed
	}
	toks, err := render.Parse(s.draft.Format)
	if err != nil {
		return "", err
	}
	renderOpts := render.Options{Icons: s.draft.Icons, ShowIcons: s.draft.Icons.Enabled(s.term.runtime.StderrTTY, s.term.runtime.Getenv("TERM")), Styles: s.draft.Style, Color: previewColor(s.draft.Color, s.term.runtime), Timeout: render.DefaultTimeout}
	env := s.env
	env.Display = &s.draft.Display
	moduleOpts := s.draft.Modules.Options()
	env.Options = &moduleOpts
	values := render.Fetch(ctx, toks, env, renderOpts)
	if contains(s.selected, "face") {
		animation, e := face.Build(opts)
		if e != nil {
			return "", e
		}
		values["face"] = modules.Result{Text: animation.Still}
	}
	// Only generated color escapes are trusted. Module output is already sanitized
	// by render; the transcript sanitizes user-derived paths and diagnostics.
	preview := render.Assemble(toks, values, renderOpts)
	s.instructions = shellInstructions(s.shell, s.path)
	return preview, nil
}
func previewColor(mode string, runtime Runtime) bool {
	if mode == "always" {
		return true
	}
	if mode == "never" {
		return false
	}
	return runtime.StderrTTY && runtime.Getenv("NO_COLOR") == "" && runtime.Getenv("TERM") != "dumb"
}
func shellInstructions(shell, path string) string {
	if shell == "none" {
		return ""
	}
	quoted := "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	if shell == "fish" {
		quoted = "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(path) + "'"
	}
	file := "~/." + shell + "rc"
	if shell == "fish" {
		file = "~/.config/fish/config.fish"
	}
	return "With smol on PATH, add this line manually to " + file + ":\nsmol --config " + quoted + "\nThis prints a banner on interactive startup; it is not a PS1/prompt hook."
}
func writeCompletion(stdout, stderr io.Writer, path, instructions string) error {
	_, outErr := fmt.Fprintln(stdout, path)
	_, errErr := fmt.Fprintln(stderr, "Saved "+safe(path))
	if instructions != "" {
		_, e := fmt.Fprintln(stderr, instructions)
		errErr = errors.Join(errErr, e)
	}
	return errors.Join(outErr, errErr)
}
