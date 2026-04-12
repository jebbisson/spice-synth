// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package patches

import (
	"github.com/jebbisson/spice-synth/voice"
	"testing"
)

func TestSpicePatches(t *testing.T) {
	patches := Spice()
	if len(patches) == 0 {
		t.Fatal("Expected at least one patch in Spice bank")
	}
	var _ *voice.Instrument = patches[0]
	if patches[0].Name != "desert_bass" {
		t.Errorf("Expected desert_bass, got %s", patches[0].Name)
	}
}

func TestGMPatches(t *testing.T) {
	patches := GM()
	if len(patches) == 0 {
		t.Fatal("Expected at least one patch in GM bank")
	}
}
