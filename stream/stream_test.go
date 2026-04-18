// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package stream

import (
	"os"
	"testing"
)

const testInstrumentYAML = `instruments:
  - name: desert_bass
    group: bass
    variants:
      - name: default
        op1: { a: 0, d: 8, s: 10, r: 4, l: 20, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 0, d: 6, s: 12, r: 8, l: 5, m: 2, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 4
        connection: 0
      - name: bright
        op1: { a: 0, d: 4, s: 6, r: 2, l: 10, m: 4, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 0, d: 4, s: 8, r: 4, l: 3, m: 4, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 2
        connection: 0
  - name: electric_guitar
    group: lead
    variants:
      - name: clean
        op1: { a: 0, d: 10, s: 14, r: 6, l: 30, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 0, d: 8, s: 12, r: 4, l: 5, m: 2, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 6
        connection: 0
`

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

func TestLoadInstrumentsFromYAML(t *testing.T) {
	s := New(44100)
	defer s.Close()

	path := writeTestInstrumentFile(t)
	if err := LoadInstrumentsFromYAML(s, path); err != nil {
		t.Fatalf("LoadInstrumentsFromYAML: %v", err)
	}
	if _, err := s.Voices().GetInstrument("desert_bass.default"); err != nil {
		t.Fatalf("expected desert_bass.default to load: %v", err)
	}
	if _, err := s.Voices().GetInstrument("desert_bass.bright"); err != nil {
		t.Fatalf("expected desert_bass.bright to load: %v", err)
	}
	if _, err := s.Voices().GetInstrument("electric_guitar.clean"); err != nil {
		t.Fatalf("expected electric_guitar.clean to load: %v", err)
	}
}

func TestLoadInstrumentsFromYAMLGroup(t *testing.T) {
	s := New(44100)
	defer s.Close()

	path := writeTestInstrumentFile(t)
	if err := LoadInstrumentsFromYAMLGroup(s, path, "bass"); err != nil {
		t.Fatalf("LoadInstrumentsFromYAMLGroup: %v", err)
	}
	if _, err := s.Voices().GetInstrument("desert_bass.default"); err != nil {
		t.Fatalf("expected desert_bass.default to load: %v", err)
	}
	if _, err := s.Voices().GetInstrument("electric_guitar.clean"); err == nil {
		t.Fatal("did not expect electric_guitar.clean to load for bass group")
	}
}

func TestLoadInstrumentsFromYAMLNilStream(t *testing.T) {
	path := writeTestInstrumentFile(t)
	if err := LoadInstrumentsFromYAML(nil, path); err == nil {
		t.Fatal("expected error for nil stream")
	}
}

func writeTestInstrumentFile(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/instruments.yaml"
	if err := os.WriteFile(path, []byte(testInstrumentYAML), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
