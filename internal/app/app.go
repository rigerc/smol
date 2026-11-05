// Package app implements the smol command with injected process dependencies.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/modules"
	"github.com/rigerc/smol/internal/render"
)

// Runtime supplies the process inputs used by the command.
type Runtime struct {
	FS          fs.FS
	Getenv      func(string) string
	StdoutIsTTY bool
	Version     string
}

// Run executes the command and returns its process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime Runtime) int {
	getenv := runtime.Getenv
	stdoutIsTTY := runtime.StdoutIsTTY
	version := runtime.Version

	fl := flag.NewFlagSet("smol", flag.ContinueOnError)
	fl.SetOutput(stderr)
	var (
		format      = fl.String("format", "", "format template, e.g. \"{os} · {mem}\" (overrides config)")
		colorFlag   = fl.String("color", "", "auto, always or never (overrides config)")
		shellFlag   = fl.String("shell", "none", "wrap color escapes for: none, bash, zsh, fish")
		configPath  = fl.String("config", "", "config file (default ~/.config/smol/config.toml)")
		timeout     = fl.Duration("timeout", render.DefaultTimeout, "max time to wait for modules")
		listMods    = fl.Bool("list-modules", false, "list available modules and exit")
		checkConfig = fl.Bool("check-config", false, "validate the config file and exit")
		printConfig = fl.Bool("print-default-config", false, "print the default config and exit")
		showVersion = fl.Bool("version", false, "print version and exit")
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
	switch {
	case *showVersion:
		return writeLine(stdout, stderr, "smol "+version)
	case *listMods:
		return writeLine(stdout, stderr, strings.Join(modules.Names(), "\n"))
	case *printConfig:
		return writeLine(stdout, stderr, strings.TrimSuffix(config.DefaultTOML, "\n"))
	}

	path, explicit := *configPath, *configPath != ""
	if !explicit {
		path = config.DefaultPath(getenv)
	}
	cfg, cfgErr := config.Load(path, explicit)

	if *checkConfig {
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
	shell, err := render.ParseShell(*shellFlag)
	if err != nil {
		report(stderr, "smol: "+err.Error())
		return 2
	}

	toks, err := render.Parse(cfg.Format)
	if err != nil { // unreachable after validation, but never panic in a prompt
		return 0
	}
	env := modules.Env{FS: runtime.FS, Getenv: getenv}
	opts := render.Options{
		Color:   useColor(cfg.Color, getenv, stdoutIsTTY),
		Shell:   shell,
		Timeout: *timeout,
		Styles:  cfg.Style,
	}
	line := render.Render(ctx, toks, env, opts)
	return writeLine(stdout, stderr, line)
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
