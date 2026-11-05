package setup

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ishan5ain/tuiweave/inspect"
	"github.com/ishan5ain/tuiweave/snaptest"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/render"
)

func TestTentativeDraftIsolation(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.draft.Modules.Disk.Mount = "/srv"
	candidate, err := s.previewDraft()("server")
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Modules.Disk.Mount != "/" || s.draft.Modules.Disk.Mount != "/srv" || s.preset != "default" {
		t.Fatal("preset preview changed accepted state")
	}
	s.state = "style.custom"
	s.styleProperty = "Background"
	s.index = 0
	name := s.selected[0]
	candidate, err = s.previewDraft()("#112233")
	if err != nil || candidate.Style[name].BG != "#112233" || s.draft.Style[name].BG != "" {
		t.Fatal("style preview mutated accepted draft", err)
	}
	s.state = "modules.ready"
	candidate, err = s.previewDraft()("mem")
	if err != nil || candidate.Format != "{mem}" || s.draft.Format != config.DefaultFormat {
		t.Fatal(candidate.Format, err)
	}
	s.state = "face.identity"
	s.faceMode = "seed"
	candidate, err = s.previewDraft()("tentative")
	if err != nil || candidate.Face.Seed == nil || *candidate.Face.Seed != "tentative" || s.draft.Face.Seed != nil {
		t.Fatal("seed preview", err)
	}
	s.state = "face.expression"
	candidate, err = s.previewDraft()("happy")
	if err != nil || candidate.Face.State != "happy" || s.draft.Face.State != "idle" {
		t.Fatal("expression preview", err)
	}
	s.draft.Face.Random = true
	s.sampleSeed = "stable"
	s.state = "shell.ready"
	a, _ := s.previewDraft()("none")
	b, _ := s.previewDraft()("none")
	if a.Face.Random || *a.Face.Seed != "stable" || *a.Face.Seed != *b.Face.Seed || !s.draft.Face.Random {
		t.Fatal("unstable random sample")
	}
}

func TestPreviewCacheAndInvalidation(t *testing.T) {
	s := freshSession(t)
	cfg := s.draft
	var cache previewCache
	cache.configure(cfg)
	missing := cache.missing(cfg)
	cache.remember(missing, fetchPreview(context.Background(), cfg, s.env, missing))
	if len(cache.missing(cfg)) != 0 || !strings.Contains(cache.line(cfg, s.term.runtime), "Ubuntu") {
		t.Fatal("snapshot not reused")
	}
	cfg.Color = "never"
	if cache.configure(cfg) {
		t.Fatal("styling invalidated values")
	}
	cfg.Modules.Disk.Mount = "/srv"
	if !cache.configure(cfg) || len(cache.missing(cfg)) == 0 {
		t.Fatal("changed module inputs not invalidated")
	}
	cfg.Format = "{face}"
	if !strings.Contains(cache.line(cfg, s.term.runtime), "Preview:") {
		t.Fatal("face preview")
	}
	cfg.Format = "{battery}"
	if cache.line(cfg, s.term.runtime) != "Preview: no available values" {
		t.Fatal("empty preview")
	}
}

func newTestWizard(t *testing.T) (*session, wizardModel) {
	t.Helper()
	s := freshSession(t)
	s.derive()
	runtime := s.term.runtime
	runtime.Basic = false
	runtime.Width = 80
	runtime.Rows = 24
	runtime.Getenv = func(k string) string {
		if k == "NO_COLOR" {
			return "1"
		}
		return "xterm"
	}
	m := wizardModel{bridge: &wizardBridge{ctx: context.Background(), requests: make(chan wizardRequest)}, cancel: func() {}, runtime: runtime, env: s.env, preview: "Preview: preparing…", pending: make(map[string]bool)}
	return s, m
}
func requireWizard(t *testing.T, model tea.Model) wizardModel {
	t.Helper()
	m, ok := model.(wizardModel)
	if !ok {
		t.Fatalf("unexpected %T", model)
	}
	return m
}
func askWizard(t *testing.T, m wizardModel, s *session) wizardModel {
	t.Helper()
	q := s.question()
	q.preview = s.previewDraft()
	next, cmd := m.Update(wizardRequest{q: &q, reply: make(chan answer, 1)})
	m = requireWizard(t, next)
	if cmd != nil {
		msg := cmd()
		next, _ = m.Update(msg)
		m = requireWizard(t, next)
	}
	return m
}

func TestPersistentPreviewNavigation(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "color.ready"
	m = askWizard(t, m, s)
	before := m.preview
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = requireWizard(t, next)
	if !strings.Contains(m.preview, "\x1b[") || m.preview == before || s.draft.Color != "auto" {
		t.Fatal("tentative color not previewed")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = requireWizard(t, next)
	if !m.prompt.finished || s.draft.Color != "auto" {
		t.Fatal("back changed accepted answer")
	}
	s.state = "icons.ready"
	s.draft.Icons.Set = "ascii"
	m = askWizard(t, m, s)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = requireWizard(t, next)
	if !strings.Contains(m.preview, "OS Ubuntu") || s.draft.Icons.Mode != "never" {
		t.Fatal("tentative icons")
	}
	plain := ansi.Strip(m.View().Content)
	if !strings.HasPrefix(plain, "smol · setup\nPreview:") || strings.Index(plain, "Output icons") < strings.Index(plain, "Preview:") {
		t.Fatal(plain)
	}
	if strings.Count(plain, "smol · setup") != 1 {
		t.Fatal("duplicate header")
	}
}

func TestPreviewInputValidationAndStaleResults(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "style.custom"
	s.styleProperty = "Foreground"
	m = askWizard(t, m, s)
	last := m.preview
	m.prompt.editor.SetValue("#bad")
	if cmd := m.refreshPreview(); cmd != nil || m.preview != last {
		t.Fatal("invalid input replaced preview")
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requireWizard(t, next)
	if m.prompt.finished || m.prompt.feedback == "" {
		t.Fatal("invalid input accepted")
	}
	m.prompt.editor.SetValue("#010203")
	_ = m.refreshPreview()
	if m.candidate.Style[s.selected[0]].FG != "#010203" {
		t.Fatal("valid text not previewed")
	}
	generation := m.cache.generation
	next, _ = m.Update(previewFetched{generation: generation - 1, toks: []render.Token{{Module: "os"}}, values: map[string]modules.Result{"os": {Text: "stale"}}})
	m = requireWizard(t, next)
	if strings.Contains(m.preview, "stale") {
		t.Fatal("stale result rendered")
	}
}

func TestWizardSizingAndInspection(t *testing.T) {
	for _, width := range []int{20, 40, 80} {
		s, m := newTestWizard(t)
		s.state = "modules.ready"
		m.runtime.Width = width
		m = askWizard(t, m, s)
		if width < 40 {
			if !m.fallback {
				t.Fatal("narrow fallback missing")
			}
			continue
		}
		if m.fallback || len(strings.Split(m.View().Content, "\n")) >= 24 {
			t.Fatal("menu does not fit", m.View().Content)
		}
		node := m.Inspect()
		if len(node.Children) == 0 {
			t.Fatal("inspection missing")
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("overflow %d: %q", width, line)
			}
		}
		next, _ := m.Update(tea.WindowSizeMsg{Width: 19, Height: 10})
		if !requireWizard(t, next).fallback {
			t.Fatal("resize fallback missing")
		}
	}
	s, m := newTestWizard(t)
	s.state = "icons.set"
	m = askWizard(t, m, s)
	action := m.prompt.Inspect().Children[0].Actions[0]
	if action.Enabled {
		next, _ := m.Update(inspect.Invoke(action.ID))
		if requireWizard(t, next).prompt.currentValue() == "" {
			t.Fatal("inspection action lost selection")
		}
	}
}

func TestWizardPreviewSnapshots(t *testing.T) {
	for _, width := range []int{40, 80} {
		t.Run(strings.Repeat("w", width/40), func(t *testing.T) {
			s, m := newTestWizard(t)
			s.state = "icons.ready"
			m.runtime.Width = width
			m = askWizard(t, m, s)
			snaptest.Snap(t, m.View().Content)
		})
	}
}

func TestRichStyleEditor(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.state = "icons.ready"
	accept(t, s, "always")
	accept(t, s, "ascii")
	accept(t, s, "always")
	accept(t, s, "Edit styles")
	accept(t, s, "mem")
	accept(t, s, "Background")
	accept(t, s, "Custom color")
	if s.question().validate("#bad") == nil {
		t.Fatal("invalid custom color accepted")
	}
	accept(t, s, "#112233")
	accept(t, s, "Attributes")
	accept(t, s, "bold,dim,italic,underline,reverse")
	st := s.draft.Style["mem"]
	if st.BG != "#112233" || len(styleAttributes(st)) != 5 {
		t.Fatal(st)
	}
	accept(t, s, "Foreground")
	accept(t, s, "terminal default")
	accept(t, s, "Done")
	accept(t, s, "Done")
	if s.state != "shell.ready" || s.draft.Style["mem"].FG != "" {
		t.Fatal(s.state)
	}
	for _, state := range []string{"icons.ready", "icons.set", "style.module", "style.property", "style.color", "style.custom", "style.attributes"} {
		s.state = state
		if s.question().title == "" {
			t.Fatal(state)
		}
		s.back()
		if s.state == state {
			t.Fatalf("no Back for %s", state)
		}
	}
}

func TestPreviewClippingRespectsPlainOutput(t *testing.T) {
	text := strings.Repeat("plain ", 30)
	clipped := previewRows(text, 20)
	if strings.Count(clipped, "\n") != 1 || strings.Contains(clipped, "\x1b") || !strings.HasSuffix(clipped, "…") {
		t.Fatalf("%q", clipped)
	}
	for _, line := range strings.Split(clipped, "\n") {
		if ansi.StringWidth(line) > 20 {
			t.Fatal(line)
		}
	}
	colored := previewRows("\x1b[31m"+text+"\x1b[0m", 20)
	if !strings.HasSuffix(colored, "\x1b[0m") {
		t.Fatal("clipped style leaks")
	}
}

func TestWizardInspectionUsesAbsoluteBounds(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "icons.set"
	m = askWizard(t, m, s)
	tree := m.Inspect()
	question := tree.Children[len(tree.Children)-1]
	bounds := question.Children[0].Bounds
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if bounds.Y < 3 || bounds.Y >= len(lines) || !strings.Contains(lines[bounds.Y], "nerd-font") {
		t.Fatalf("bounds %+v do not match %q", bounds, lines)
	}
	m.reply = nil
	if len(m.Inspect().Children) != 2 {
		t.Fatal("hidden controls exposed while working")
	}
}
