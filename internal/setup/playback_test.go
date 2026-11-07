package setup

import (
	"context"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/tuiweave"
	"github.com/rigerc/tuiweave/snaptest"
)

func TestFacePlaybackSave(t *testing.T) {
	for _, mode := range []string{"static", "persist"} {
		t.Run(mode, func(t *testing.T) {
			s := freshSession(t)
			s.derive()
			s.state = "face.mode"
			accept(t, s, "preset")
			accept(t, s, "kitty")
			accept(t, s, "Accept")
			accept(t, s, "working")
			if s.state != "face.playback" || s.question().value != "static" {
				t.Fatal("missing playback choice", s.state)
			}
			before := cloneDraft(s.draft)
			candidate, err := s.previewDraft()(mode)
			if err != nil || candidate.Face.AnimatePersist != (mode == "persist") || !reflect.DeepEqual(before, s.draft) {
				t.Fatal("preview changed accepted configuration", err)
			}
			accept(t, s, mode)
			if s.state != "modules.ready" {
				t.Fatal(s.state)
			}
			s.back()
			if s.state != "face.playback" || s.question().value != mode {
				t.Fatal("lost accepted playback choice")
			}
			s.shell = "fish"
			s.state = "review.ready"
			s.term.runtime.Input = strings.NewReader("\n2\n")
			if code := s.run(context.Background(), io.Discard); code != 0 {
				t.Fatal("save failed", code)
			}
			saved, err := config.Load(s.path, true)
			if err != nil || saved.Face.AnimatePersist != (mode == "persist") {
				t.Fatal("lost playback setting", err)
			}
			if !strings.Contains(s.instructions, "smol --config '") || strings.Contains(s.instructions, "--animate-persist") {
				t.Fatal("startup should use the saved preference", s.instructions)
			}
			reloaded := freshSession(t)
			reloaded.path = s.path
			if err := reloaded.load(context.Background()); err != nil {
				t.Fatal(err)
			}
			reloaded.state = "face.playback"
			if reloaded.question().value != mode {
				t.Fatal("reload lost playback choice")
			}
		})
	}
}

func TestFacePlaybackReviewAndDisable(t *testing.T) {
	s := freshSession(t)
	s.draft.Format = "{face} {host}"
	s.derive()
	s.state = "review.ready"
	accept(t, s, "Edit animation")
	if s.state != "face.playback" {
		t.Fatal(s.state)
	}
	s.back()
	if s.state != "review.ready" {
		t.Fatal("back did not return to review")
	}
	accept(t, s, "Edit animation")
	before := cloneDraft(s.draft)
	if err := s.accept(context.Background(), "invalid"); err == nil || !reflect.DeepEqual(before, s.draft) {
		t.Fatal("invalid choice changed draft")
	}
	accept(t, s, "persist")
	if s.state != "review.ready" || !strings.Contains(s.reviewSummary(), "Face animation: after exit") {
		t.Fatal("review omitted animation", s.state, s.reviewSummary())
	}
	accept(t, s, "Edit modules")
	s.back()
	if s.state != "face.playback" {
		t.Fatal("modules back should revisit playback")
	}
	s.back()
	if s.state != "face.expression" {
		t.Fatal("old review return leaked into normal navigation")
	}
	s.state = "face.mode"
	accept(t, s, "disabled")
	if s.draft.Face.AnimatePersist || s.state != "modules.ready" {
		t.Fatal("disabling the face left animation enabled")
	}
	s.back()
	if s.state != "face.mode" {
		t.Fatal("disabled face should skip playback")
	}
	s.state = "review.ready"
	accept(t, s, "Edit animation")
	if s.state != "face.mode" {
		t.Fatal("must select a face before enabling playback")
	}
}

func TestFacePlaybackWizardScenario(t *testing.T) {
	for _, width := range []int{40, 80} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			s, m := newTestWizard(t)
			s.draft.Format = "{face} {host}"
			s.derive()
			s.state = "face.playback"
			s.term.runtime.Width = width
			m.runtime.Width = width
			m = askWizard(t, m, s)
			result := snaptest.RunScenario(m,
				snaptest.ScenarioStep{Name: "choose persistent animation", Msg: tea.KeyPressMsg{Code: tea.KeyDown}},
				snaptest.ScenarioStep{Name: "accept playback", Msg: tea.KeyPressMsg{Code: tea.KeyEnter}},
			)
			snaptest.SnapScenario(t, result)
			next := requireWizard(t, result.Model)
			if !next.candidate.Face.AnimatePersist {
				t.Fatal("highlight did not preview animation")
			}
		})
	}
}

func TestFacePlaybackPromptStyles(t *testing.T) {
	s := freshSession(t)
	s.state = "face.playback"
	s.draft.Face.AnimatePersist = true
	model := newPrompt(s.question(), 80)
	view := model.View().Content
	if !strings.Contains(ansi.Strip(view), "Animate after smol exits") {
		t.Fatal("missing playback choice")
	}
	snaptest.Snap(t, view)
	snaptest.SnapCells(t, view, snaptest.WithRoles(tuiweave.CatppuccinMocha()))
}
