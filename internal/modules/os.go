package modules

import (
	"bufio"
	"context"
	"errors"
	"io/fs"
	"strings"
)

type osModule struct{}

func (osModule) Fetch(_ context.Context, env Env) (Result, error) {
	var data []byte
	var err error
	for _, p := range []string{"etc/os-release", "etc/lsb-release", "usr/lib/os-release"} {
		data, err = fs.ReadFile(env.FS, p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return Result{}, err
	}
	kv := parseOSRelease(string(data))
	if kv["DISTRIB_DESCRIPTION"] != "" {
		return coreResult("os", kv["DISTRIB_DESCRIPTION"], data, env), nil
	}
	if kv["DISTRIB_ID"] != "" {
		return coreResult("os", strings.TrimSpace(kv["DISTRIB_ID"]+" "+kv["DISTRIB_RELEASE"]), data, env), nil
	}
	if v := kv["PRETTY_NAME"]; v != "" {
		return coreResult("os", v, data, env), nil
	}
	if name := kv["NAME"]; name != "" {
		if ver := kv["VERSION"]; ver != "" {
			return coreResult("os", name+" "+ver, data, env), nil
		}
		return coreResult("os", name, data, env), nil
	}
	return Result{}, errors.New("os-release has no NAME or PRETTY_NAME")
}

// parseOSRelease parses the KEY=value format used by os-release(5).
// Values may be unquoted, single-quoted, or double-quoted.
func parseOSRelease(s string) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = unquote(strings.TrimSpace(v))
	}
	return out
}

func unquote(v string) string {
	if len(v) >= 2 {
		q := v[0]
		if (q == '"' || q == '\'') && v[len(v)-1] == q {
			v = v[1 : len(v)-1]
			if q == '"' {
				// os-release allows backslash escapes inside double quotes.
				r := strings.NewReplacer(`\"`, `"`, `\\`, `\`, "\\`", "`", `\$`, `$`)
				v = r.Replace(v)
			}
		}
	}
	return v
}
