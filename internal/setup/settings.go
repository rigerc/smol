package setup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rigerc/smol/internal/config"
	"github.com/rigerc/smol/internal/modules"
)

// setting describes a persisted option using the same validation as config files.
type setting struct {
	key, title string
	choices    []string
	text       func(*config.Config) *string
	number     func(*config.Config) *int
	flag       func(*config.Config) *bool
}

func settings() []setting {
	return []setting{
		{key: "modules.colors.shape", title: "Colors: swatch shape", choices: []string{"blocks", "circles"}, text: func(c *config.Config) *string { return &c.Modules.Colors.Shape }},
		{key: "modules.disk.mount", title: "Disk: absolute mount path", text: func(c *config.Config) *string { return &c.Modules.Disk.Mount }},
		{key: "modules.time.format", title: "Time: Go layout (15:04, Mon 15:04, ...)", text: func(c *config.Config) *string { return &c.Modules.Time.Format }},
		{key: "modules.battery.device", title: "Battery: device (empty selects first battery)", text: func(c *config.Config) *string { return &c.Modules.Battery.Device }},
		{key: "modules.localip.interface", title: "Local IP: interface (empty selects automatically)", text: func(c *config.Config) *string { return &c.Modules.LocalIP.Interface }},
		{key: "modules.localip.family", title: "Local IP: address family", choices: []string{"ipv4", "ipv6"}, text: func(c *config.Config) *string { return &c.Modules.LocalIP.Family }},
		{key: "modules.packages.manager", title: "Packages: database", choices: []string{"auto", "dpkg", "pacman", "apk"}, text: func(c *config.Config) *string { return &c.Modules.Packages.Manager }},
		{key: "display.size.unit", title: "Size: unit system", choices: []string{"iec", "si"}, text: func(c *config.Config) *string { return &c.Display.Size.Unit }},
		{key: "display.size.digits", title: "Size: decimal digits (-1 automatic, 0–6)", number: func(c *config.Config) *int { return &c.Display.Size.Digits }},
		{key: "display.duration.abbreviation", title: "Duration: abbreviate units", flag: func(c *config.Config) *bool { return &c.Display.Duration.Abbreviation }},
		{key: "display.duration.space-before-unit", title: "Duration: space before unit", flag: func(c *config.Config) *bool { return &c.Display.Duration.SpaceBeforeUnit }},
		{key: "display.duration.compact", title: "Duration: omit zero units", flag: func(c *config.Config) *bool { return &c.Display.Duration.Compact }},
		{key: "display.duration.round", title: "Duration: round to nearest minute", flag: func(c *config.Config) *bool { return &c.Display.Duration.Round }},
		{key: "display.percent.digits", title: "Percentage: decimal digits (0–6)", number: func(c *config.Config) *int { return &c.Display.Percent.Digits }},
		{key: "display.percent.width", title: "Percentage: bar width (1–100)", number: func(c *config.Config) *int { return &c.Display.Percent.Width }},
		{key: "display.temp.unit", title: "Temperature: unit", choices: []string{"c", "f", "k"}, text: func(c *config.Config) *string { return &c.Display.Temp.Unit }},
		{key: "display.temp.digits", title: "Temperature: decimal digits (0–6)", number: func(c *config.Config) *int { return &c.Display.Temp.Digits }},
	}
}

func (f setting) value(c config.Config) string {
	switch {
	case f.text != nil:
		return *f.text(&c)
	case f.number != nil:
		return strconv.Itoa(*f.number(&c))
	default:
		return strconv.FormatBool(*f.flag(&c))
	}
}

func (f setting) apply(c *config.Config, value string) error {
	switch {
	case f.text != nil:
		*f.text(c) = value
	case f.number != nil:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s: enter a whole number", f.key)
		}
		*f.number(c) = n
	default:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s: choose true or false", f.key)
		}
		*f.flag(c) = b
	}
	return c.Validate()
}

func (s *session) settingsQuestion() question {
	switch s.state {
	case "settings.ready":
		labels := map[string]string{}
		if s.settingsExit == "review.ready" {
			labels["Continue"] = "Back to review"
		}
		return question{labels: labels, title: "Additional settings", description: "Configure module options, display formatting, custom format, or icon details.", choices: []string{"Continue", "Module options", "Display formatting", "Custom format", "Icon details"}, value: "Continue"}
	case "module.options", "display.options":
		prefix, title := "modules.", "Module options"
		if s.state == "display.options" {
			prefix, title = "display.", "Display formatting"
		}
		q := question{title: title, choices: []string{"Done"}, value: "Done", labels: map[string]string{}}
		for _, f := range settings() {
			if strings.HasPrefix(f.key, prefix) {
				q.choices = append(q.choices, f.key)
				q.labels[f.key] = f.title + ": " + f.value(s.draft)
			}
		}
		if prefix == "modules." {
			q.description = "Options take effect when the module is included in the format."
		}
		return q
	case "setting.value":
		for _, f := range settings() {
			if f.key != s.settingKey {
				continue
			}
			q := question{title: f.title, value: f.value(s.draft), choices: f.choices, allowEmpty: f.text != nil}
			if f.flag != nil {
				q.choices = []string{"true", "false"}
			}
			base := cloneDraft(s.draft)
			q.validate = func(value string) error {
				c := cloneDraft(base)
				return f.apply(&c, value)
			}
			return q
		}
	case "format.custom":
		return question{title: "Custom output format", description: "Use {module}, {module:field}, and {{ / }} for literal braces. Repeated modules and custom separators are supported.", value: s.draft.Format, allowEmpty: true, validate: config.ValidateFormat}
	case "icons.options":
		return question{title: "Icon details", choices: []string{"Done", "Separator", "Module overrides"}, value: "Done"}
	case "icons.separator":
		base := cloneDraft(s.draft)
		return question{title: "Icon separator", value: s.draft.Icons.Separator, allowEmpty: true, validate: func(value string) error {
			c := cloneDraft(base)
			c.Icons.Separator = value
			return c.Validate()
		}}
	case "icons.module":
		return question{title: "Choose an icon override", choices: append([]string{"Done"}, modules.Names()...), value: "Done"}
	case "icons.override":
		value := "Built-in icon"
		if glyph, ok := s.draft.Icons.Overrides[s.iconModule]; ok {
			value = "Custom icon"
			if glyph == "" {
				value = "Hide icon"
			}
		}
		return question{title: s.iconModule + " icon", choices: []string{"Built-in icon", "Hide icon", "Custom icon"}, value: value}
	case "icons.custom":
		base := cloneDraft(s.draft)
		name := s.iconModule
		return question{title: name + " custom icon", value: s.draft.Icons.Overrides[name], allowEmpty: true, validate: func(value string) error {
			c := cloneDraft(base)
			if c.Icons.Overrides == nil {
				c.Icons.Overrides = make(map[string]string)
			}
			c.Icons.Overrides[name] = value
			return c.Validate()
		}}
	}
	return question{}
}

func (s *session) acceptSettings(value string) error {
	switch s.state {
	case "settings.ready":
		switch value {
		case "Continue":
			s.state = s.settingsExit
			if s.state == "" {
				s.state = "icons.ready"
			}
		case "Module options":
			s.state = "module.options"
		case "Display formatting":
			s.state = "display.options"
		case "Custom format":
			s.state = "format.custom"
		case "Icon details":
			s.state = "icons.options"
		}
	case "module.options", "display.options":
		if value == "Done" {
			s.state = "settings.ready"
		} else {
			s.settingReturn, s.settingKey, s.state = s.state, value, "setting.value"
		}
	case "setting.value":
		for _, f := range settings() {
			if f.key == s.settingKey {
				next := cloneDraft(s.draft)
				if err := f.apply(&next, value); err != nil {
					return err
				}
				s.draft = next
				s.state = s.settingReturn
				return nil
			}
		}
		return fmt.Errorf("unknown setting %q", s.settingKey)
	case "format.custom":
		if err := config.ValidateFormat(value); err != nil {
			return err
		}
		s.draft.Format = value
		s.derive()
		s.queue = nil
		for _, name := range s.selected {
			m, _ := modules.Get(name)
			if len(m.Fields) > 0 {
				s.queue = append(s.queue, name)
			}
		}
		s.index = 0
		s.state = "settings.ready"
	case "icons.options":
		switch value {
		case "Done":
			s.state = "settings.ready"
		case "Separator":
			s.state = "icons.separator"
		case "Module overrides":
			s.state = "icons.module"
		}
	case "icons.separator":
		s.draft.Icons.Separator = value
		s.state = "icons.options"
	case "icons.module":
		if value == "Done" {
			s.state = "icons.options"
		} else {
			s.iconModule, s.state = value, "icons.override"
		}
	case "icons.override":
		switch value {
		case "Built-in icon":
			delete(s.draft.Icons.Overrides, s.iconModule)
			s.state = "icons.module"
		case "Hide icon":
			s.setIconOverride("")
			s.state = "icons.module"
		case "Custom icon":
			s.state = "icons.custom"
		}
	case "icons.custom":
		s.setIconOverride(value)
		s.state = "icons.module"
	default:
		s.acceptStyle(value)
	}
	return nil
}

func (s *session) setIconOverride(value string) {
	if s.draft.Icons.Overrides == nil {
		s.draft.Icons.Overrides = make(map[string]string)
	}
	s.draft.Icons.Overrides[s.iconModule] = value
}
