package setup

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/inspect"
	"github.com/ishan5ain/tuiweave/layout"
	"github.com/ishan5ain/tuiweave/menu"
	"github.com/ishan5ain/tuiweave/textarea"
	"github.com/muesli/cancelreader"
)

var errBasic = errors.New("use numbered input")

type inputEnded struct{}
type eofReader struct {
	io.Reader
	ended func()
}

func (r eofReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if errors.Is(err, io.EOF) {
		r.ended()
	}
	return n, err
}

type eofFile struct {
	cancelreader.File
	ended func()
}

func (r eofFile) Read(p []byte) (int, error) {
	n, err := r.File.Read(p)
	if errors.Is(err, io.EOF) {
		r.ended()
	}
	return n, err
}

var errBack = errors.New("back")
var errCancel = errors.New("cancelled")
var errAbort = errors.New("setup exited")

type question struct {
	title    string
	choices  []string
	value    string
	checked  []string
	multiple bool
	seed     bool
	validate func(string) error
}
type answer struct {
	value string
	err   error
}
type promptModel struct {
	q        question
	menu     menu.Model
	editor   textarea.Model
	width    int
	height   int
	result   answer
	finished bool
	feedback string
}

func newPrompt(q question, width int) promptModel {
	theme := tuiweave.CatppuccinMocha()
	choices := theme
	choices.SelectionBg = lipgloss.NoColor{}
	choices.SelectionFg = theme.Accent
	m := promptModel{q: q, menu: menu.New(choices), editor: textarea.New(theme), width: width}
	m.editor.SetValue(q.value)
	if len(q.choices) > 0 {
		m.rebuild()
		m.menu.SelectID(q.value)
		m.menu.Focus()
	} else {
		m.editor.Focus()
	}
	m.size()
	return m
}
func (m *promptModel) rebuild() {
	items := make([]menu.Item, 0, len(m.q.choices))
	for _, choice := range m.q.choices {
		label := choice
		if m.q.multiple {
			mark := "[ ] "
			if contains(m.q.checked, choice) {
				mark = "[x] "
			}
			label = mark + choice
		}
		items = append(items, menu.Item{ID: choice, Label: label})
	}
	m.menu.SetItems(items...)
}
func (m *promptModel) size() {
	if m.width <= 0 {
		m.width = 80
	}
	rows := len(m.q.choices)
	if rows == 0 {
		rows = max(1, m.editor.ContentHeight())
	}
	layout.Vertical(layout.Len(rows)).Apply(layout.NewRect(0, 0, m.width, rows), &m.menu)
	m.editor.SetSize(m.width, max(1, m.editor.ContentHeight()))
	// Recompute after setting width, since wrapping changes content height.
	m.editor.SetSize(m.width, max(1, m.editor.ContentHeight()))
}
func (m promptModel) Init() tea.Cmd { return nil }
func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case inspect.ActionMsg:
		tree := m.Inspect()
		for _, child := range tree.Children {
			for _, action := range child.Actions {
				if action.ID != msg.ID || !action.Enabled {
					continue
				}
				local := strings.TrimPrefix(msg.ID, child.ID+".")
				if local == "activate" {
					return m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				}
				var cmd tea.Cmd
				if child.ID == "choices" {
					m.menu, cmd = m.menu.Update(inspect.Invoke(local))
				} else {
					m.editor, cmd = m.editor.Update(inspect.Invoke(local))
					m.size()
				}
				return m, cmd
			}
		}
		return m, nil
	case inputEnded:
		m.result.err = errCancel
		m.finished = true
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.size()
		if m.needsBasic() {
			m.result.err = errBasic
			m.finished = true
			return m, tea.Quit
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			m.result.err = errCancel
			m.finished = true
			return m, tea.Quit
		case "esc":
			m.result.err = errBack
			m.finished = true
			return m, tea.Quit
		case "enter":
			value := m.editor.Value()
			if len(m.q.choices) > 0 {
				value = m.menu.SelectedID()
			}
			if m.q.multiple {
				value = strings.Join(m.q.checked, ",")
			}
			if m.q.validate != nil {
				if err := m.q.validate(value); err != nil {
					m.feedback = err.Error()
					return m, nil
				}
			}
			m.result.value = value
			m.finished = true
			return m, tea.Quit
		case "space":
			if m.q.multiple {
				m.q.checked = toggle(m.q.checked, m.menu.SelectedID())
				m.rebuild()
				return m, nil
			}
		}
	case tea.PasteMsg:
		if controls(msg.Content) {
			m.feedback = "Control characters and newlines are not allowed."
			return m, nil
		}
	}
	var cmd tea.Cmd
	if len(m.q.choices) > 0 {
		m.menu, cmd = m.menu.Update(msg)
	} else {
		m.editor, cmd = m.editor.Update(msg)
		m.size()
	}
	if m.needsBasic() {
		m.result.err = errBasic
		m.finished = true
		return m, tea.Quit
	}
	return m, cmd
}
func (m promptModel) needsBasic() bool {
	return m.width < 40 || (m.height > 0 && len(strings.Split(m.View().Content, "\n")) >= m.height)
}
func (m promptModel) View() tea.View {
	if m.finished {
		return tea.NewView("")
	}
	content := m.menu.View()
	hints := "up/down choose; Enter accepts; Esc back; Ctrl+C cancels"
	if m.q.multiple {
		hints = "Space toggles; Enter continues; Esc back; Ctrl+C cancels"
	}
	if len(m.q.choices) == 0 {
		content = m.editor.View()
		hints = "Enter accepts; Esc back; Ctrl+C cancels"
	}
	return tea.NewView(ansi.Wrap(safe(m.q.title), m.width, "") + "\n" + content + "\n" + ansi.Wrap(safe(m.feedback), m.width, "") + "\n" + ansi.Wrap(hints, m.width, ""))
}
func (m promptModel) Inspect() inspect.Node {
	bounds := inspect.Bounds{Width: m.width, Height: len(strings.Split(m.View().Content, "\n"))}
	if len(m.q.choices) > 0 {
		return inspect.Group("prompt", "prompt", bounds, inspect.BindAt("choices", inspect.Bounds{Y: len(strings.Split(ansi.Wrap(safe(m.q.title), m.width, ""), "\n")), Width: m.width, Height: len(m.q.choices)}, m.menu))
	}
	return inspect.Group("prompt", "prompt", bounds, inspect.BindAt("editor", inspect.Bounds{Y: len(strings.Split(ansi.Wrap(safe(m.q.title), m.width, ""), "\n")), Width: m.width, Height: m.editor.ContentHeight()}, m.editor))
}

type terminal struct {
	runtime Runtime
	out     io.Writer
	lines   *bufio.Reader
	cancel  cancelreader.CancelReader
}

func (t *terminal) close() error {
	if t.cancel != nil {
		return t.cancel.Close()
	}
	return nil
}
func (t *terminal) say(text string) error {
	width := t.runtime.Width
	if width <= 0 {
		width = 80
	}
	_, err := fmt.Fprintln(t.out, ansi.Wrap(safe(text), width, ""))
	return err
}
func (t *terminal) ask(ctx context.Context, q question) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	basic := t.runtime.Basic || t.runtime.Width > 0 && t.runtime.Width < 40
	// Large menus remain append-only instead of overflowing the inline renderer.
	if t.runtime.Rows > 0 && len(strings.Split(newPrompt(q, t.runtime.Width).View().Content, "\n")) >= t.runtime.Rows {
		basic = true
	}
	if basic {
		t.runtime.Basic = true
		return t.basic(ctx, q)
	}
	opts := []tea.ProgramOption{tea.WithInput(t.runtime.Input), tea.WithOutput(t.out), tea.WithContext(ctx), tea.WithWindowSize(t.runtime.Width, t.runtime.Rows), tea.WithoutSignalHandler(), tea.WithoutCatchPanics()}
	if t.runtime.Getenv("NO_COLOR") != "" || t.runtime.Getenv("TERM") == "dumb" {
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}
	var program *tea.Program
	ended := func() { program.Send(inputEnded{}) }
	if file, ok := t.runtime.Input.(cancelreader.File); ok {
		opts = append(opts, tea.WithInput(eofFile{File: file, ended: ended}))
	} else {
		opts = append(opts, tea.WithInput(eofReader{Reader: t.runtime.Input, ended: ended}))
	}
	// The callback is used only after Run starts reading the configured input.
	program = tea.NewProgram(newPrompt(q, t.runtime.Width), opts...)
	model, err := program.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	result, ok := model.(promptModel)
	if !ok {
		return "", errors.New("unexpected terminal result")
	}
	if errors.Is(result.result.err, errBasic) {
		t.runtime.Basic = true
		t.runtime.Width = result.width
		t.runtime.Rows = result.height
		q.checked = result.q.checked
		if len(q.choices) == 0 {
			q.value = result.editor.Value()
		}
		return t.basic(ctx, q)
	}
	if result.result.err != nil {
		return "", result.result.err
	}
	if err = t.say(q.title + ": " + result.result.value); err != nil {
		return "", err
	}
	return result.result.value, nil
}
func (t *terminal) basic(ctx context.Context, q question) (string, error) {
	if t.lines == nil {
		r, err := cancelreader.NewReader(t.runtime.Input)
		if err != nil {
			return "", err
		}
		t.cancel = r
		t.lines = bufio.NewReader(r)
	}
	stop := context.AfterFunc(ctx, func() { t.cancel.Cancel() })
	defer stop()
	if q.multiple {
		q.title += "\nUse comma-separated numbers, or none for no system modules."
	}
	if err := t.say(strings.NewReplacer("–", "-", "—", "-", "…", "...").Replace(q.title)); err != nil {
		return "", err
	}
	for i, choice := range q.choices {
		if err := t.say(fmt.Sprintf("%d. %s", i+1, choice)); err != nil {
			return "", err
		}
	}
	for {
		def := q.value
		if q.multiple {
			var indices []string
			for i, choice := range q.choices {
				if contains(q.checked, choice) {
					indices = append(indices, strconv.Itoa(i+1))
				}
			}
			def = strings.Join(indices, ",")
		} else {
			for i, choice := range q.choices {
				if choice == q.value {
					def = strconv.Itoa(i + 1)
				}
			}
		}
		if err := t.say("[" + def + "] Enter accepts; :back / :cancel at choices"); err != nil {
			return "", err
		}
		line, err := t.lines.ReadString('\n')
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, cancelreader.ErrCanceled) {
				return "", errCancel
			}
			return "", err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if len(q.choices) > 0 || !q.seed {
			if line == ":back" {
				return "", errBack
			}
			if line == ":cancel" {
				return "", errCancel
			}
		}
		value := line
		if len(q.choices) > 0 {
			if value == "" {
				value = def
			}
			var chosen []string
			valid := true
			for _, token := range strings.Split(value, ",") {
				n, e := strconv.Atoi(strings.TrimSpace(token))
				if e != nil || n < 1 || n > len(q.choices) || contains(chosen, q.choices[n-1]) {
					valid = false
					break
				}
				chosen = append(chosen, q.choices[n-1])
			}
			if q.multiple && (line == "none" || value == "") {
				chosen = nil
				valid = true
			}
			if !valid || (!q.multiple && len(chosen) != 1) {
				if err = t.say("Choose listed numbers without duplicates."); err != nil {
					return "", err
				}
				continue
			}
			value = strings.Join(chosen, ",")
		} else if value == "" {
			value = q.value
		}
		if q.validate != nil {
			if validationErr := q.validate(value); validationErr != nil {
				if err = t.say(validationErr.Error()); err != nil {
					return "", err
				}
				continue
			}
		}
		return value, nil
	}
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func toggle(values []string, value string) []string {
	result := append([]string(nil), values...)
	for i, v := range result {
		if v == value {
			return append(result[:i], result[i+1:]...)
		}
	}
	return append(result, value)
}
