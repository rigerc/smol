package setup

import (
	"fmt"
	"strings"

	"github.com/rigerc/smol/internal/icons"
	"github.com/rigerc/smol/internal/render"
)

func (s *session) styleQuestion() question {
	switch s.state {
	case "icons.ready":
		return question{title: "Output icons (Nerd Font glyphs require a compatible terminal font)", label: "Icons", choices: []string{"never", "auto", "always"}, value: s.draft.Icons.Mode}
	case "icons.set":
		return question{title: "Icon set — Nerd Font sample: " + icons.Default().Prefix("os", "") + icons.Default().Prefix("cpu", "") + icons.Default().Prefix("mem", "") + "; ASCII sample: OS CPU MEM", label: "Icon set", choices: []string{"nerd-font", "ascii"}, value: s.draft.Icons.Set}
	case "style.module":
		return question{title: "Choose a module to style", choices: append(append([]string{}, s.selected...), "Done"), value: "Done"}
	case "style.property":
		return question{title: s.selected[s.index] + " style", choices: []string{"Foreground", "Background", "Attributes", "Done"}, value: "Done"}
	case "style.color":
		st := s.draft.Style[s.selected[s.index]]
		value := st.FG
		if s.styleProperty == "Background" {
			value = st.BG
		}
		choices := append([]string{"terminal default"}, render.ColorNames()...)
		choices = append(choices, "Custom color")
		if value == "" {
			value = "terminal default"
		} else if strings.HasPrefix(value, "#") || strings.HasPrefix(value, "index:") {
			value = "Custom color"
		}
		return question{title: s.selected[s.index] + " " + strings.ToLower(s.styleProperty), choices: choices, value: value}
	case "style.custom":
		st := s.draft.Style[s.selected[s.index]]
		value := st.FG
		if s.styleProperty == "Background" {
			value = st.BG
		}
		return question{title: "Color: name, index:0..255, #RRGGBB, or empty for terminal default", value: value, validate: func(v string) error {
			if !render.ValidColor(v) {
				return fmt.Errorf("use a color name, index:0..255 or #RRGGBB")
			}
			return nil
		}}
	case "style.attributes":
		return question{title: s.selected[s.index] + " attributes", choices: []string{"bold", "dim", "italic", "underline", "reverse"}, checked: styleAttributes(s.draft.Style[s.selected[s.index]]), multiple: true}
	}
	return question{}
}

func styleAttributes(st render.Style) []string {
	var out []string
	for _, attr := range []struct {
		name    string
		enabled bool
	}{{"bold", st.Bold}, {"dim", st.Dim}, {"italic", st.Italic}, {"underline", st.Underline}, {"reverse", st.Reverse}} {
		if attr.enabled {
			out = append(out, attr.name)
		}
	}
	return out
}

func (s *session) acceptStyle(value string) {
	switch s.state {
	case "icons.ready":
		s.draft.Icons.Mode = value
		s.state = "icons.set"
	case "icons.set":
		s.draft.Icons.Set = value
		s.state = "color.ready"
	case "style.module":
		if value == "Done" {
			s.state = "shell.ready"
			return
		}
		for i, name := range s.selected {
			if name == value {
				s.index = i
			}
		}
		s.state = "style.property"
	case "style.property":
		s.styleProperty = value
		switch value {
		case "Done":
			s.state = "style.module"
		case "Attributes":
			s.state = "style.attributes"
		default:
			s.state = "style.color"
		}
	case "style.color":
		if value == "Custom color" {
			s.state = "style.custom"
			return
		}
		if value == "terminal default" {
			value = ""
		}
		s.setStyleColor(value)
		s.state = "style.property"
	case "style.custom":
		s.setStyleColor(value)
		s.state = "style.property"
	case "style.attributes":
		attrs := strings.Split(value, ",")
		st := s.draft.Style[s.selected[s.index]]
		st.Bold = contains(attrs, "bold")
		st.Dim = contains(attrs, "dim")
		st.Italic = contains(attrs, "italic")
		st.Underline = contains(attrs, "underline")
		st.Reverse = contains(attrs, "reverse")
		s.draft.Style[s.selected[s.index]] = st
		s.state = "style.property"
	}
}

func (s *session) setStyleColor(value string) {
	name := s.selected[s.index]
	st := s.draft.Style[name]
	if s.styleProperty == "Background" {
		st.BG = value
	} else {
		st.FG = value
	}
	s.draft.Style[name] = st
}
