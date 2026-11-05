package render

import (
	"context"
	"strings"
	"time"

	"github.com/rigerc/smol/internal/modules"
)

// DefaultTimeout bounds how long Render waits for modules. Modules run in
// parallel, so this is also the worst-case added latency.
const DefaultTimeout = 20 * time.Millisecond

// Options control rendering.
type Options struct {
	Color   bool
	Shell   Shell
	Timeout time.Duration
	Lookup  func(string) (modules.Module, bool)
	Styles  map[string]Style // keyed by module name
}

type result struct {
	name string
	val  string
	err  error
}

// fetchAll runs the named modules concurrently and returns the values of
// those that finished successfully before ctx expired. Modules that fail,
// are unavailable, are unknown or time out are simply absent from the map.
func fetchAll(ctx context.Context, names []string, env modules.Env, lookup func(string) (modules.Module, bool)) map[string]string {
	ch := make(chan result, len(names))
	started := 0
	for _, n := range names {
		m, ok := lookup(n)
		if !ok {
			continue
		}
		started++
		go func() {
			v, err := m.Fetch(ctx, env)
			ch <- result{n, v, err}
		}()
	}
	out := make(map[string]string, started)
	for i := 0; i < started; i++ {
		select {
		case r := <-ch:
			if r.err == nil {
				if value := cleanValue(r.val); value != "" {
					out[r.name] = value
				}
			}
		case <-ctx.Done():
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
func Fetch(ctx context.Context, toks []Token, env modules.Env, opts Options) map[string]string {
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
	return fetchAll(ctx, ModuleNames(toks), env, lookup)
}

// Assemble formats already-fetched values; it performs no I/O.
func Assemble(toks []Token, vals map[string]string, opts Options) string {
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
			default:
				sep = t.Literal
			}
			continue
		}
		v, ok := vals[t.Module]
		v = cleanValue(v)
		if !ok || v == "" {
			continue
		}
		if emitted {
			b.WriteString(escapeValue(sep, opts.Shell))
		}
		v = escapeValue(v, opts.Shell)
		if opts.Color {
			v = paint(v, opts.Styles[t.Module], opts.Shell)
		}
		b.WriteString(v)
		emitted = true
		sep = ""
	}
	return b.String()
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
