package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ishan5ain/tuiweave/reorder"
	"github.com/pelletier/go-toml/v2"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/render"
	"github.com/rigerc/smol/presets"
)

// Runtime contains process resources required by interactive setup. Destination
// inspection and writes use the local filesystem; module reads use the Env FS.
type Runtime struct {
	Input     io.Reader
	Getenv    func(string) string
	Entropy   func(context.Context) (string, error)
	CWD       string
	StdinTTY  bool
	StderrTTY bool
	Basic     bool
	Width     int
	Rows      int
}

type session struct {
	term          *terminal
	env           modules.Env
	path          string
	snap          snapshot
	draft         config.Config
	state         string
	preset        string
	selected      []string
	fields        map[string]string
	queue         []string
	index         int
	styleProperty string
	dirty         bool
	faceMode      string
	sampleSeed    string
	shell         string
	review        string
	instructions  string
	store         store
}

// Run executes setup, keeping all interaction on stderr and emitting a saved
// absolute path to stdout only after publication.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime Runtime, env modules.Env) int {
	inv, err := parse(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "smol init:", err)
		return 2
	}
	if inv.help {
		if _, err = fmt.Fprintln(stderr, "Usage: smol init [--config PATH | --config=PATH] [--help]"); err != nil {
			return 1
		}
		return 0
	}
	if runtime.Getenv == nil {
		runtime.Getenv = func(string) string { return "" }
	}
	if !runtime.StdinTTY || !runtime.StderrTTY || runtime.Input == nil {
		_, _ = fmt.Fprintln(stderr, "smol init: interactive setup requires terminal stdin and stderr; use --print-default-config for manual setup.")
		return 2
	}
	path, err := destination(inv, runtime.CWD, runtime.Getenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "smol init:", err)
		return 2
	}
	if runtime.Width <= 0 {
		runtime.Width = 80
	}
	if runtime.Getenv("TERM") == "dumb" || runtime.Getenv("TERM") == "" {
		runtime.Basic = true
	}
	t := &terminal{runtime: runtime, out: stderr, env: env}
	defer func() { _ = t.close() }()
	if err = t.welcome(); err != nil {
		return finishError(ctx, t, err)
	}
	s := session{term: t, env: env, path: path, state: "inspecting", shell: detectShell(runtime.Getenv("SHELL"))}
	if !runtime.Basic && runtime.Width >= 40 {
		return runWizard(ctx, &s, stdout)
	}
	if err = s.load(ctx); err != nil {
		return finishError(ctx, t, err)
	}
	return s.run(ctx, stdout)
}

func (s *session) run(ctx context.Context, stdout io.Writer) int {
	t := s.term
	var err error
	for {
		if err = ctx.Err(); err != nil {
			return finishError(ctx, t, err)
		}
		if s.state == "review.ready" {
			if err = s.prepare(ctx); err != nil {
				if ctx.Err() != nil {
					return finishError(ctx, t, err)
				}
				if e := t.say("Could not prepare review: " + err.Error()); e != nil {
					return 1
				}
				action, e := t.ask(ctx, question{title: "Preview failed", choices: []string{"Retry", "Edit modules", "Edit face", "Exit"}, value: "Retry"})
				if e != nil {
					return finishError(ctx, t, e)
				}
				if action == "Exit" {
					return 1
				}
				if action == "Edit modules" {
					s.state = "modules.ready"
				}
				if action == "Edit face" {
					s.state = "face.mode"
				}
				continue
			}
		}
		if s.state == "save.pending" {
			var published bool
			var commitErr error
			saveErr := t.work(ctx, "Saving configuration", func() error {
				published, commitErr = s.store.commit(ctx, s.path, s.snap, []byte(s.review))
				return commitErr
			})
			if published {
				// Publication wins over a simultaneously delivered cancellation.
				if ctx.Err() == nil {
					commitErr = saveErr
				}
				outErr := errors.Join(commitErr, writeCompletion(stdout, t.out, s.path, s.instructions))
				if outErr != nil {
					_, _ = fmt.Fprintln(t.out, "Configuration WAS saved to", safe(s.path), "but completion/durability failed:", safe(outErr.Error()))
					return 1
				}
				return 0
			}
			if ctx.Err() != nil {
				return finishError(ctx, t, errors.Join(ctx.Err(), saveErr))
			}
			if err = t.say("Configuration was not saved: " + saveErr.Error()); err != nil {
				return 1
			}
			if errors.Is(saveErr, errConflict) {
				if e := s.refresh(ctx); e != nil {
					return finishError(ctx, t, e)
				}
			} else {
				action, e := t.ask(ctx, question{title: "Save failed", choices: []string{"Review and retry", "Exit"}, value: "Review and retry"})
				if e != nil {
					return finishError(ctx, t, e)
				}
				if action == "Exit" {
					return 1
				}
			}
			s.state = "review.ready"
			continue
		}
		q := s.question()
		q.preview = s.previewDraft()
		value, e := t.ask(ctx, q)
		if errors.Is(e, errBack) {
			if s.state == "destination.ready" {
				return finishError(ctx, t, errCancel)
			}
			s.back()
			continue
		}
		if e != nil {
			return finishError(ctx, t, e)
		}
		if e = s.accept(ctx, value); e != nil {
			if errors.Is(e, errCancel) || errors.Is(e, errAbort) {
				return finishError(ctx, t, e)
			}
			if ctx.Err() != nil {
				return finishError(ctx, t, e)
			}
			if err = t.say(e.Error()); err != nil {
				return 1
			}
		}
	}
}
func finishError(ctx context.Context, t *terminal, err error) int {
	code := 1
	if errors.Is(err, errCancel) || ctx.Err() != nil {
		code = 130
		var exit interface{ ExitCode() int }
		if errors.As(context.Cause(ctx), &exit) {
			code = exit.ExitCode()
		}
	}
	message := "Setup failed. Configuration was not saved: " + err.Error()
	if code == 130 {
		message = "Cancelled. Configuration was not saved."
	}
	if code == 143 {
		message = "Terminated before saving."
	}
	if strings.Contains(err.Error(), "leftover ") {
		message += "\n" + err.Error()
	}
	if e := t.say(message); e != nil && code == 1 {
		return 1
	}
	return code
}
func (s *session) load(ctx context.Context) error {
	for {
		snap, err := inspectTarget(s.path)
		if err != nil {
			if e := s.term.say("Cannot inspect destination: " + err.Error()); e != nil {
				return e
			}
			action, e := s.term.ask(ctx, question{title: "Inspection failed", choices: []string{"Retry", "Exit"}, value: "Retry"})
			if e != nil {
				return e
			}
			if action == "Exit" {
				return err
			}
			continue
		}
		s.snap = snap
		s.draft = config.Default()
		s.preset = "default"
		if snap.exists {
			cfg, e := config.Parse(s.path, snap.data)
			if e != nil {
				if err = s.term.say("WARNING: existing configuration is malformed; Existing is unavailable. Replacement requires explicit confirmation.\n" + e.Error()); err != nil {
					return err
				}
			} else {
				s.draft = cfg
				s.preset = "existing"
			}
		}
		s.derive()
		s.state = "destination.ready"
		return nil
	}
}
func (s *session) derive() {
	toks, _ := render.Parse(s.draft.Format)
	s.selected = render.ModuleNames(toks)
	s.fields = make(map[string]string)
	seen := map[string]bool{}
	for _, tok := range toks {
		if tok.Module != "" && !seen[tok.Module] {
			if tok.Field != "" {
				s.fields[tok.Module] = tok.Field
			}
			seen[tok.Module] = true
		}
	}
	s.faceMode = "disabled"
	if contains(s.selected, "face") {
		switch {
		case s.draft.Face.Random:
			s.faceMode = "random-each-run"
		case s.draft.Face.Preset != "":
			s.faceMode = "preset"
		default:
			s.faceMode = "seed"
		}
	}
	s.dirty = false
}
func (s *session) question() question {
	q := question{}
	switch s.state {
	case "destination.ready":
		q = question{title: "Destination", label: "Destination", action: true, description: "Fixed for this session.", choices: []string{"Continue", "Cancel"}, value: "Continue"}
	case "preset.ready":
		choices := []string{"default", "minimal", "banner", "prompt", "server", "icons"}
		if s.preset == "existing" || s.snap.exists && validConfig(s.path, s.snap.data) {
			choices = append([]string{"existing"}, choices...)
		}
		q = question{title: "Configuration preset", choices: choices, value: s.preset}
	case "face.mode":
		q = question{title: "Face mode", choices: []string{"disabled", "preset", "seed", "random-once", "random-each-run"}, value: s.faceMode}
	case "face.identity":
		if s.faceMode == "preset" {
			choices := face.Presets()
			sort.Strings(choices)
			value := s.draft.Face.Preset
			if value == "" {
				value = choices[0]
			}
			q = question{title: "Face preset", choices: choices, value: value}
		} else {
			value := "smol"
			if s.draft.Face.Seed != nil {
				value = *s.draft.Face.Seed
			}
			q = question{title: "Face seed (1–256 characters)", value: value, seed: true, validate: func(v string) error {
				if v == value {
					return nil
				}
				if utf8.RuneCountInString(v) < 1 || utf8.RuneCountInString(v) > 256 || controls(v) {
					return errors.New("use 1–256 characters without controls")
				}
				return nil
			}}
		}
	case "face.error":
		q = question{title: "Face generation failed; configuration was not written", action: true, choices: []string{"Retry", "Back", "Exit"}, value: "Retry"}
	case "face.sample":
		choices := []string{"Accept"}
		if strings.HasPrefix(s.faceMode, "random-") {
			choices = append(choices, "Reroll")
		}
		q = question{title: "Face sample", action: true, choices: choices, value: "Accept"}
	case "face.expression":
		states := face.States()
		sort.Strings(states)
		states = append([]string{"idle"}, without(states, "idle")...)
		q = question{title: "Face expression", choices: states, value: s.draft.Face.State}
	case "modules.ready":
		names := without(modules.Names(), "face")
		sort.Strings(names)
		q = question{title: "System modules (face: " + s.faceMode + "; Esc returns to Face setup)\nChanging selection, fields or order normalizes format literals to · separators.", label: "Modules", choices: names, checked: without(s.selected, "face"), multiple: true, validate: func(v string) error {
			if v == "" && !contains(s.selected, "face") {
				return errors.New("select at least one module, including face")
			}
			return nil
		}}
	case "fields.ready":
		name := s.queue[s.index]
		module, _ := modules.Get(name)
		choices := []string{"default text"}
		for _, f := range module.Fields {
			choices = append(choices, f.Name)
		}
		value := s.fields[name]
		if value == "" {
			value = "default text"
		}
		q = question{title: name + " field", choices: choices, value: value}
	case "order.ready":
		var items []reorder.Item
		var indices []string
		for i, name := range s.selected {
			label := name
			if f := s.fields[name]; f != "" {
				label += " (" + f + ")"
			}
			items = append(items, reorder.Item{ID: strconv.Itoa(i + 1), Label: label})
			indices = append(indices, strconv.Itoa(i+1))
		}
		q = question{title: "Module order", label: "Order", order: items, value: strings.Join(indices, " "), validate: func(v string) error { _, err := permutation(v, s.selected); return err }}
	case "color.ready":
		q = question{title: "Output color policy", choices: []string{"auto", "always", "never"}, value: s.draft.Color}
	case "styles.ready":
		q = question{title: "Module styles (inactive styles are preserved)", label: "Styles", choices: []string{"Keep styles", "Edit styles", "Edit foregrounds"}, value: "Keep styles"}
	case "style.field":
		name := s.selected[s.index]
		colors := append([]string{"terminal default"}, render.ColorNames()...)
		value := s.draft.Style[name].FG
		if value == "" {
			value = "terminal default"
		}
		q = question{title: name + " foreground", choices: colors, value: value}
	case "shell.ready":
		q = question{title: "Print shell startup instructions (no shell files are modified)", choices: []string{"none", "bash", "zsh", "fish"}, value: s.shell}
	case "review.ready":
		q = question{title: "Review configuration", label: "Review", action: true, choices: []string{"Save", "Edit face", "Edit modules", "Edit icons", "Edit styles", "Edit colors", "Edit shell", "Cancel"}, value: "Save"}
	case "confirm.ready":
		verb := "Create "
		if s.snap.exists {
			verb = "Replace "
		}
		q = question{title: verb + s.path + "?", label: "Confirm", action: true, choices: []string{"No", "Yes"}, value: "No"}
	}
	if q.title == "" {
		q = s.styleQuestion()
	}
	q.section = sectionName(s.state)
	q.key = s.state + ":" + strconv.Itoa(s.index)
	return q
}

// sectionName groups adjacent prompts into one stage of the setup flow.
func sectionName(state string) string {
	switch {
	case state == "destination.ready" || state == "preset.ready":
		return "Configuration"
	case strings.HasPrefix(state, "face."):
		return "Face"
	case state == "modules.ready" || state == "fields.ready" || state == "order.ready":
		return "Modules"
	case state == "icons.ready" || state == "icons.set" || state == "color.ready" || state == "styles.ready" || strings.HasPrefix(state, "style."):
		return "Appearance"
	case state == "shell.ready":
		return "Shell"
	default:
		return "Save"
	}
}

func (s *session) accept(ctx context.Context, value string) error {
	switch s.state {
	case "destination.ready":
		if value == "Cancel" {
			return errCancel
		}
		s.state = "preset.ready"
	case "preset.ready":
		if value != s.preset {
			if value != "existing" {
				if value == "default" {
					s.draft.Format = config.DefaultFormat
					s.draft.Color = "auto"
				} else {
					data, err := presets.Read(value + ".toml")
					if err != nil {
						return err
					}
					if value == "banner" {
						s.draft.Face.Seed = nil
						s.draft.Face.Random = false
					}
					if err = toml.Unmarshal(data, &s.draft); err != nil {
						return err
					}
				}
				s.derive()
			}
			s.preset = value
		}
		s.state = "face.mode"
	case "face.mode":
		s.faceMode = value
		if value == "disabled" {
			s.setFace(false)
			s.state = "modules.ready"
		} else {
			s.setFace(true)
			if strings.HasPrefix(value, "random-") {
				s.state = "face.sample"
				return s.roll(ctx)
			}
			s.state = "face.identity"
		}
	case "face.identity":
		s.draft.Face.Random = false
		if s.faceMode == "preset" {
			s.draft.Face.Preset = value
			s.draft.Face.Seed = nil
		} else {
			s.draft.Face.Preset = ""
			s.draft.Face.Seed = &value
		}
		s.state = "face.sample"
		return s.showFace()
	case "face.error":
		if value == "Exit" {
			return errAbort
		}
		if value == "Back" {
			s.state = "face.mode"
			return nil
		}
		s.state = "face.sample"
		if strings.HasPrefix(s.faceMode, "random-") {
			return s.roll(ctx)
		}
		return s.showFace()
	case "face.sample":
		if value == "Reroll" {
			return s.roll(ctx)
		}
		s.state = "face.expression"
	case "face.expression":
		s.draft.Face.State = value
		s.state = "modules.ready"
	case "modules.ready":
		chosen := []string{}
		if value != "" {
			chosen = strings.Split(value, ",")
		}
		next := []string{}
		for _, name := range s.selected {
			if name == "face" || contains(chosen, name) {
				next = append(next, name)
			}
		}
		for _, name := range chosen {
			if !contains(next, name) {
				next = append(next, name)
			}
		}
		if !slices.Equal(next, s.selected) {
			s.dirty = true
		}
		s.selected = next
		for name := range s.fields {
			if !contains(next, name) {
				delete(s.fields, name)
			}
		}
		s.queue = nil
		for _, name := range next {
			module, _ := modules.Get(name)
			if len(module.Fields) > 0 {
				s.queue = append(s.queue, name)
			}
		}
		s.index = 0
		s.state = "order.ready"
		if len(s.queue) > 0 {
			s.state = "fields.ready"
		}
	case "fields.ready":
		name := s.queue[s.index]
		if value == "default text" {
			value = ""
		}
		if value != s.fields[name] {
			s.dirty = true
		}
		s.fields[name] = value
		s.index++
		if s.index == len(s.queue) {
			s.state = "order.ready"
		}
	case "order.ready":
		next, err := permutation(value, s.selected)
		if err != nil {
			return err
		}
		if !slices.Equal(next, s.selected) {
			s.dirty = true
		}
		s.selected = next
		if s.dirty {
			s.normalize()
		}
		s.state = "icons.ready"
	case "color.ready":
		s.draft.Color = value
		s.state = "styles.ready"
	case "styles.ready":
		switch value {
		case "Edit styles":
			s.state = "style.module"
		case "Edit foregrounds":
			s.index = 0
			s.state = "style.field"
		default:
			s.state = "shell.ready"
		}
	case "style.field":
		if value == "terminal default" {
			value = ""
		}
		name := s.selected[s.index]
		style := s.draft.Style[name]
		style.FG = value
		s.draft.Style[name] = style
		s.index++
		if s.index == len(s.selected) {
			s.state = "shell.ready"
		}
	case "shell.ready":
		s.shell = value
		s.state = "review.ready"
	case "review.ready":
		switch value {
		case "Save":
			s.state = "confirm.ready"
		case "Edit face":
			s.state = "face.mode"
		case "Edit modules":
			s.state = "modules.ready"
		case "Edit icons":
			s.state = "icons.ready"
		case "Edit styles":
			s.state = "styles.ready"
		case "Edit colors":
			s.state = "color.ready"
		case "Edit shell":
			s.state = "shell.ready"
		case "Cancel":
			return errCancel
		}
	case "confirm.ready":
		if value == "Yes" {
			s.state = "save.pending"
		} else {
			s.state = "review.ready"
		}
	default:
		s.acceptStyle(value)
	}
	return nil
}
func (s *session) back() {
	switch s.state {
	case "preset.ready":
		s.state = "destination.ready"
	case "face.mode":
		s.state = "preset.ready"
	case "face.identity", "face.error":
		s.state = "face.mode"
	case "face.sample":
		s.state = "face.identity"
		if strings.HasPrefix(s.faceMode, "random-") {
			s.state = "face.mode"
		}
	case "face.expression":
		s.state = "face.sample"
	case "modules.ready":
		s.state = "face.mode"
		if s.faceMode != "disabled" {
			s.state = "face.expression"
		}
	case "fields.ready":
		if s.index > 0 {
			s.index--
		} else {
			s.state = "modules.ready"
		}
	case "order.ready":
		s.state = "modules.ready"
		if len(s.queue) > 0 {
			s.state = "fields.ready"
			s.index = len(s.queue) - 1
		}
	case "icons.ready":
		s.state = "order.ready"
	case "icons.set":
		s.state = "icons.ready"
	case "color.ready":
		s.state = "icons.set"
	case "styles.ready":
		s.state = "color.ready"
	case "style.field":
		if s.index > 0 {
			s.index--
		} else {
			s.state = "styles.ready"
		}
	case "style.module":
		s.state = "styles.ready"
	case "style.property":
		s.state = "style.module"
	case "style.color", "style.attributes":
		s.state = "style.property"
	case "style.custom":
		s.state = "style.color"
	case "shell.ready":
		s.state = "styles.ready"
	case "review.ready":
		s.state = "shell.ready"
	case "confirm.ready":
		s.state = "review.ready"
	}
}
func (s *session) roll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var seed string
	err := s.term.work(ctx, "Generating face", func() error {
		var err error
		if s.term.runtime.Entropy == nil {
			return errors.New("face entropy source unavailable")
		}
		seed, err = s.term.runtime.Entropy(ctx)
		return err
	})
	if err != nil {
		s.state = "face.error"
		return fmt.Errorf("cannot generate face: %w", err)
	}
	s.sampleSeed = seed
	s.draft.Face.Preset = ""
	s.draft.Face.Seed = nil
	s.draft.Face.Random = s.faceMode == "random-each-run"
	if s.faceMode == "random-once" {
		s.draft.Face.Seed = &seed
	}
	return s.showFace()
}
func (s *session) showFace() error {
	opts := s.draft.Face
	if opts.Random {
		opts.Random = false
		seed := s.sampleSeed
		opts.Seed = &seed
	}
	animation, err := face.Build(opts)
	if err != nil {
		s.state = "face.error"
		return err
	}
	label := "Sample: "
	if s.faceMode == "random-each-run" {
		label = "Sample — changes next run: "
	}
	return s.term.say(label + animation.Still)
}
func (s *session) setFace(enabled bool) {
	present := contains(s.selected, "face")
	if present == enabled {
		return
	}
	toks, _ := render.Parse(s.draft.Format)
	if enabled {
		if s.draft.Format != "" {
			s.draft.Format += " · "
		}
		s.draft.Format += "{face}"
		s.selected = append(s.selected, "face")
		return
	}
	var out strings.Builder
	for _, tok := range toks {
		if tok.Module == "face" {
			continue
		}
		out.WriteString(tokenText(tok))
	}
	s.draft.Format = out.String()
	s.selected = without(s.selected, "face")
}
func tokenText(tok render.Token) string {
	if tok.Module == "" {
		return strings.NewReplacer("{", "{{", "}", "}}").Replace(tok.Literal)
	}
	field := ""
	if tok.Field != "" {
		field = ":" + tok.Field
	}
	return "{" + tok.Module + field + "}"
}
func (s *session) normalize() {
	var tokens []string
	for _, name := range s.selected {
		tokens = append(tokens, tokenText(render.Token{Module: name, Field: s.fields[name]}))
	}
	s.draft.Format = strings.Join(tokens, " · ")
}
func without(values []string, value string) []string {
	var out []string
	for _, v := range values {
		if v != value {
			out = append(out, v)
		}
	}
	return out
}
func validConfig(path string, data []byte) bool {
	_, err := config.Parse(path, data)
	return err == nil
}
func permutation(value string, selected []string) ([]string, error) {
	parts := strings.Fields(strings.ReplaceAll(value, ",", " "))
	var result []string
	if len(parts) != len(selected) {
		return nil, errors.New("list every enabled module number once")
	}
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(selected) || contains(result, selected[n-1]) {
			return nil, errors.New("list every enabled module number once")
		}
		result = append(result, selected[n-1])
	}
	return result, nil
}
func detectShell(value string) string {
	value = filepath.Base(value)
	if value == "bash" || value == "zsh" || value == "fish" {
		return value
	}
	return "none"
}

func (s *session) refresh(ctx context.Context) error {
	for {
		action, err := s.term.ask(ctx, question{title: "Destination changed", choices: []string{"Refresh", "Exit"}, value: "Refresh"})
		if err != nil {
			return err
		}
		if action == "Exit" {
			return errors.New("setup exited after destination conflict")
		}
		snap, err := inspectTarget(s.path)
		if err == nil {
			s.snap = snap
			return nil
		}
		if err = s.term.say("Cannot refresh destination: " + err.Error()); err != nil {
			return err
		}
	}
}
