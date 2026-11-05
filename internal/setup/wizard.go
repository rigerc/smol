package setup

import (
	"context"
	"errors"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/ishan5ain/tuiweave/inspect"
	"github.com/ishan5ain/tuiweave/layout"
	"github.com/muesli/cancelreader"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/render"
)

// wizardBridge serializes the existing save/navigation workflow with one renderer.
// Only the workflow owns session state; the UI receives immutable question snapshots.
type wizardBridge struct {
	ctx      context.Context
	requests chan wizardRequest
}

type wizardRequest struct {
	text   string
	status string
	q      *question
	reply  chan answer
	ack    chan struct{}
}

type wizardWriter struct{ bridge *wizardBridge }

func (w wizardWriter) Write(p []byte) (int, error) {
	ack := make(chan struct{})
	select {
	case w.bridge.requests <- wizardRequest{text: string(p), ack: ack}:
	case <-w.bridge.ctx.Done():
		return 0, w.bridge.ctx.Err()
	}
	select {
	case <-ack:
		return len(p), nil
	case <-w.bridge.ctx.Done():
		return 0, w.bridge.ctx.Err()
	}
}

func (b *wizardBridge) next() tea.Cmd {
	return func() tea.Msg {
		select {
		case req := <-b.requests:
			return req
		case <-b.ctx.Done():
			return wizardClosed{}
		}
	}
}

type wizardClosed struct{}
type wizardDone struct{ code int }
type previewFetched struct {
	generation int
	toks       []render.Token
	values     map[string]modules.Result
}

type wizardModel struct {
	bridge    *wizardBridge
	run       tea.Cmd
	cancel    context.CancelFunc
	runtime   Runtime
	env       modules.Env
	prompt    promptModel
	reply     chan answer
	status    string
	preview   string
	candidate config.Config
	cache     previewCache
	pending   map[string]bool
	fallback  bool
	done      bool
}

func (m wizardModel) Init() tea.Cmd { return tea.Batch(m.run, m.bridge.next()) }
func (m *wizardModel) size() {
	rows := max(1, m.runtime.Rows-4)
	if m.runtime.Rows == 0 {
		rows = 0
	}
	layout.Vertical(layout.Len(rows)).Apply(layout.NewRect(0, 0, m.runtime.Width, rows), &m.prompt)
}
func (m *wizardModel) refreshPreview() tea.Cmd {
	if m.prompt.q.preview == nil {
		return nil
	}
	value := m.prompt.currentValue()
	if m.prompt.q.validate != nil && m.prompt.q.validate(value) != nil {
		return nil
	}
	cfg, err := m.prompt.q.preview(value)
	if err != nil {
		return nil
	}
	if m.cache.configure(cfg) {
		m.pending = make(map[string]bool)
	}
	m.candidate = cfg
	m.preview = m.cache.line(cfg, m.runtime)
	var missing []render.Token
	for _, tok := range m.cache.missing(cfg) {
		if !m.pending[tok.Module] {
			missing = append(missing, tok)
			m.pending[tok.Module] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}
	ctx, env := m.bridge.ctx, m.env
	generation := m.cache.generation
	return func() tea.Msg {
		return previewFetched{generation: generation, toks: missing, values: fetchPreview(ctx, cfg, env, missing)}
	}
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case wizardDone:
		m.done = true
		if m.bridge.ctx.Err() != nil {
			message := "Cancelled. Configuration was not saved."
			if msg.code == 143 {
				message = "Terminated before saving."
			}
			if msg.code == 130 || msg.code == 143 {
				return m, tea.Sequence(tea.Println(message), tea.Quit)
			}
		}
		return m, tea.Quit
	case wizardClosed:
		return m, nil // The workflow finishes cleanup/publication before wizardDone.
	case wizardRequest:
		if msg.text != "" {
			return m, tea.Sequence(tea.Println(strings.TrimSuffix(msg.text, "\n")), func() tea.Msg {
				if msg.ack != nil {
					close(msg.ack)
				}
				return nil
			}, m.bridge.next())
		}
		if msg.status != "" {
			m.status = msg.status
			m.reply = nil
			return m, m.bridge.next()
		}
		if msg.q != nil {
			m.prompt = newPrompt(*msg.q, m.runtime.Width)
			m.reply = msg.reply
			m.status = ""
			m.size()
			cmd := m.refreshPreview()
			if m.prompt.needsBasic() {
				m.fallback = true
				return m, tea.Quit
			}
			return m, cmd
		}
	case previewFetched:
		if msg.generation != m.cache.generation {
			return m, nil
		}
		m.cache.remember(msg.toks, msg.values)
		for _, tok := range msg.toks {
			delete(m.pending, tok.Module)
		}
		if m.candidate.Format != "" {
			m.preview = m.cache.line(m.candidate, m.runtime)
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.runtime.Width = msg.Width
		m.runtime.Rows = msg.Height
		m.size()
		if m.reply != nil && m.prompt.needsBasic() {
			m.fallback = true
			return m, tea.Quit
		}
		return m, nil
	case inputEnded:
		m.cancel()
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || msg.String() == "ctrl+d" {
			m.cancel()
			return m, nil
		}
	}
	if m.reply == nil || m.bridge.ctx.Err() != nil {
		return m, nil
	}
	next, cmd := m.prompt.Update(msg)
	// SAFETY: promptModel.Update always returns a promptModel, including exit paths.
	m.prompt = next.(promptModel)
	if m.prompt.finished {
		if errors.Is(m.prompt.result.err, errBasic) {
			m.fallback = true
			return m, tea.Quit
		}
		m.reply <- m.prompt.result
		m.reply = nil
		m.status = "Updating…"
		return m, m.bridge.next()
	}
	return m, tea.Batch(cmd, m.refreshPreview())
}

func previewRows(text string, width int) string {
	lines := strings.Split(ansi.Hardwrap(text, max(1, width), true), "\n")
	if len(lines) <= 2 {
		return strings.Join(lines, "\n")
	}
	clipped := lines[0] + "\n" + ansi.Truncate(lines[1], max(1, width-1), "") + "…"
	if strings.Contains(text, "\x1b[") {
		clipped += "\x1b[0m"
	}
	return clipped
}
func (m wizardModel) View() tea.View {
	if m.done || m.fallback {
		return tea.NewView("")
	}
	body := m.status
	if m.reply != nil {
		body = m.prompt.View().Content
	}
	if m.runtime.Getenv("NO_COLOR") != "" || m.runtime.Getenv("TERM") == "dumb" {
		body = ansi.Strip(body)
	}
	return tea.NewView(header(m.runtime) + "\n" + previewRows(m.preview, m.runtime.Width) + "\n" + body)
}
func (m wizardModel) Inspect() inspect.Node {
	preview := previewRows(m.preview, m.runtime.Width)
	offset := 1 + len(strings.Split(preview, "\n"))
	children := []inspect.Node{
		{ID: "header", Kind: "text", Label: "smol · setup", Bounds: inspect.Bounds{Width: m.runtime.Width, Height: 1}},
		{ID: "preview", Kind: "output", Label: ansi.Strip(preview), Bounds: inspect.Bounds{Y: 1, Width: m.runtime.Width, Height: offset - 1}},
	}
	if m.reply != nil {
		question := m.prompt.Inspect()
		question.ID = "question"
		question.Bounds.Y += offset
		for i := range question.Children {
			question.Children[i].Bounds.Y += offset
		}
		children = append(children, question)
	}
	return inspect.Group("wizard", "wizard", inspect.Bounds{Width: m.runtime.Width, Height: len(strings.Split(m.View().Content, "\n"))}, children...)
}

func runWizard(ctx context.Context, s *session, stdout io.Writer) int {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	t := s.term
	out := t.out
	bridge := &wizardBridge{ctx: runCtx, requests: make(chan wizardRequest)}
	t.bridge = bridge
	t.out = wizardWriter{bridge: bridge}
	finished := make(chan struct{})
	code := 1
	go func() {
		defer close(finished)
		if err := s.load(runCtx); err != nil {
			code = finishError(runCtx, t, err)
		} else {
			code = s.run(runCtx, stdout)
		}
	}()
	run := func() tea.Msg { <-finished; return wizardDone{code: code} }

	model := wizardModel{bridge: bridge, run: run, cancel: cancel, runtime: t.runtime, env: s.env, preview: "Preview: preparing…", pending: make(map[string]bool)}
	opts := []tea.ProgramOption{tea.WithInput(t.runtime.Input), tea.WithOutput(out), tea.WithWindowSize(t.runtime.Width, t.runtime.Rows), tea.WithColorProfile(colorprofile.TrueColor), tea.WithoutSignalHandler(), tea.WithoutCatchPanics()}
	var program *tea.Program
	ended := func() { program.Send(inputEnded{}) }
	if file, ok := t.runtime.Input.(cancelreader.File); ok {
		opts = append(opts, tea.WithInput(eofFile{File: file, ended: ended}))
	} else {
		opts = append(opts, tea.WithInput(eofReader{Reader: t.runtime.Input, ended: ended}))
	}
	program = tea.NewProgram(model, opts...)
	result, err := program.Run()
	final, ok := result.(wizardModel)
	if !ok {
		cancel()
		<-finished
		return 1
	}
	if final.fallback && err == nil {
		// The worker is blocked on this reply. Transfer ownership only after raw input
		// and the inline renderer have stopped, then release it into numbered input.
		t.bridge = nil
		t.out = out
		t.runtime.Basic = true
		t.runtime.Width = final.runtime.Width
		t.runtime.Rows = final.runtime.Rows
		t.preview = final.cache
		final.reply <- answer{err: errBasic, q: final.prompt.q, value: final.prompt.currentValue()}
	} else {
		cancel()
	}
	<-finished
	if err != nil {
		return 1
	}
	return code
}
