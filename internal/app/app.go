// Package app implements the smol command with injected process dependencies.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"time"

	faceroll "github.com/rigerc/faceroll/go"

	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/face"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/playback"
	"github.com/rigerc/smol/internal/render"
)

// Runtime supplies the process inputs used by the command.
type Runtime struct {
	Now         func() time.Time
	Statfs      func(string) (modules.DiskStats, error)
	Uname       func() (string, error)
	Interfaces  func() ([]modules.Interface, error)
	FS          fs.FS
	Getenv      func(string) string
	StdoutIsTTY bool
	Version     string
	Playback    playback.Options
}

// Run executes the command and returns its process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime Runtime) int {
	getenv := runtime.Getenv
	stdoutIsTTY := runtime.StdoutIsTTY
	version := runtime.Version

	fl := flag.NewFlagSet("smol", flag.ContinueOnError)
	fl.SetOutput(stderr)
	var (
		format          = fl.String("format", "", "format template, e.g. \"{os} · {mem}\" (overrides config)")
		colorFlag       = fl.String("color", "", "auto, always or never (overrides config)")
		shellFlag       = fl.String("shell", "none", "wrap color escapes for: none, bash, zsh, fish")
		configPath      = fl.String("config", "", "config file (default ~/.config/smol/config.toml)")
		timeout         = fl.Duration("timeout", render.DefaultTimeout, "max time to wait for modules")
		debug           = fl.Bool("debug", false, "write module timing diagnostics to stderr")
		listFields      = fl.Bool("list-fields", false, "list available module fields")
		listPaths       = fl.Bool("list-config-paths", false, "list config search paths")
		effectiveConfig = fl.Bool("print-config", false, "print effective configuration")
		listMods        = fl.Bool("list-modules", false, "list available modules and exit")
		checkConfig     = fl.Bool("check-config", false, "validate the config file and exit")
		printConfig     = fl.Bool("print-default-config", false, "print the default config and exit")
		facePreset      = fl.String("face-preset", "", "face preset (replaces a configured seed)")
		faceSeed        = fl.String("face-seed", "", "deterministic face seed (replaces a configured preset)")
		faceState       = fl.String("face-state", "", "face expression (default idle)")
		listPresets     = fl.Bool("list-face-presets", false, "list face preset identifiers and exit")
		listStates      = fl.Bool("list-face-states", false, "list face expressions and exit")
		animate         = fl.Bool("animate", false, "animate {face} on an interactive terminal")
		duration        = fl.Duration("duration", 0, "animation duration; omit to run until interrupted")
		showVersion     = fl.Bool("version", false, "print version and exit")
	)
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if fl.NArg() != 0 {
		report(stderr, "smol: unexpected argument: "+fl.Arg(0))
		return 2
	}
	if *timeout <= 0 {
		report(stderr, "smol: timeout must be positive")
		return 2
	}
	if *debug && *animate {
		report(stderr, "smol: --debug and --animate are mutually exclusive")
		return 2
	}
	switch {
	case *listFields:
		var lines []string
		for _, m := range modules.All() {
			for _, f := range m.Fields {
				lines = append(lines, m.Name+"."+f.Name+": "+f.Description)
			}
		}
		return writeLine(stdout, stderr, strings.Join(lines, "\n"))
	case *listPresets:
		return writeLine(stdout, stderr, strings.Join(face.Presets(), "\n"))
	case *listStates:
		return writeLine(stdout, stderr, strings.Join(face.States(), "\n"))
	case *showVersion:
		return writeLine(stdout, stderr, "smol "+version)
	case *listMods:
		var lines []string
		for _, name := range modules.Names() {
			description := "Configured character"
			if m, ok := modules.Get(name); ok {
				description = m.Description
			}
			lines = append(lines, name+": "+description)
		}
		return writeLine(stdout, stderr, strings.Join(lines, "\n"))
	case *printConfig:
		return writeLine(stdout, stderr, strings.TrimSuffix(config.DefaultTOML, "\n"))
	}

	path, explicit := *configPath, *configPath != ""
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	candidates := config.Search(getenv, exists)
	if explicit {
		candidates = []config.Candidate{{Path: path, Found: exists(path)}}
	} else {
		path = ""
		for _, c := range candidates {
			if c.Found {
				path = c.Path
				break
			}
		}
	}
	if *listPaths {
		var lines []string
		for _, c := range candidates {
			line := c.Path
			if c.Found {
				line += " (found)"
			}
			lines = append(lines, line)
		}
		return writeLine(stdout, stderr, strings.Join(lines, "\n"))
	}
	cfg, cfgErr := config.Load(path, explicit)

	if *checkConfig {
		if *debug {
			report(stderr, "smol: config "+path)
		}
		if cfgErr != nil {
			report(stderr, "smol: "+cfgErr.Error())
			return 1
		}
		return writeLine(stdout, stderr, "config ok")
	}
	// In normal operation a broken config must never put error text into a
	// prompt: fall back to defaults. `smol --check-config` reports the error.
	if cfgErr != nil {
		cfg = config.Default()
	}

	supplied := make(map[string]bool)
	fl.Visit(func(f *flag.Flag) { supplied[f.Name] = true })
	if supplied["format"] {
		if err := config.ValidateFormat(*format); err != nil {
			report(stderr, "smol: "+err.Error())
			return 2
		}
		cfg.Format = *format
	}
	if supplied["color"] {
		cfg.Color = *colorFlag
		if err := cfg.Validate(); err != nil {
			report(stderr, "smol: "+err.Error())
			return 2
		}
	}
	if supplied["face-preset"] && supplied["face-seed"] {
		report(stderr, "smol: --face-preset and --face-seed are mutually exclusive")
		return 2
	}
	if supplied["face-preset"] {
		if *facePreset == "" {
			report(stderr, "smol: --face-preset must not be empty")
			return 2
		}
		cfg.Face.Preset, cfg.Face.Seed = *facePreset, nil
	}
	if supplied["face-seed"] {
		cfg.Face.Seed, cfg.Face.Preset = faceSeed, ""
	}
	if supplied["face-state"] {
		cfg.Face.State = *faceState
	}
	if err := cfg.Validate(); err != nil {
		report(stderr, "smol: "+err.Error())
		return 2
	}
	if supplied["duration"] && (!*animate || *duration <= 0) {
		report(stderr, "smol: --duration requires --animate and a positive duration")
		return 2
	}

	if *effectiveConfig {
		text, err := config.TOML(cfg)
		if err != nil {
			report(stderr, "smol: "+err.Error())
			return 1
		}
		return writeLine(stdout, stderr, text)
	}
	shell, err := render.ParseShell(*shellFlag)
	if err != nil {
		report(stderr, "smol: "+err.Error())
		return 2
	}

	toks, err := render.Parse(cfg.Format)
	if err != nil { // unreachable after validation, but never panic in a prompt
		return 0
	}
	hasFace := slices.Contains(render.ModuleNames(toks), "face")
	if *animate && (!hasFace || shell != render.ShellNone) {
		report(stderr, "smol: --animate requires {face} and --shell none")
		return 2
	}
	moduleOptions := cfg.Modules.Options()
	env := modules.Env{FS: runtime.FS, Getenv: getenv, Now: runtime.Now, Statfs: runtime.Statfs, Uname: runtime.Uname, Interfaces: runtime.Interfaces, Display: &cfg.Display, Options: &moduleOptions}
	opts := render.Options{
		Color:   useColor(cfg.Color, getenv, stdoutIsTTY),
		Shell:   shell,
		Timeout: *timeout,
		Styles:  cfg.Style,
	}
	var animation faceroll.Animation
	if hasFace {
		animation, err = face.Build(cfg.Face)
		if err != nil {
			report(stderr, "smol: "+err.Error())
			return 1
		}
		opts.Lookup = func(name string) (modules.Module, bool) {
			if name == "face" {
				return modules.Module{Fetch: func(context.Context, modules.Env) (modules.Result, error) {
					return modules.Result{Text: animation.Still}, nil
				}}, true
			}
			return modules.Get(name)
		}
	}
	if *debug {
		opts.Debug = func(message string) { report(stderr, "smol: "+message) }
	}
	values := render.Fetch(ctx, toks, env, opts)
	// Face generation is local and complete already; it must not disappear when
	// the shared system-module timeout expires.
	if hasFace {
		values["face"] = modules.Result{Text: animation.Still}
	}
	line := render.Assemble(toks, values, opts)
	if !*animate || !stdoutIsTTY || getenv("TERM") == "dumb" {
		return writeLine(stdout, stderr, line)
	}
	frames := make([]playback.Frame, len(animation.Frames))
	plain := opts
	plain.Color = false
	for i, frame := range animation.Frames {
		values["face"] = modules.Result{Text: frame.Text}
		frames[i] = playback.Frame{
			Text:     render.Assemble(toks, values, opts),
			Width:    faceroll.CellWidth(render.Assemble(toks, values, plain)),
			Duration: time.Duration(frame.MS) * time.Millisecond,
		}
	}
	playOpts := runtime.Playback
	playOpts.Duration = *duration
	if err := playback.Run(ctx, stdout, frames, line, playOpts); err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return 130
		}
		report(stderr, "smol: "+err.Error())
		return 1
	}
	return 0
}

// useColor resolves the color mode. NO_COLOR (https://no-color.org) wins over
// "auto" and "always" is explicit, so it also wins over NO_COLOR only when the
// user asked for it on purpose via config or flag.
func useColor(mode string, getenv func(string) string, isTTY bool) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" {
		return false
	}
	return isTTY
}

func writeLine(out, stderr io.Writer, line string) int {
	if _, err := fmt.Fprintln(out, line); err != nil {
		report(stderr, "smol: write output: "+err.Error())
		return 1
	}
	return 0
}

// report is best effort: a diagnostic stream can itself be closed or broken.
func report(out io.Writer, message string) {
	_, _ = fmt.Fprintln(out, message)
}
