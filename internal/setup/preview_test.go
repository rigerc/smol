package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ishan5ain/tuiweave"
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
	if !strings.HasPrefix(plain, "smol · setup · 3/4 · Appearance\n\n╭ Preview") || strings.Index(plain, "Output icons") < strings.Index(plain, "╭ Preview") {
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
		m.runtime.Rows = 48
		m = askWizard(t, m, s)
		if width < 40 {
			if !m.fallback {
				t.Fatal("narrow fallback missing")
			}
			continue
		}
		if m.fallback || len(strings.Split(m.View().Content, "\n")) >= m.runtime.Rows {
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
			m.runtime.Rows = 32
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

func TestPreviewPanelFitsAndHonorsNoColor(t *testing.T) {
	for _, width := range []int{1, 5, 19, 40, 80} {
		runtime := Runtime{Width: width, Getenv: func(string) string { return "1" }}
		panel := previewPanel("Preview: \x1b[31m"+strings.Repeat("preview ", 30)+"\x1b[0m", runtime)
		if strings.Contains(panel, "\x1b") {
			t.Fatal("preview ignored NO_COLOR")
		}
		for _, line := range strings.Split(panel, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("preview exceeds %d columns: %q", width, line)
			}
		}
		if width >= 6 && (!strings.HasPrefix(panel, "╭ Preview") || !strings.Contains(panel, "╰")) {
			t.Fatal("missing preview border")
		}
	}
}

func TestPreviewPanelIsTransparent(t *testing.T) {
	runtime := Runtime{Width: 40, Getenv: func(key string) string {
		if key == "TERM" {
			return "xterm"
		}
		return ""
	}}
	panel := previewPanel("Preview: plain", runtime)
	if !strings.Contains(panel, "\x1b[") {
		t.Fatal("expected styled preview")
	}
	for _, background := range []string{"[48;", "[48m"} {
		if strings.Contains(panel, background) {
			t.Fatalf("preview paints a background: %q", panel)
		}
	}
}

func TestQuestionGroupsAndTitles(t *testing.T) {
	for _, tc := range []struct{ state, section string }{
		{"destination.ready", "Configuration"}, {"preset.ready", "Configuration"},
		{"face.mode", "Face"}, {"face.identity", "Face"}, {"face.sample", "Face"}, {"face.expression", "Face"},
		{"modules.ready", "Modules"}, {"fields.ready", "Modules"}, {"order.ready", "Modules"},
		{"icons.ready", "Appearance"}, {"icons.set", "Appearance"}, {"color.ready", "Appearance"}, {"styles.ready", "Appearance"}, {"style.field", "Appearance"},
		{"shell.ready", "Shell"}, {"review.ready", "Save"}, {"confirm.ready", "Save"},
	} {
		t.Run(tc.state, func(t *testing.T) {
			s := freshSession(t)
			s.state = tc.state
			s.selected = []string{"mem"}
			s.queue = []string{"mem"}
			s.faceMode = "seed"
			q := s.question()
			if q.section != tc.section {
				t.Fatalf("section = %q, want %q", q.section, tc.section)
			}
			if q.title == "" {
				t.Fatal("missing title")
			}
		})
	}
	s := freshSession(t)
	s.path = "/home/ada/.config/smol/config.toml"
	s.state = "destination.ready"
	q := s.question()
	if q.title != "Destination" || q.description == "" {
		t.Fatalf("destination prompt = %+v", q)
	}
	view := ansi.Strip(newPrompt(q, 80).View().Content)
	if !strings.Contains(view, "◆  Configuration · Destination") || !strings.Contains(view, "Fixed for this session") {
		t.Fatal(view)
	}
	wd, m := newTestWizard(t)
	wd.state = "destination.ready"
	m = askWizard(t, m, wd)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requireWizard(t, next)
	wd.state = "preset.ready"
	m = askWizard(t, m, wd)
	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "◇  Destination") || !strings.Contains(plain, "◆  Configuration preset") {
		t.Fatal(plain)
	}
	wd.state = "face.mode"
	m = askWizard(t, m, wd)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = requireWizard(t, next)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = requireWizard(t, next)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requireWizard(t, next)
	wd.state = "face.identity"
	m = askWizard(t, m, wd)
	plain = ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "◇  Character") || !strings.Contains(plain, "│  Use a fixed seed") || !strings.Contains(plain, "◆  Face seed") {
		t.Fatal(plain)
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

func TestInlineMenuScrollsWithoutLosingSelections(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "modules.ready"
	m.runtime.Rows = 24
	m = askWizard(t, m, s)
	if m.fallback || m.reply == nil {
		t.Fatal("long menus must remain inline at 80x24")
	}
	if got := strings.Split(m.prompt.currentValue(), ","); len(got) != len(s.selected) {
		t.Fatal("fallback lost selected modules")
	}
	for _, name := range s.selected {
		if !contains(m.prompt.multi.SelectedIDs(), name) {
			t.Fatalf("fallback lost %s", name)
		}
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
	if bounds.Y < 3 || bounds.Y >= len(lines) || !strings.Contains(lines[bounds.Y], "◆") || !strings.Contains(lines[bounds.Y+2], "nerd-font") {
		t.Fatalf("bounds %+v do not match %q", bounds, lines)
	}
	m.reply = nil
	if len(m.Inspect().Children) != 2 {
		t.Fatal("hidden controls exposed while working")
	}
}

func TestWizardReorderPreviewAndHistory(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "order.ready"
	m.runtime.Rows = 80
	m = askWizard(t, m, s)
	before := m.candidate.Format
	next, _ := m.Update(inspect.Invoke("order.move-down"))
	m = requireWizard(t, next)
	if m.candidate.Format == before || m.candidate.Format == "" {
		t.Fatal("reorder did not refresh preview")
	}
	value := m.prompt.currentValue()
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requireWizard(t, next)
	if len(m.history) != 1 || m.history[0].value != value {
		t.Fatal("accepted order was not retained")
	}
	s.state = "fields.ready"
	// Keep the following prompt in Modules so order remains collapsed above it.
	s.queue = []string{"mem"}
	s.index = 0
	m = askWizard(t, m, s)
	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "◇  Order") || !strings.Contains(plain, "◆  Modules") {
		t.Fatal(plain)
	}
	question := m.Inspect().Children[2]
	if lines := strings.Split(plain, "\n"); !strings.Contains(lines[question.Children[0].Bounds.Y], "◆") {
		t.Fatal("collapsed history displaced inspection bounds")
	}
}

func TestInlineListsKeepCursorVisible(t *testing.T) {
	for _, state := range []string{"modules.ready", "order.ready"} {
		t.Run(state, func(t *testing.T) {
			s, m := newTestWizard(t)
			s.state = state
			if state == "order.ready" {
				s.selected = modules.Names()
			}
			m = askWizard(t, m, s)
			if m.fallback {
				t.Fatal("long list fell back")
			}
			next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			m = requireWizard(t, next)
			tree := m.prompt.Inspect().Children[0]
			if tree.Scroll == nil || tree.Scroll.Offset == 0 || tree.Scroll.Offset+tree.Scroll.Visible != tree.Scroll.Total {
				t.Fatalf("cursor did not scroll to end: %+v", tree.Scroll)
			}
			plain := ansi.Strip(m.View().Content)
			if tree.Selected == nil || !strings.Contains(plain, tree.Selected.Label) || len(strings.Split(plain, "\n")) >= 24 {
				t.Fatal("selected item not visible in terminal", plain)
			}
			if state == "order.ready" {
				next, _ = m.Update(inspect.Invoke("order.move-up"))
				m = requireWizard(t, next)
				if m.prompt.order.Selected() != len(s.selected)-2 {
					t.Fatal("scrolling prevented reorder")
				}
			}
		})
	}
}

func TestInlineHistoryLeavesRoomForActiveField(t *testing.T) {
	s, m := newTestWizard(t)
	for _, state := range []string{"face.mode", "face.identity", "face.expression", "face.sample"} {
		s.state = state
		m = askWizard(t, m, s)
		next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = requireWizard(t, next)
		if m.fallback {
			t.Fatalf("%s fell back", state)
		}
	}
	if len(strings.Split(m.View().Content, "\n")) >= 24 {
		t.Fatal("history overflowed terminal")
	}
}

func TestInlineMenuScrollPosition(t *testing.T) {
	for _, rows := range []int{16, 24} {
		for _, width := range []int{40, 80} {
			t.Run(fmt.Sprintf("%dx%d", width, rows), func(t *testing.T) {
				s, m := newTestWizard(t)
				m.runtime.Rows, m.runtime.Width = rows, width
				s.state = "modules.ready"
				m = askWizard(t, m, s)
				if m.fallback || !strings.Contains(m.View().Content, "Choices 1-") || !strings.Contains(m.View().Content, "4 selected") {
					t.Fatal("missing scroll affordance", m.View().Content)
				}
				for range len(m.prompt.q.choices) {
					next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
					m = requireWizard(t, next)
				}
				tree := m.prompt.Inspect().Children[0]
				if m.fallback || tree.Scroll == nil || tree.Scroll.Offset+tree.Scroll.Visible != tree.Scroll.Total {
					t.Fatal("last choice is not visible", m.View().Content)
				}
				if len(strings.Split(m.View().Content, "\n")) >= rows {
					t.Fatal("menu overflow", m.View().Content)
				}
			})
		}
	}
}

func TestInlineReorderPreviewAndBack(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "order.ready"
	m = askWizard(t, m, s)
	original := s.draft.Format
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})
	m = requireWizard(t, next)
	if m.prompt.currentValue() != "1 2 3 4" {
		t.Fatal("moving the first item up changed the order")
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	m = requireWizard(t, next)
	if m.prompt.currentValue() != "2 1 3 4" || m.prompt.order.SelectedItem().ID != "1" || m.candidate.Format != "{kernel} · {os} · {uptime} · {mem}" || s.draft.Format != original {
		t.Fatal("reorder did not preserve focus and tentative state", m.prompt.currentValue(), m.candidate.Format)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = requireWizard(t, next)
	if !errors.Is(m.prompt.result.err, errBack) || s.draft.Format != original {
		t.Fatal("back accepted a tentative order")
	}
	m = askWizard(t, m, s)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	m = requireWizard(t, next)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requireWizard(t, next)
	accept(t, s, m.prompt.result.value)
	if s.draft.Format != "{kernel} · {os} · {uptime} · {mem}" {
		t.Fatal("accepted order was not applied", s.draft.Format)
	}
}

func TestReorderSurvivesNumberedFallback(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "order.ready"
	m = askWizard(t, m, s)
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	m = requireWizard(t, next)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 19, Height: 10})
	m = requireWizard(t, next)
	if !m.fallback {
		t.Fatal("narrow terminal did not switch input modes")
	}
	q := m.prompt.q
	q.value = m.prompt.currentValue()
	s.term.runtime.Input = strings.NewReader("1 1 3 4\n\n")
	var diag bytes.Buffer
	s.term.out = &diag
	defer func() { _ = s.term.close() }()
	value, err := s.term.basic(context.Background(), q)
	if err != nil || value != "2 1 3 4" {
		t.Fatal("fallback lost tentative order", value, err)
	}
	if !strings.Contains(diag.String(), "1. os") || !strings.Contains(diag.String(), "2. kernel") || !strings.Contains(diag.String(), "list every enabled module number once") {
		t.Fatal("numbered mode lost stable indices or validation", diag.String())
	}
}

func TestInlineReviewAndFullTOML(t *testing.T) {
	s, m := newTestWizard(t)
	s.state = "review.ready"
	s.faceMode = "disabled"
	var diag bytes.Buffer
	s.term.out = &diag
	s.term.bridge = &wizardBridge{ctx: context.Background(), requests: make(chan wizardRequest, 1)}
	if err := s.prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if diag.Len() != 0 || s.review == "" {
		t.Fatal("inline review should not dump TOML into scrollback")
	}
	m = askWizard(t, m, s)
	if !strings.Contains(m.View().Content, "Modules: os, kernel, uptime, mem") || strings.Contains(m.View().Content, "[style.") {
		t.Fatal("review summary missing", m.View().Content)
	}
	accept(t, s, "View full TOML")
	m = askWizard(t, m, s)
	if m.prompt.q.document != s.review || m.fallback {
		t.Fatal("full TOML is unavailable")
	}
	t.Run("top", func(t *testing.T) {
		snaptest.Snap(t, m.View().Content)
		snaptest.SnapCells(t, m.View().Content, snaptest.WithRoles(tuiweave.CatppuccinMocha()))
	})
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	m = requireWizard(t, next)
	if !m.prompt.document.AtBottom() || m.prompt.document.YOffset() == 0 {
		t.Fatal("full TOML does not scroll")
	}
	t.Run("bottom", func(t *testing.T) {
		snaptest.Snap(t, m.View().Content)
	})
	q := m.prompt.q
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = requireWizard(t, next)
	accept(t, s, m.prompt.result.value)
	if s.state != "review.ready" || m.prompt.result.value != "Back to review" {
		t.Fatal("TOML viewer did not return to review")
	}
	accept(t, s, "View full TOML")
	s.back()
	if s.state != "review.ready" {
		t.Fatal("Esc from TOML must return to review")
	}
	s.term.bridge = nil
	s.term.runtime.Input = strings.NewReader("\n")
	defer func() { _ = s.term.close() }()
	value, err := s.term.basic(context.Background(), q)
	if err != nil || value != "Back to review" || !strings.Contains(diag.String(), "[style.os]") {
		t.Fatal("numbered TOML view cannot return", value, err, diag.String())
	}
}
