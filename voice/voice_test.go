// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

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
