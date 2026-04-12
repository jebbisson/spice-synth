// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package chip

import (
	"testing"
)

func TestNewOPL3(t *testing.T) {
	o := New(44100)
	if o == nil {
		t.Fatal("Expected OPL3 instance, got nil")
	}
}

func TestWriteRegister(t *testing.T) {
	o := New(44100)
	// Testing that writing doesn't crash
	o.WriteRegister(0, 0x20, 0xFF)
}

func TestGenerateSamples(t *testing.T) {
	o := New(44100)
	samples, err := o.GenerateSamples(100)
	if err != nil {
		t.Fatalf("GenerateSamples failed: %v", err)
	}
	// Stereo means 2 * n samples
	expectedLen := 200
	if len(samples) != expectedLen {
		t.Errorf("Expected %d samples, got %d", expectedLen, len(samples))
	}
}

func TestReset(t *testing.T) {
	o := New(44100)
	o.WriteRegister(0, 0x20, 0xFF)
	o.Reset()
	// We can't easily verify internal C state without more complex mocks,
	// but we ensure it doesn't crash.
}
