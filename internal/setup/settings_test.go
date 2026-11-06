package setup

import (
	"context"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/snaptest"
	"github.com/pelletier/go-toml/v2"
	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/modules"
)

func TestWizardSettingsSave(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.draft.Format = "{colors} {mem:percent} {uptime}"
	s.derive()
	values := map[string]string{
		"modules.colors.shape": "circles", "modules.disk.mount": "/srv", "modules.time.format": "Mon 15:04:05",
		"modules.battery.device": "BAT1", "modules.localip.interface": "eth0", "modules.localip.family": "ipv6", "modules.packages.manager": "apk",
		"display.size.unit": "si", "display.size.digits": "3", "display.duration.abbreviation": "false",
		"display.duration.space-before-unit": "true", "display.duration.compact": "true", "display.duration.round": "true",
		"display.percent.digits": "2", "display.percent.width": "20", "display.temp.unit": "f", "display.temp.digits": "1",
	}
	for _, f := range settings() {
		t.Run(f.key, func(t *testing.T) {
			s.state = "settings.ready"
			group, menu := "Module options", "module.options"
			if strings.HasPrefix(f.key, "display.") {
				group, menu = "Display formatting", "display.options"
			}
			accept(t, s, group)
			if !contains(s.question().choices, f.key) {
				t.Fatal("setting is not reachable")
			}
			accept(t, s, f.key)
			q := s.question()
			value, ok := values[f.key]
			if !ok {
				t.Fatal("missing test value")
			}
			before := cloneDraft(s.draft)
			if err := q.validate(value); err != nil {
				t.Fatal(err)
			}
			preview, err := s.previewDraft()(value)
			if err != nil || f.value(preview) != value || !reflect.DeepEqual(before, s.draft) {
				t.Fatalf("preview changed accepted state or lost candidate: %v", err)
			}
			accept(t, s, value)
			if s.state != menu || f.value(s.draft) != value {
				t.Fatal(s.state, f.value(s.draft))
			}
		})
	}
	want := cloneDraft(s.draft)
	s.state = "review.ready"
	s.term.runtime.Input = strings.NewReader("\n2\n")
	if code := s.run(context.Background(), io.Discard); code != 0 {
		t.Fatalf("save failed: %d", code)
	}
	got, err := config.Load(s.path, true)
	if err != nil || !reflect.DeepEqual(want, got) {
		t.Fatalf("save lost settings: %v\nwant %+v\ngot %+v", err, want, got)
	}
}

func TestWizardSettingValidation(t *testing.T) {
	invalid := map[string]string{
		"modules.colors.shape": "triangles", "modules.disk.mount": "relative", "modules.time.format": "",
		"modules.battery.device": "../BAT0", "modules.localip.family": "bad", "modules.packages.manager": "rpm",
		"display.size.unit": "bad", "display.size.digits": "7", "display.duration.round": "maybe",
		"display.percent.digits": "-1", "display.percent.width": "101", "display.temp.unit": "bad", "display.temp.digits": "-1",
	}
	for key, value := range invalid {
		t.Run(key, func(t *testing.T) {
			s := freshSession(t)
			s.state, s.settingKey, s.settingReturn = "setting.value", key, "module.options"
			before := cloneDraft(s.draft)
			if err := s.question().validate(value); err == nil {
				t.Fatal("accepted invalid option")
			}
			if err := s.accept(context.Background(), value); err == nil || s.state != "setting.value" || !reflect.DeepEqual(before, s.draft) {
				t.Fatal("invalid answer changed session")
			}
		})
	}
	s := freshSession(t)
	s.state, s.settingKey = "setting.value", "display.percent.width"
	if err := s.question().validate("1.5"); err == nil {
		t.Fatal("accepted fractional width")
	}
}

func TestWizardCustomFormatAndIcons(t *testing.T) {
	s := freshSession(t)
	s.derive()
	s.state = "review.ready"
	s.queue, s.index = []string{"os", "kernel"}, 1
	accept(t, s, "Edit format")
	format := "{{palette}} {colors} | {mem:used}/{mem:total} {mem:percent}"
	if err := s.question().validate("{unknown}"); err == nil {
		t.Fatal("accepted unknown module")
	}
	accept(t, s, format)
	if s.draft.Format != format || !reflect.DeepEqual(s.selected, []string{"colors", "mem"}) {
		t.Fatal(s.draft.Format, s.selected)
	}
	if !reflect.DeepEqual(s.queue, []string{"mem"}) || s.index != 0 {
		t.Fatal("custom format left stale field navigation", s.queue, s.index)
	}
	accept(t, s, "Icon details")
	accept(t, s, "Separator")
	if err := s.question().validate("\x1b[31m"); err == nil {
		t.Fatal("accepted control sequence")
	}
	accept(t, s, "")
	accept(t, s, "Module overrides")
	accept(t, s, "colors")
	accept(t, s, "Custom icon")
	if err := s.question().validate("bad\nicon"); err == nil {
		t.Fatal("accepted multiline icon")
	}
	accept(t, s, "palette")
	accept(t, s, "mem")
	accept(t, s, "Hide icon")
	if s.draft.Icons.Separator != "" || s.draft.Icons.Overrides["colors"] != "palette" {
		t.Fatal(s.draft.Icons)
	}
	if value, ok := s.draft.Icons.Overrides["mem"]; !ok || value != "" {
		t.Fatal("hide icon did not persist empty override")
	}
	accept(t, s, "colors")
	accept(t, s, "Built-in icon")
	if _, ok := s.draft.Icons.Overrides["colors"]; ok {
		t.Fatal("built-in icon did not remove override")
	}
	accept(t, s, "Done")
	accept(t, s, "Done")
	accept(t, s, "Continue")
	if s.state != "review.ready" {
		t.Fatal(s.state)
	}
	text, err := config.TOML(s.draft)
	if err != nil {
		t.Fatal(err)
	}
	round, err := config.Parse("wizard", []byte(text))
	if err != nil || !reflect.DeepEqual(s.draft, round) {
		t.Fatalf("custom format/icons lost during roundtrip: %v", err)
	}
}

func TestWizardSettingsBack(t *testing.T) {
	for _, tc := range []struct{ from, to string }{
		{"settings.ready", "order.ready"}, {"module.options", "settings.ready"}, {"display.options", "settings.ready"},
		{"format.custom", "settings.ready"}, {"icons.options", "settings.ready"}, {"icons.separator", "icons.options"},
		{"icons.module", "icons.options"}, {"icons.override", "icons.module"}, {"icons.custom", "icons.override"}, {"setting.value", "display.options"},
	} {
		t.Run(tc.from, func(t *testing.T) {
			s := freshSession(t)
			s.state, s.settingReturn = tc.from, "display.options"
			before := cloneDraft(s.draft)
			s.back()
			if s.state != tc.to || !reflect.DeepEqual(s.draft, before) {
				t.Fatal(s.state)
			}
		})
	}
}

func TestWizardPalettePreview(t *testing.T) {
	s := freshSession(t)
	s.draft.Format = "{colors}"
	s.draft.Modules.Colors.Shape = "circles"
	s.derive()
	if got := cloneDraft(s.draft).Modules.Options().Colors.Shape; got != "circles" {
		t.Fatal("clone lost shape:", got)
	}
	s.state = "module.options"
	accept(t, s, "modules.colors.shape")
	s.draft.Modules.Colors.Shape = "blocks"
	q := s.question()
	q.preview = s.previewDraft()
	m := newPrompt(q, 80)
	result := snaptest.RunScenario(m,
		snaptest.ScenarioStep{Name: "circles", Msg: tea.KeyPressMsg{Code: tea.KeyDown}},
		snaptest.ScenarioStep{Name: "accept", Msg: tea.KeyPressMsg{Code: tea.KeyEnter}},
	)
	final := requirePrompt(t, result.Model)
	if final.result.value != "circles" {
		t.Fatal(final.result)
	}
	candidate, err := q.preview(final.result.value)
	if err != nil || s.draft.Modules.Colors.Shape != "blocks" {
		t.Fatal("preview mutated accepted shape", err)
	}
	var cache previewCache
	cache.configure(candidate)
	toks := cache.missing(candidate)
	cache.remember(toks, fetchPreview(context.Background(), candidate, s.env, toks))
	if line := cache.line(candidate, s.term.runtime); strings.Count(line, "●") != 16 || strings.Contains(line, "█") {
		t.Fatal(line)
	}
}

func TestWizardSettingsBasicEmpty(t *testing.T) {
	for _, key := range []string{"modules.battery.device", "modules.localip.interface"} {
		s := freshSession(t)
		s.draft.Modules.Battery.Device, s.draft.Modules.LocalIP.Interface = "BAT0", "eth0"
		s.state, s.settingKey, s.settingReturn = "setting.value", key, "module.options"
		s.term.runtime.Input = strings.NewReader(":empty\n")
		value, err := s.term.ask(context.Background(), s.question())
		if err != nil || value != "" {
			t.Fatal(value, err)
		}
		accept(t, s, value)
		_ = s.term.close()
	}
}

// A newly added persistent option must have a wizard control.
func TestWizardConfigurationCoverage(t *testing.T) {
	covered := map[string]bool{"format": true, "color": true, "style": true,
		"face.random": true, "face.preset": true, "face.seed": true, "face.state": true,
		"icons.mode": true, "icons.set": true, "icons.separator": true, "icons.overrides": true}
	for _, f := range settings() {
		covered[f.key] = true
	}
	cfg := config.Default()
	seed := "test"
	cfg.Face.Random, cfg.Face.Preset, cfg.Face.Seed = true, "kitty", &seed
	cfg.Icons.Overrides = map[string]string{"mem": "M"}
	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	section := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]")
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		path := strings.TrimSpace(key)
		if section != "" {
			path = section + "." + path
		}
		if strings.HasPrefix(path, "style.") || strings.HasPrefix(path, "icons.overrides.") {
			continue
		}
		if !covered[path] {
			t.Errorf("no wizard control for %s", path)
		}
	}
}

func TestWizardSettingsSnapshots(t *testing.T) {
	for _, width := range []int{39, 80} {
		for _, state := range []string{"settings.ready", "module.options", "display.options", "setting.value", "format.custom", "icons.options", "icons.override"} {
			t.Run(strconv.Itoa(width)+"/"+state, func(t *testing.T) {
				s := freshSession(t)
				s.derive()
				s.state, s.settingKey, s.iconModule = state, "modules.colors.shape", "colors"
				model := newPrompt(s.question(), width)
				snaptest.Snap(t, model.View().Content)
				snaptest.SnapCells(t, model.View().Content, snaptest.WithRoles(tuiweave.CatppuccinMocha()))
			})
		}
	}
}

func TestPaletteOptionsInvalidatePreview(t *testing.T) {
	var cache previewCache
	cfg := config.Default()
	cache.configure(cfg)
	cfg.Modules.Colors = &modules.ColorsOptions{Shape: "circles"}
	if !cache.configure(cfg) {
		t.Fatal("shape change did not invalidate preview")
	}
}
