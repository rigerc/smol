// Package render turns a format template into the final one-line output.
package render

import (
	"fmt"
	"strings"
	"unicode"
)

// Token is one piece of a parsed format: either literal text or a module
// reference. Exactly one of the fields is set.
type Token struct {
	Literal string
	Module  string
}

// Parse splits a format string such as "{os} · {kernel}" into tokens.
// "{{" and "}}" produce literal braces.
func Parse(format string) ([]Token, error) {
	for _, r := range format {
		if unicode.IsControl(r) {
			return nil, fmt.Errorf("format: control character %U is not allowed", r)
		}
	}
	var (
		toks []Token
		lit  strings.Builder
	)
	flush := func() {
		if lit.Len() > 0 {
			toks = append(toks, Token{Literal: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(format); i++ {
		c := format[i]
		switch c {
		case '{':
			if i+1 < len(format) && format[i+1] == '{' {
				lit.WriteByte('{')
				i++
				continue
			}
			end := strings.IndexByte(format[i+1:], '}')
			if end < 0 {
				return nil, fmt.Errorf("format: unclosed '{' at position %d", i)
			}
			name := strings.TrimSpace(format[i+1 : i+1+end])
			if name == "" {
				return nil, fmt.Errorf("format: empty module name at position %d", i)
			}
			if strings.ContainsAny(name, "{ \t") {
				return nil, fmt.Errorf("format: invalid module name %q at position %d", name, i)
			}
			flush()
			toks = append(toks, Token{Module: name})
			i += end + 1
		case '}':
			if i+1 < len(format) && format[i+1] == '}' {
				lit.WriteByte('}')
				i++
				continue
			}
			return nil, fmt.Errorf("format: unmatched '}' at position %d", i)
		default:
			lit.WriteByte(c)
		}
	}
	flush()
	return toks, nil
}

// ModuleNames returns the distinct module names referenced by toks, in order
// of first appearance.
func ModuleNames(toks []Token) []string {
	seen := map[string]bool{}
	var names []string
	for _, t := range toks {
		if t.Module != "" && !seen[t.Module] {
			seen[t.Module] = true
			names = append(names, t.Module)
		}
	}
	return names
}
