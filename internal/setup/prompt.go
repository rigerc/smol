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
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/focus"
	"github.com/ishan5ain/tuiweave/inspect"
	"github.com/ishan5ain/tuiweave/menu"
	"github.com/ishan5ain/tuiweave/multiselect"
	"github.com/ishan5ain/tuiweave/reorder"
	"github.com/ishan5ain/tuiweave/textinput"
	"github.com/ishan5ain/tuiweave/viewport"
	"github.com/muesli/cancelreader"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/modules"
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
	preview     func(string) (config.Config, error)
	title       string
	description string
	label       string
	section     string
	key         string
	order       []reorder.Item
	choices     []string
	value       string
	checked     []string
	multiple    bool
	seed        bool
	action      bool
	validate    func(string) error
}
type answer struct {
	q     question
	value string
	err   error
}
type promptModel struct {
	q        question
	menu     menu.Model
	multi    multiselect.Model
	order    reorder.Model
	editor   textinput.Model
	focus    focus.Manager
	width    int
	height   int
	result   answer
	finished bool
	feedback string
}

func newPrompt(q question, width int) promptModel {
	theme := tuiweave.CatppuccinMocha()
	m := promptModel{q: q, menu: menu.NewOn(theme, nil), editor: textinput.New(theme), multi: multiselect.New(theme), order: reorder.New(theme), focus: focus.NewManager(1), width: width}
	m.menu.Inline = true
	m.editor.Inline = true
	m.multi.Inline = true
	m.order.Inline = true
	if q.order != nil {
		items := make([]reorder.Item, 0, len(q.order))
		for _, id := range strings.Fields(q.value) {
			for _, item := range q.order {
				if item.ID == id {
					items = append(items, item)
					break
				}
			}
		}
		m.order.SetItems(items...)
	}
	m.editor.SetValue(q.value)
	if len(q.choices) > 0 {
		m.rebuild()
		m.menu.SelectID(q.value)
	}
	m.focus.Apply(m.focusable())
	m.size()
	return m
}
func (m promptModel) currentValue() string {
	if m.q.order != nil {
		return strings.Join(m.order.IDs(), " ")
	}
	if m.q.multiple {
		return strings.Join(m.multi.SelectedIDs(), ",")
	}
	if len(m.q.choices) > 0 {
		return m.menu.SelectedID()
	}
	return m.editor.Value()
}
func (m *promptModel) rebuild() {
	items := make([]menu.Item, 0, len(m.q.choices))
	for _, choice := range m.q.choices {
		items = append(items, menu.Item{ID: choice, Label: choice})
	}
	m.menu.SetItems(items...)
	m.multi.SetItems(items...)
	m.multi.SetCheckedIDs(m.q.checked...)
}

// SetSize lets the session owner allocate space below its persistent header.
func (m *promptModel) SetSize(width, height int) { m.width = width; m.height = height; m.size() }
func (m *promptModel) size() {
	if m.width <= 0 {
		m.width = 80
	}
	title := m.q.displayTitle()
	// Keep long questions visible in full above the component's clipped header.
	if ansi.StringWidth(title) > m.width-3 || strings.Contains(title, "\n") {
		title = ""
	}
	m.menu.Title = title
	m.editor.Title = title
	m.multi.Title = title
	m.order.Title = title
	m.menu.Description = safe(m.q.description)
	m.editor.Description = safe(m.q.description)
	m.multi.Description = safe(m.q.description)
	m.order.Description = safe(m.q.description)
	m.menu.SetSize(m.width, m.menu.NaturalHeight())
	m.editor.SetSize(m.width, m.editor.NaturalHeight())
	m.multi.SetSize(m.width, m.multi.NaturalHeight())
	m.order.SetSize(m.width, m.order.NaturalHeight())
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
				if local == "activate" || child.ID == "modules" && local == multiselect.ActionSubmit {
					return m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				}
				var cmd tea.Cmd
				switch child.ID {
				case "order":
					m.order, cmd = m.order.Update(inspect.Invoke(local))
				case "modules":
					m.multi, cmd = m.multi.Update(inspect.Invoke(local))
					m.q.checked = m.multi.SelectedIDs()
				case "choices":
					m.menu, cmd = m.menu.Update(inspect.Invoke(local))
				default:
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
			value := m.currentValue()
			if m.q.validate != nil {
				if err := m.q.validate(value); err != nil {
					m.feedback = err.Error()
					return m, nil
				}
			}
			m.result.value = value
			m.finished = true
			return m, tea.Quit
		}
	case tea.PasteMsg:
		if controls(msg.Content) {
			m.feedback = "Control characters and newlines are not allowed."
			return m, nil
		}
	}
	var cmd tea.Cmd
	switch {
	case m.q.order != nil:
		m.order, cmd = m.order.Update(msg)
	case m.q.multiple:
		m.multi, cmd = m.multi.Update(msg)
		m.q.checked = m.multi.SelectedIDs()
	case len(m.q.choices) > 0:
		m.menu, cmd = m.menu.Update(msg)
	default:
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
	return tea.NewView("\n" + m.titlePrefix() + m.fieldView() + m.footer())
}

// footer adds workflow navigation to the component's own interaction hints.
func (m promptModel) footer() string {
	text := "\n"
	if m.feedback != "" {
		text += ansi.Wrap(safe(m.feedback), m.width, "") + "\n"
	}
	return text + ansi.Wrap("Enter accepts; Esc back; Ctrl+C cancels", m.width, "") + "\n"
}

// fieldWindow uses a tuiweave viewport for long inline lists. Component
// headers and interaction hints stay visible while the cursor scrolls.
func (m promptModel) fieldWindow() (string, *inspect.Scroll) {
	var content string
	var selected, count int
	switch {
	case m.q.order != nil:
		content, selected, count = m.order.View(), m.order.Selected(), m.order.Len()
	case m.q.multiple:
		content, selected, count = m.multi.View(), m.multi.Selected(), len(m.q.choices)
	case len(m.q.choices) > 0:
		content, selected, count = m.menu.View(), m.menu.Selected(), len(m.q.choices)
	default:
		return m.editor.View(), nil
	}
	if m.height <= 0 || count == 0 {
		return content, nil
	}
	rows := strings.Split(content, "\n")
	header := 1
	if m.q.description != "" {
		header++
	}
	// Leave one row unused so the inline renderer never scrolls the screen.
	budget := m.height - 2 - strings.Count(m.titlePrefix(), "\n") - strings.Count(m.footer(), "\n")
	visible := min(count, max(1, budget-header-2))
	if len(rows) <= budget || len(rows) < header+count+2 {
		return content, nil
	}
	offset := min(max(0, selected-visible/2), count-visible)
	window := viewport.New(tuiweave.CatppuccinMocha())
	window.SetContent(strings.Join(rows[header:header+count], "\n"))
	window.SetSize(m.width, visible)
	window.ScrollTo(offset)
	parts := append([]string(nil), rows[:header]...)
	parts = append(parts, window.View())
	parts = append(parts, rows[header+count:]...)
	return strings.Join(parts, "\n"), &inspect.Scroll{Total: count, Visible: visible, Offset: offset}
}

func (m promptModel) fieldView() string {
	content, _ := m.fieldWindow()
	return content
}

// displayTitle groups the field under its section without a separate header.
func (q question) displayTitle() string {
	title := safe(q.title)
	if q.section != "" && !strings.HasPrefix(strings.ToLower(title), strings.ToLower(q.section)+" ") {
		return q.section + " · " + title
	}
	return title
}

// shortLabel names a field in the collapsed form history.
func (q question) shortLabel() string {
	if q.label != "" {
		return safe(q.label)
	}
	title := strings.SplitN(safe(q.title), "\n", 2)[0]
	if i := strings.Index(title, " ("); i > 0 {
		title = title[:i]
	}
	return title
}

// titlePrefix preserves questions too long for a single inline header row.
func (m promptModel) titlePrefix() string {
	if m.menu.Title != "" {
		return ""
	}
	return ansi.Wrap(m.q.displayTitle(), m.width, "") + "\n\n"
}

func (m *promptModel) focusable() focus.Focusable {
	if m.q.order != nil {
		return &m.order
	}
	if m.q.multiple {
		return &m.multi
	}
	if len(m.q.choices) > 0 {
		return &m.menu
	}
	return &m.editor
}
func (m promptModel) Inspect() inspect.Node {
	bounds := inspect.Bounds{Width: m.width, Height: len(strings.Split(m.View().Content, "\n"))}
	y := 1 + strings.Count(m.titlePrefix(), "\n")
	var id string
	var component inspect.Inspectable
	switch {
	case m.q.order != nil:
		id, component = "order", m.order
	case m.q.multiple:
		id, component = "modules", m.multi
	case len(m.q.choices) > 0:
		id, component = "choices", m.menu
	default:
		id, component = "editor", m.editor
	}
	content, scroll := m.fieldWindow()
	node := inspect.BindAt(id, inspect.Bounds{Y: y, Width: m.width, Height: len(strings.Split(content, "\n"))}, component)
	if scroll != nil {
		node.Scroll = scroll
	}
	return inspect.Group("prompt", "prompt", bounds, node)
}

type terminal struct {
	bridge  *wizardBridge
	env     modules.Env
	preview previewCache
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
	if t.bridge != nil {
		reply := make(chan answer, 1)
		select {
		case t.bridge.requests <- wizardRequest{q: &q, reply: reply}:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		select {
		case result := <-reply:
			if errors.Is(result.err, errBasic) {
				q = result.q
				q.value = result.value
				return t.basic(ctx, q)
			}
			return result.value, result.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
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
		q.value = result.currentValue()
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
	if q.preview != nil {
		cfg, err := q.preview(q.value)
		if q.multiple {
			cfg, err = q.preview(strings.Join(q.checked, ","))
		}
		if err == nil {
			t.preview.configure(cfg)
			missing := t.preview.missing(cfg)
			t.preview.remember(missing, fetchPreview(ctx, cfg, t.env, missing))
			if _, err = fmt.Fprintln(t.out, "smol · setup\n\n"+previewPanel(t.preview.line(cfg, t.runtime), t.runtime)+"\n"); err != nil {
				return "", err
			}
		}
	}
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
	if q.order != nil {
		q.title += ": enter every number once"
		for _, item := range q.order {
			q.title += "\n" + item.ID + ". " + item.Label
		}
	}
	if q.multiple {
		q.title += "\nUse comma-separated numbers, or none for no system modules."
	}
	if err := t.say("\n" + strings.NewReplacer("–", "-", "—", "-", "…", "...").Replace(q.title) + "\n"); err != nil {
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
		if err := t.say("\n[" + def + "] Enter accepts; :back / :cancel at choices"); err != nil {
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
