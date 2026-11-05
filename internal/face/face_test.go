package face

import (
	"reflect"
	"strings"
	"testing"

	faceroll "github.com/rigerc/faceroll/go"
)

func TestBuild(t *testing.T) {
	seed := "smol"
	empty := ""
	for _, options := range []Options{{State: "idle"}, {State: "happy", Preset: "kitty"}, {State: "idle", Seed: &empty}} {
		first, err := Build(options)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Build(options)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, second) || first.Still == "" || len(first.Frames) == 0 {
			t.Fatalf("invalid or unstable animation: %+v", first)
		}
	}
	implicit, err := Build(Options{State: "idle"})
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := Build(Options{State: "idle", Seed: &seed})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(implicit, explicit) {
		t.Fatal("implicit seed differs from smol")
	}
	preset, err := faceroll.CreatePresetCharacter("kitty", faceroll.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Build(Options{State: "happy", Preset: "kitty"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, preset.Animations[faceroll.StateHappy]) {
		t.Fatal("preset expression mismatch")
	}
}

func TestInvalidOptions(t *testing.T) {
	seed := "seed"
	for _, options := range []Options{{State: "nope"}, {State: ""}, {State: "idle", Preset: "nope"}, {State: "idle", Preset: "kitty", Seed: &seed}} {
		if _, err := Build(options); err == nil {
			t.Errorf("accepted %+v", options)
		}
	}
}

func TestDiscovery(t *testing.T) {
	for _, preset := range Presets() {
		if err := (Options{State: "idle", Preset: preset}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, state := range States() {
		if err := (Options{State: state}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	names := Presets()
	names[0] = "changed"
	if Presets()[0] == "changed" {
		t.Fatal("mutated upstream presets")
	}
}

func TestRandomIdentity(t *testing.T) {
	source := strings.NewReader(strings.Repeat("a", 32))
	options := Options{Random: true, State: "idle"}
	resolved, err := Resolve(options, source)
	if err != nil || resolved.Random || resolved.Seed == nil || *resolved.Seed != strings.Repeat("61", 16) || source.Len() != 16 {
		t.Fatalf("%#v %v", resolved, err)
	}
	a, err := Build(resolved)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(resolved)
	if err != nil || a.Still != b.Still {
		t.Fatalf("identity changed: %v", err)
	}
	empty := ""
	for _, invalid := range []Options{{Random: true, Seed: &empty, State: "idle"}, {Random: true, Preset: "kitty", State: "idle"}, {Random: true, State: "unknown"}} {
		if _, err := Resolve(invalid, source); err == nil {
			t.Fatalf("accepted %#v", invalid)
		}
	}
	if _, err := Resolve(options, strings.NewReader("short")); err == nil {
		t.Fatal("short entropy accepted")
	}
	if _, err := RandomSeed(nil); err == nil {
		t.Fatal("nil entropy accepted")
	}
	unchanged := Options{State: "idle"}
	got, err := Resolve(unchanged, nil)
	if err != nil || got.Random || got.Seed != nil {
		t.Fatalf("%#v %v", got, err)
	}
}
