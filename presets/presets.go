// Package presets embeds the shipped setup presets for installed binaries.
package presets

import "embed"

// files contains the authoritative preset definitions.
//
//go:embed *.toml
var files embed.FS

// Read returns one embedded TOML preset.
func Read(name string) ([]byte, error) { return files.ReadFile(name) }
