package main

import (
	"encoding/json"
	"os/exec"
	"testing"
)

// Versioned go install rejects main-module replacements and exclusions even
// when local builds and installs pass.
func TestVersionedInstallManifest(t *testing.T) {
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = "../.."
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Replace []json.RawMessage
		Exclude []json.RawMessage
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Replace) != 0 || len(manifest.Exclude) != 0 {
		t.Fatal("go.mod must not use replace or exclude directives: they prevent versioned go install")
	}
}
