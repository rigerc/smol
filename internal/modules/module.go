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
	"strings"
	"time"

	"github.com/rigerc/smol/internal/displayfmt"
)

// ErrUnavailable is returned by a module that has nothing to report on this
// system (for example, battery on a desktop). The renderer drops such modules
// silently.
var ErrUnavailable = errors.New("module unavailable")

// Env supplies all module inputs. Paths in FS are relative to the system root.
type Env struct {
	FS         fs.FS
	Getenv     func(string) string
	Now        func() time.Time
	Statfs     func(string) (DiskStats, error)
	Uname      func() (string, error)
	Interfaces func() ([]Interface, error)
	Display    *displayfmt.Options
	Options    *Options
}

// Interface is an address snapshot without platform dependencies.
type Interface struct {
	Name       string
	IPv4, IPv6 []string
}

// Field describes a named result field.
type Field struct{ Name, Description string }

// Result holds a module's default text and optional fields.
type Result struct {
	Text   string
	Fields map[string]string
}

// Module describes and fetches system information.
type Module struct {
	Name, Description string
	Fields            []Field
	Fetch             func(context.Context, Env) (Result, error)
}

// registry is constructed once and never mutated. Public accessors copy fields.
var registry = buildRegistry()

func buildRegistry() []Module {
	specs := []struct {
		name, desc, fields string
		fetch              func(context.Context, Env) (Result, error)
	}{
		{"colors", "Terminal 16-color palette", "", colors},
		{"cpu", "CPU model and logical threads", "threads cores freq temp", cpuModule{}.Fetch},
		{"host", "Hostname", "", hostModule{}.Fetch},
		{"kernel", "Kernel release", "arch release name", kernelModule{}.Fetch},
		{"load", "System load averages", "percent", loadModule{}.Fetch},
		{"mem", "Memory usage", "used free available total percent bar", memModule{}.Fetch},
		{"os", "Operating system", "id version", osModule{}.Fetch},
		{"shell", "Interactive shell", "", shellModule{}.Fetch},
		{"uptime", "System uptime", "days hours minutes seconds", uptimeModule{}.Fetch},
		{"user", "Current user", "", userModule{}.Fetch},
	}
	out := make([]Module, 0, len(specs)+12)
	for _, s := range specs {
		m := Module{Name: s.name, Description: s.desc}
		for _, f := range strings.Fields(s.fields) {
			m.Fields = append(m.Fields, Field{f, fieldDescription(s.name, f)})
		}
		m.Fetch = s.fetch
		out = append(out, m)
	}
	out = append(out, extraSpecs()...)
	slices.SortFunc(out, func(a, b Module) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// All returns independent descriptors sorted by name.
func All() []Module {
	out := slices.Clone(registry)
	for i := range out {
		out[i].Fields = slices.Clone(out[i].Fields)
	}
	return out
}

// Get returns a system module. Face is injected by app.
func Get(name string) (Module, bool) {
	for _, m := range registry {
		if m.Name == name {
			m.Fields = slices.Clone(m.Fields)
			return m, true
		}
	}
	return Module{}, false
}

// Names includes system modules and the configured face.
func Names() []string {
	out := []string{"face"}
	for _, m := range All() {
		out = append(out, m.Name)
	}
	slices.Sort(out)
	return out
}

// Known reports whether a module name is supported.
func Known(name string) bool {
	if name == "face" {
		return true
	}
	_, ok := Get(name)
	return ok
}
func display(env Env) displayfmt.Options {
	if env.Display != nil {
		return *env.Display
	}
	return displayfmt.Default()
}

func fieldDescription(module, field string) string {
	switch field {
	case "percent":
		return "Usage percentage"
	case "bar":
		return "Usage bar"
	case "freq":
		return "Maximum CPU frequency"
	case "temp":
		return "Maximum CPU sensor temperature"
	case "arch":
		return "Machine architecture"
	case "ipv6":
		return "First IPv6 address"
	case "interface":
		return "Network interface name"
	}
	return module + " " + field
}
