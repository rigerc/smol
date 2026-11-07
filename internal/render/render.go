package render

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	faceroll "github.com/rigerc/faceroll/go"

	"github.com/rigerc/smol/internal/icons"
	"github.com/rigerc/smol/internal/modules"
)

// DefaultTimeout bounds how long Render waits for modules. Modules run in
// parallel, so this is also the worst-case added latency.
const DefaultTimeout = 20 * time.Millisecond

// Options control rendering.
type Options struct {
	Icons     icons.Options
	ShowIcons bool
	Debug     func(string)
	Color     bool
	Shell     Shell
	Timeout   time.Duration
	Lookup    func(string) (modules.Module, bool)
	Styles    map[string]Style // keyed by module name
}

type result struct {
	name    string
	val     modules.Result
	err     error
	elapsed time.Duration
}

// fetchAll runs the named modules concurrently and returns the values of
// those that finished successfully before ctx expired. Modules that fail,
// are unavailable, are unknown or time out are simply absent from the map.
func fetchAll(ctx context.Context, names []string, env modules.Env, lookup func(string) (modules.Module, bool), debug func(string)) map[string]modules.Result {
	ch := make(chan result, len(names))
	started := 0
	pending := map[string]bool{}
	start := time.Now()
	for _, n := range names {
		m, ok := lookup(n)
		if !ok {
			if debug != nil {
				debug(n + " unknown")
			}
			continue
		}
		started++
		pending[n] = true
		go func() {
			begin := time.Now()
			v, err := m.Fetch(ctx, env)
			ch <- result{n, v, err, time.Since(begin)}
		}()
	}
	out := make(map[string]modules.Result, started)
	for i := 0; i < started; i++ {
		select {
		case r := <-ch:
			delete(pending, r.name)
			if debug != nil {
				status := "ok"
				if r.err != nil {
					status = r.err.Error()
					if errors.Is(r.err, context.DeadlineExceeded) {
						status = "timeout"
					}
				}
				debug(fmt.Sprintf("%s %.1fms %s", r.name, float64(r.elapsed)/float64(time.Millisecond), status))
			}
			if r.err == nil {
				out[r.name] = r.val
			}
		case <-ctx.Done():
			if debug != nil {
				for _, n := range names {
					if pending[n] {
						debug(fmt.Sprintf("%s %.1fms timeout", n, float64(time.Since(start))/float64(time.Millisecond)))
					}
				}
			}
			return out
		}
	}
	return out
}

// Render fetches every module referenced by toks and assembles the line.
//
// Literal text that sits between two modules acts as a separator. When a
// module has no value, it is dropped together with the separator in front of
// it (or, for the first module, nothing extra), so a failed module never
// leaves "· ·" or a dangling separator behind. Literals before the first
// module and after the last one are always kept.
func Render(ctx context.Context, toks []Token, env modules.Env, opts Options) string {
	return Assemble(toks, Fetch(ctx, toks, env, opts), opts)
}

// Fetch reads referenced modules once under a shared deadline.
func Fetch(ctx context.Context, toks []Token, env modules.Env, opts Options) map[string]modules.Result {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	lookup := opts.Lookup
	if lookup == nil {
		lookup = modules.Get
	}
	start := time.Now()
	out := fetchAll(ctx, ModuleNames(toks), env, lookup, opts.Debug)
	if opts.Debug != nil {
		opts.Debug(fmt.Sprintf("total %.1fms", float64(time.Since(start))/float64(time.Millisecond)))
	}
	return out
}

// Assemble formats already-fetched values; it performs no I/O.
func Assemble(toks []Token, vals map[string]modules.Result, opts Options) string {
	return assemble(toks, vals, opts, 0).Text
}

// FaceLine preserves normal formatting while reserving face cells for images.
type FaceLine struct {
	Text    string
	Columns int
	Faces   []int
}

// AssembleFaceLine replaces each face value with a fixed-width blank area.
// Icon prefixes and separator elision follow exactly the normal text renderer.
func AssembleFaceLine(toks []Token, vals map[string]modules.Result, opts Options, faceWidth int) FaceLine {
	return assemble(toks, vals, opts, faceWidth)
}

func assemble(toks []Token, vals map[string]modules.Result, opts Options, faceWidth int) FaceLine {
	line := FaceLine{}
	firstMod, lastMod := -1, -1
	for i, t := range toks {
		if t.Module != "" {
			if firstMod < 0 {
				firstMod = i
			}
			lastMod = i
		}
	}

	var (
		b       strings.Builder
		sep     string // most recent inter-module literal
		emitted bool   // a module has already been written
	)
	for i, t := range toks {
		if t.Module == "" {
			switch {
			case firstMod < 0, i < firstMod, i > lastMod:
				b.WriteString(escapeValue(t.Literal, opts.Shell))
				line.Columns += faceroll.CellWidth(t.Literal)
			default:
				sep = t.Literal
			}
			continue
		}
		result, ok := vals[t.Module]
		v := result.Text
		if t.Field != "" {
			v = result.Fields[t.Field]
		}
		v = cleanValue(v)
		if !ok || v == "" {
			continue
		}
		if emitted {
			b.WriteString(escapeValue(sep, opts.Shell))
			line.Columns += faceroll.CellWidth(sep)
		}
		prefix := ""
		if opts.ShowIcons {
			prefix = cleanValue(opts.Icons.Prefix(t.Module, result.Fields["id"]))
		}
		if faceWidth > 0 && t.Module == "face" {
			line.Faces = append(line.Faces, line.Columns+faceroll.CellWidth(prefix))
			v = strings.Repeat(" ", faceWidth)
		}
		line.Columns += faceroll.CellWidth(prefix + v)
		if opts.Color && t.Module == "colors" {
			if prefix != "" {
				prefix = paint(escapeValue(prefix, opts.Shell), opts.Styles[t.Module], opts.Shell)
			}
			v = prefix + paintPalette(v, opts.Shell)
		} else {
			v = escapeValue(prefix+v, opts.Shell)
			if opts.Color {
				v = paint(v, opts.Styles[t.Module], opts.Shell)
			}
		}
		b.WriteString(v)
		emitted = true
		sep = ""
	}
	line.Text = b.String()
	return line
}

// cleanValue prevents data from proc files and environment values from
// adding lines or injecting terminal control sequences into the output.
func cleanValue(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || (r >= 128 && r <= 159) {
			return -1
		}
		return r
	}, value)
}
