// Package face validates and builds faceroll characters without process I/O.
package face

import (
	"errors"

	faceroll "github.com/rigerc/faceroll/go"
)

// Options selects a preset or a repeatable generated character and expression.
type Options struct {
	Preset string  `toml:"preset"`
	Seed   *string `toml:"seed"`
	State  string  `toml:"state"`
}

// Validate checks identifiers even when the format does not reference face.
func (o Options) Validate() error {
	if o.Preset != "" && o.Seed != nil {
		return errors.New("face: preset and seed are mutually exclusive")
	}
	if o.Preset != "" {
		if _, err := faceroll.GetFacePreset(o.Preset); err != nil {
			return err
		}
	}
	_, err := faceroll.GetState(o.State)
	return err
}

// Build creates one animation; an absent seed defaults to the stable seed smol.
func Build(o Options) (faceroll.Animation, error) {
	if err := o.Validate(); err != nil {
		return faceroll.Animation{}, err
	}
	var character *faceroll.Character
	var err error
	if o.Preset != "" {
		character, err = faceroll.CreatePresetCharacter(o.Preset, faceroll.CreateOptions{})
	} else {
		seed := "smol"
		if o.Seed != nil {
			seed = *o.Seed
		}
		character, err = faceroll.Roll(faceroll.RollOptions{Seed: &seed})
	}
	if err != nil {
		return faceroll.Animation{}, err
	}
	return character.Animations[faceroll.StateName(o.State)], nil
}

// Presets lists the upstream preset identifiers without modifying its tables.
func Presets() []string {
	names := make([]string, len(faceroll.FacePresets))
	for i, preset := range faceroll.FacePresets {
		names[i] = string(preset.ID)
	}
	return names
}

// States lists the upstream expression identifiers.
func States() []string {
	states := faceroll.StateNames()
	names := make([]string, len(states))
	for i, state := range states {
		names[i] = string(state)
	}
	return names
}
