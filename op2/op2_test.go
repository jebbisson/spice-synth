// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package op2

import (
	"bytes"
	"testing"
)

func TestDefaultBank(t *testing.T) {
	bank, err := DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank() error: %v", err)
	}

	// Should have non-empty melodic instruments.
	nonEmpty := 0
	for i := 0; i < numMelodic; i++ {
		if bank.Melodic[i].Name != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Fatal("no melodic instruments have names")
	}
	t.Logf("found %d named melodic instruments", nonEmpty)

	// Should have non-empty percussion instruments.
	percNonEmpty := 0
	for i := 0; i < numPercussion; i++ {
		if bank.Percussion[i].Name != "" {
			percNonEmpty++
		}
	}
	if percNonEmpty == 0 {
		t.Fatal("no percussion instruments have names")
	}
	t.Logf("found %d named percussion instruments", percNonEmpty)
}

func TestMelodicInstrument(t *testing.T) {
	bank, err := DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank() error: %v", err)
	}

	// Program 0 should be some kind of piano.
	inst := bank.MelodicInstrument(0)
	if inst == nil {
		t.Fatal("MelodicInstrument(0) returned nil")
	}
	t.Logf("program 0: %s", inst.Name)

	// Program 38 should be some kind of synth bass.
	inst38 := bank.MelodicInstrument(38)
	if inst38 == nil {
		t.Fatal("MelodicInstrument(38) returned nil")
	}
	t.Logf("program 38: %s", inst38.Name)

	// Out-of-range should fall back to 0.
	instOOB := bank.MelodicInstrument(200)
	if instOOB == nil {
		t.Fatal("MelodicInstrument(200) returned nil")
	}
	if instOOB.Name != inst.Name {
		t.Errorf("out-of-range instrument name = %s, want %s", instOOB.Name, inst.Name)
	}
}

func TestPercussionInstrument(t *testing.T) {
	bank, err := DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank() error: %v", err)
	}

	// Note 36 = Bass Drum in GM.
	inst := bank.PercussionInstrument(36)
	if inst == nil {
		t.Fatal("PercussionInstrument(36) returned nil")
	}
	t.Logf("perc note 36: %s", inst.Name)

	// Note 38 = Snare.
	inst38 := bank.PercussionInstrument(38)
	if inst38 == nil {
		t.Fatal("PercussionInstrument(38) returned nil")
	}
	t.Logf("perc note 38: %s", inst38.Name)
}

func TestToInstrument(t *testing.T) {
	bank, err := DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank() error: %v", err)
	}

	// Verify that ToInstrument produces valid OPL2 register values.
	inst := bank.MelodicInstrument(0)
	if inst.Op1.Attack > 15 {
		t.Errorf("modulator attack %d > 15", inst.Op1.Attack)
	}
	if inst.Op1.Decay > 15 {
		t.Errorf("modulator decay %d > 15", inst.Op1.Decay)
	}
	if inst.Op2.Level > 63 {
		t.Errorf("carrier level %d > 63", inst.Op2.Level)
	}
	if inst.Feedback > 7 {
		t.Errorf("feedback %d > 7", inst.Feedback)
	}
	if inst.Connection > 1 {
		t.Errorf("connection %d > 1", inst.Connection)
	}
}

func TestDoubleVoice(t *testing.T) {
	bank, err := DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank() error: %v", err)
	}

	// Find a double-voice instrument.
	found := false
	for i := 0; i < numMelodic; i++ {
		if bank.Melodic[i].DoubleVoice {
			insts := bank.Melodic[i].ToInstruments()
			if len(insts) != 2 {
				t.Errorf("double-voice instrument %d returned %d instruments, want 2", i, len(insts))
			}
			t.Logf("found double-voice: program %d (%s)", i, bank.Melodic[i].Name)
			found = true
			break
		}
	}
	if !found {
		t.Log("no double-voice instruments found in bank (this is OK for vanilla banks)")
	}
}

func TestInvalidHeader(t *testing.T) {
	data := make([]byte, 12000)
	copy(data, "INVALID!")
	_, err := Load(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for invalid header")
	}
}

func TestFileTooSmall(t *testing.T) {
	data := []byte("#OPL_II#")
	_, err := Load(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for too-small file")
	}
}

func TestAllInstrumentsConvert(t *testing.T) {
	bank, err := DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank() error: %v", err)
	}

	// Every instrument should convert without panic.
	for i := uint8(0); i < numMelodic; i++ {
		inst := bank.MelodicInstrument(i)
		if inst == nil {
			t.Errorf("MelodicInstrument(%d) returned nil", i)
		}
	}
	for note := uint8(35); note <= 81; note++ {
		inst := bank.PercussionInstrument(note)
		if inst == nil {
			t.Errorf("PercussionInstrument(%d) returned nil", note)
		}
	}
}
