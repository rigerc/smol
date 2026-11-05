package config

import (
	"github.com/rigerc/smol/internal/modules"
	"reflect"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	vars := map[string]string{"HOME": "/home/user", "XDG_CONFIG_DIRS": "/one:/two"}
	getenv := func(k string) string { return vars[k] }
	got := Search(getenv, func(p string) bool { return strings.HasPrefix(p, "/one") })
	want := []Candidate{{"/home/user/.config/smol/config.toml", false}, {"/one/smol/config.toml", true}, {"/two/smol/config.toml", false}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	vars["XDG_CONFIG_HOME"] = "/custom"
	delete(vars, "XDG_CONFIG_DIRS")
	got = Search(getenv, func(string) bool { return false })
	if got[0].Path != "/custom/smol/config.toml" || got[1].Path != "/etc/xdg/smol/config.toml" {
		t.Fatal(got)
	}
}
func TestExpandedConfiguration(t *testing.T) {
	body := `format="{mem:percent} {cpu:threads} {face}"
[display.size]
unit="si"
digits=2
[display.duration]
abbreviation=false
compact=true
[display.percent]
digits=1
width=12
[display.temp]
unit="f"
digits=1
[modules.disk]
mount="/home"
[modules.time]
format="Mon 15:04"
[modules.battery]
device="BAT1"
[modules.localip]
interface="eth0"
family="ipv6"
[modules.packages]
manager="apk"
`
	c, err := Parse("test", []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	printed, err := TOML(c)
	if err != nil {
		t.Fatal(err)
	}
	round, err := Parse("roundtrip", []byte(printed))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, round) {
		t.Fatalf("round trip differs: %+v vs %+v\n%s", c, round, printed)
	}
	if c.Display.Duration.Abbreviation {
		t.Fatal("explicit false lost")
	}
	o := c.Modules.Options()
	if o.Disk.Mount != "/home" || o.Packages.Manager != "apk" || o.LocalIP.Family != "ipv6" || o.LocalIP.Interface != "eth0" || o.Battery.Device != "BAT1" || o.Time.Format != "Mon 15:04" {
		t.Fatal(o)
	}
	printed, err = TOML(Default())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(printed, "[modules.disk]") {
		t.Fatal(printed)
	}
	if _, err := Parse("defaults", []byte(printed)); err != nil {
		t.Fatal(err)
	}
}
func TestExpandedConfigErrors(t *testing.T) {
	for _, body := range []string{`format="{face:anything}"`, `format="{mem:unknown}"`, `format="{mem:}"`, `format="{mem:percent:extra}"`, "[modules.diks]\nmount='/'", "[modules.disk]\nunknown=true", "[modules.disk]\nmount='relative'", "[modules.localip]\nfamily='bad'", "[modules.battery]\ndevice='../BAT0'", "[modules.packages]\nmanager='rpm'", "[modules.packages]\nmanager='bad'", "[display.temp]\nunit='bad'"} {
		if _, err := Parse("test", []byte(body)); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
}

func TestPresetsAndRegisteredFields(t *testing.T) {
	for _, name := range []string{"banner", "prompt", "server", "minimal"} {
		if _, err := Load("../../presets/"+name+".toml", true); err != nil {
			t.Fatal(name, err)
		}
	}
	for _, m := range modules.All() {
		for _, f := range m.Fields {
			if err := ValidateFormat("{" + m.Name + ":" + f.Name + "}"); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestModuleDefaults(t *testing.T) {
	for _, body := range []string{"", "[modules.disk]\n[modules.time]\n[modules.battery]\n[modules.localip]\n[modules.packages]"} {
		c, err := Parse("defaults", []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if c.Modules.Disk == nil || c.Modules.Time == nil || c.Modules.Battery == nil || c.Modules.LocalIP == nil || c.Modules.Packages == nil {
			t.Fatal(c.Modules)
		}
		if c.Modules.Options() != modules.DefaultOptions() {
			t.Fatal(c.Modules.Options())
		}
		printed, err := TOML(c)
		if err != nil || strings.Contains(printed, "[modules.disk]") {
			t.Fatal(printed, err)
		}
	}
	c, err := Parse("partial", []byte("[modules.localip]\ninterface='eth0'"))
	if err != nil || c.Modules.LocalIP.Family != "ipv4" {
		t.Fatal(c, err)
	}
	a, b := Default(), Default()
	a.Modules.Disk.Mount = "/other"
	if b.Modules.Disk.Mount != "/" {
		t.Fatal("shared default pointers")
	}
	for _, body := range []string{"[modules.disk]\nmount=''", "[modules.time]\nformat=''", "[modules.localip]\nfamily=''", "[modules.packages]\nmanager=''"} {
		if _, err := Parse("empty", []byte(body)); err == nil {
			t.Fatal(body)
		}
	}
	if _, err := Parse("selectors", []byte("[modules.battery]\ndevice=''\n[modules.localip]\ninterface=''")); err != nil {
		t.Fatal(err)
	}
}
