// Package face validates and builds faceroll characters without process I/O.
package face

import (
	"encoding/hex"
	"errors"
	"io"

	faceroll "github.com/rigerc/faceroll/go"
)

// Options selects a deterministic or random character identity and expression.
type Options struct {
	Random bool    `toml:"random,omitempty"`
	Preset string  `toml:"preset,omitempty"`
	Seed   *string `toml:"seed,omitempty"`
	State  string  `toml:"state"`
}

// Validate checks identifiers even when the format does not reference face.
func (o Options) Validate() error {
	if o.Random && (o.Preset != "" || o.Seed != nil) {
		return errors.New("face: random, preset and seed are mutually exclusive")
	}
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
	if o.Random {
		return faceroll.Animation{}, errors.New("face: resolve random identity before building")
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

// Resolve samples a random identity once, leaving deterministic options unchanged.
func Resolve(o Options, entropy io.Reader) (Options, error) {
	if err := o.Validate(); err != nil {
		return o, err
	}
	if !o.Random {
		return o, nil
	}
	seed, err := RandomSeed(entropy)
	if err != nil {
		return o, err
	}
	o.Random, o.Seed = false, &seed
	return o, nil
}

// RandomSeed reads 128 bits of injected entropy, with no deterministic fallback.
func RandomSeed(entropy io.Reader) (string, error) {
	if entropy == nil {
		return "", errors.New("face: entropy source unavailable")
	}
	var data [16]byte
	if _, err := io.ReadFull(entropy, data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
