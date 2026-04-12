// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package stream

import (
	"testing"
)

func TestNew(t *testing.T) {
	s := New(44100)
	if s == nil {
		t.Fatal("Expected Stream instance, got nil")
	}
	if s.sampleRate != 44100 {
		t.Errorf("Expected sample rate 44100, got %d", s.sampleRate)
	}
}

func TestRead(t *testing.T) {
	s := New(44100)
	b := make([]byte, 400) // 100 frames
	n, err := s.Read(b)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n != 400 {
		t.Errorf("Expected to read 400 bytes, got %d", n)
	}

	// Verify that the stream can handle different volume/gain settings without crashing
	s.SetMasterVolume(0.5)
	n, err = s.Read(b)
	if err != nil || n != 400 {
		t.Errorf("Read failed after volume change: %v", err)
	}
}
