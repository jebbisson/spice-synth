// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package voice

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	// We can't easily create a real chip here without more setup,
	// but we can check if the manager initializes correctly.
	// For now, let's assume we have a nil or dummy pointer.
	m := NewManager(nil, 44100)
	if m == nil {
		t.Fatal("Expected Manager instance, got nil")
	}
	if len(m.channels) != 9 {
		t.Errorf("Expected 9 channels, got %d", len(m.channels))
	}
}

func TestNoteOn_InvalidChannel(t *testing.T) {
	m := NewManager(nil, 44100)
	err := m.NoteOn(10, 60, nil)
	if err == nil {
		t.Error("Expected error for invalid channel, got nil")
	}
}

func TestNoteOn_NilInstrument(t *testing.T) {
	m := NewManager(nil, 44100)
	err := m.NoteOn(0, 60, nil)
	if err == nil {
		t.Error("Expected error for nil instrument, got nil")
	}
}

func TestInstrumentOverrideApplyTo(t *testing.T) {
	base := &Instrument{
		Name: "base",
		Op1:  Operator{Level: 20, Attack: 3},
		Op2:  Operator{Level: 5, Attack: 4},
		Feedback:   2,
		Connection: 0,
	}
	level := uint8(11)
	attack := uint8(15)
	feedback := uint8(6)

	override := &InstrumentOverride{
		Op2:      OperatorOverride{Level: &level, Attack: &attack},
		Feedback: &feedback,
	}

	got := override.ApplyTo(base)
	if got == nil {
		t.Fatal("ApplyTo returned nil")
	}
	if got == base {
		t.Fatal("ApplyTo should clone, not mutate in place")
	}
	if got.Op2.Level != 11 || got.Op2.Attack != 15 || got.Feedback != 6 {
		t.Fatalf("override not applied correctly: %+v", got)
	}
	if base.Op2.Level != 5 || base.Op2.Attack != 4 || base.Feedback != 2 {
		t.Fatalf("base instrument was mutated: %+v", base)
	}
}
