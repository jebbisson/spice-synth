// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package sequencer

import (
	"testing"
)

func TestNewSequencer(t *testing.T) {
	seq := New(nil, 120.0, 44100)
	if seq == nil {
		t.Fatal("Expected Sequencer instance, got nil")
	}
	if seq.bpm != 120.0 {
		t.Errorf("Expected BPM 120.0, got %f", seq.bpm)
	}
}

func TestSetBPM(t *testing.T) {
	seq := New(nil, 120.0, 44100)
	seq.SetBPM(140.0)
	if seq.bpm != 140.0 {
		t.Errorf("Expected BPM 140.0, got %f", seq.bpm)
	}
}

func TestSetPattern(t *testing.T) {
	seq := New(nil, 120.0, 44100)
	p := NewPattern(16)
	seq.SetPattern(0, p)
	if seq.tracks[0] != p {
		t.Error("Expected pattern to be set for channel 0")
	}
}
