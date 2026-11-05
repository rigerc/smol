// Package modules contains the built-in info modules for smol.
//
// Every module reads from an Env rather than touching the real system
// directly, so tests can point it at fixture trees under testdata/.
package modules

import (
	"context"
	"errors"
	"io/fs"
	"slices"
)

// ErrUnavailable is returned by a module that has nothing to report on this
// system (for example, battery on a desktop). The renderer drops such modules
// silently.
var ErrUnavailable = errors.New("module unavailable")

// Env is everything a module may read from. FS is rooted at "/" in
// production (paths have no leading slash, e.g. "proc/meminfo").
type Env struct {
	FS     fs.FS
	Getenv func(string) string
}

// Module fetches one piece of system information.
type Module struct {
	Fetch func(context.Context, Env) (string, error)
}

// Get returns a system module. The configured face module is injected by app.
func Get(name string) (Module, bool) {
	switch name {
	case "cpu":
		return Module{Fetch: cpuModule{}.Fetch}, true
	case "host":
		return Module{Fetch: hostModule{}.Fetch}, true
	case "kernel":
		return Module{Fetch: kernelModule{}.Fetch}, true
	case "load":
		return Module{Fetch: loadModule{}.Fetch}, true
	case "mem":
		return Module{Fetch: memModule{}.Fetch}, true
	case "os":
		return Module{Fetch: osModule{}.Fetch}, true
	case "shell":
		return Module{Fetch: shellModule{}.Fetch}, true
	case "uptime":
		return Module{Fetch: uptimeModule{}.Fetch}, true
	case "user":
		return Module{Fetch: userModule{}.Fetch}, true
	}
	return Module{}, false
}

// Known reports whether a format/style name is supported.
func Known(name string) bool { return slices.Contains(Names(), name) }

// Names returns all module names, including the configured face module, sorted.
func Names() []string {
	return []string{"cpu", "face", "host", "kernel", "load", "mem", "os", "shell", "uptime", "user"}
}
