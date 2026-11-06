package setup

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/displayfmt"
	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/render"
)

// previewDraft captures an independent draft; highlighting never changes accepted answers.
func (s *session) previewDraft() func(string) (config.Config, error) {
	base := *s
	base.draft = cloneDraft(s.draft)
	base.fields = maps.Clone(s.fields)
	base.selected = slices.Clone(s.selected)
	return func(value string) (config.Config, error) {
		next := base
		next.draft = cloneDraft(base.draft)
		next.fields = maps.Clone(base.fields)
		next.selected = slices.Clone(base.selected)
		switch next.state {
		case "preset.ready", "modules.ready", "fields.ready", "order.ready", "color.ready", "icons.ready", "icons.set", "style.field", "style.color", "style.custom", "style.attributes", "setting.value", "format.custom", "icons.separator", "icons.override", "icons.custom":
			if err := next.accept(context.Background(), value); err != nil {
				return base.draft, err
			}
			if next.dirty {
				next.normalize()
			}
		case "face.mode":
			next.setFace(value != "disabled")
		case "face.identity":
			next.draft.Face.Random = false
			if next.faceMode == "preset" {
				next.draft.Face.Preset = value
				next.draft.Face.Seed = nil
			} else {
				next.draft.Face.Preset = ""
				next.draft.Face.Seed = &value
			}
		case "face.expression":
			next.draft.Face.State = value
		}
		if next.draft.Face.Random {
			seed := next.sampleSeed
			if seed == "" {
				seed = "smol"
			}
			next.draft.Face.Random = false
			next.draft.Face.Seed = &seed
		}
		return next.draft, next.draft.Validate()
	}
}

func cloneDraft(cfg config.Config) config.Config {
	cfg.Style = maps.Clone(cfg.Style)
	cfg.Icons.Overrides = maps.Clone(cfg.Icons.Overrides)
	opts := cfg.Modules.Options()
	cfg.Modules = config.ModulesConfig{Colors: &opts.Colors, Disk: &opts.Disk, Time: &opts.Time, Battery: &opts.Battery, LocalIP: &opts.LocalIP, Packages: &opts.Packages}
	if cfg.Face.Seed != nil {
		seed := *cfg.Face.Seed
		cfg.Face.Seed = &seed
	}
	return cfg
}

type previewCache struct {
	display    displayfmt.Options
	options    modules.Options
	generation int
	values     map[string]modules.Result
	attempted  map[string]bool
}

// configure invalidates data when module inputs or display formatting change.
func (p *previewCache) configure(cfg config.Config) bool {
	opts := cfg.Modules.Options()
	if p.generation > 0 && p.display == cfg.Display && p.options == opts {
		return false
	}
	p.display = cfg.Display
	p.options = opts
	p.generation++
	p.values = make(map[string]modules.Result)
	p.attempted = make(map[string]bool)
	return true
}

func (p *previewCache) missing(cfg config.Config) []render.Token {
	toks, _ := render.Parse(cfg.Format)
	var missing []render.Token
	for _, name := range render.ModuleNames(toks) {
		if name != "face" && !p.attempted[name] {
			missing = append(missing, render.Token{Module: name})
		}
	}
	return missing
}

func (p *previewCache) remember(toks []render.Token, values map[string]modules.Result) {
	if p.values == nil {
		p.values = make(map[string]modules.Result)
	}
	if p.attempted == nil {
		p.attempted = make(map[string]bool)
	}
	for _, tok := range toks {
		p.attempted[tok.Module] = true
	}
	maps.Copy(p.values, values)
}

func fetchPreview(ctx context.Context, cfg config.Config, env modules.Env, toks []render.Token) map[string]modules.Result {
	if env.FS == nil {
		return map[string]modules.Result{}
	}
	env.Display = &cfg.Display
	opts := cfg.Modules.Options()
	env.Options = &opts
	return render.Fetch(ctx, toks, env, render.Options{})
}

func (p *previewCache) line(cfg config.Config, runtime Runtime) string {
	values := maps.Clone(p.values)
	if values == nil {
		values = make(map[string]modules.Result)
	}
	toks, _ := render.Parse(cfg.Format)
	if contains(render.ModuleNames(toks), "face") {
		animation, err := face.Build(cfg.Face)
		if err == nil {
			values["face"] = modules.Result{Text: animation.Still}
		}
	}
	line := render.Assemble(toks, values, render.Options{Styles: cfg.Style, Color: previewColor(cfg.Color, runtime), Icons: cfg.Icons, ShowIcons: cfg.Icons.Enabled(runtime.StderrTTY, runtime.Getenv("TERM"))})
	if strings.TrimSpace(line) == "" {
		return "Preview: no available values"
	}
	return "Preview: " + line
}
