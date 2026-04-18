// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package instrument

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jebbisson/spice-synth/adl"
	"github.com/jebbisson/spice-synth/op2"
	"github.com/jebbisson/spice-synth/voice"
)

func TestExtractFromOP2Bank(t *testing.T) {
	bank, err := op2.DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank: %v", err)
	}
	f := ExtractFromOP2Bank(bank)
	if f == nil {
		t.Fatal("ExtractFromOP2Bank returned nil")
	}
	if len(f.Instruments) == 0 {
		t.Fatal("expected extracted OP2 instruments")
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	totalVariants := 0
	for _, inst := range f.Instruments {
		totalVariants += len(inst.Variants)
		if inst.Group != extractedGroup {
			t.Fatalf("expected group %q, got %q", extractedGroup, inst.Group)
		}
	}
	if totalVariants >= 175 {
		t.Fatalf("expected duplicate collapse to reduce variant count below 175, got %d", totalVariants)
	}
	if totalVariants == 0 {
		t.Fatal("expected non-zero extracted variants")
	}
}

func TestExtractFromADL(t *testing.T) {
	path := filepath.Join("..", "examples", "adl", "DUNE1.ADL")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test ADL file not available: %v", err)
	}
	af, err := adl.ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	f := ExtractFromADL(af)
	if f == nil {
		t.Fatal("ExtractFromADL returned nil")
	}
	if len(f.Instruments) == 0 {
		t.Fatal("expected extracted ADL instruments")
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	for _, inst := range f.Instruments {
		if inst.Group != extractedGroup {
			t.Fatalf("expected group %q, got %q", extractedGroup, inst.Group)
		}
	}
}

func TestExtractFromADLFilesDedupesAcrossInputs(t *testing.T) {
	path := filepath.Join("..", "examples", "adl", "DUNE1.ADL")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test ADL file not available: %v", err)
	}
	first, err := adl.ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes first: %v", err)
	}
	second, err := adl.ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes second: %v", err)
	}

	single := ExtractFromADL(first)
	multi := ExtractFromADLFiles([]*adl.File{first, second})
	if len(multi.Instruments) != len(single.Instruments) {
		t.Fatalf("expected duplicate ADL inputs to dedupe to %d instruments, got %d", len(single.Instruments), len(multi.Instruments))
	}
	if err := multi.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestExtractCollapsesIdenticalConfigs(t *testing.T) {
	base := &voice.Instrument{
		Name:       "desert_bass",
		Op1:        voice.Operator{Attack: 1, Decay: 2, Sustain: 3, Release: 4, Level: 5, Multiply: 6},
		Op2:        voice.Operator{Attack: 7, Decay: 8, Sustain: 9, Release: 10, Level: 11, Multiply: 12},
		Feedback:   3,
		Connection: 1,
	}
	dup := *base
	dup.Name = "dune1_042"
	file := extractFromVoiceInstruments([]*voice.Instrument{base, &dup})
	if len(file.Instruments) != 1 {
		t.Fatalf("expected 1 grouped instrument, got %d", len(file.Instruments))
	}
	if len(file.Instruments[0].Variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(file.Instruments[0].Variants))
	}
	if file.Instruments[0].Variants[0].Name != "default" {
		t.Fatalf("expected first variant name default, got %q", file.Instruments[0].Variants[0].Name)
	}
	if file.Instruments[0].Name != "desert_bass" {
		t.Fatalf("expected grouped instrument name desert_bass, got %q", file.Instruments[0].Name)
	}
	if file.Instruments[0].DefaultNote != "C4" {
		t.Fatalf("expected default note C4, got %q", file.Instruments[0].DefaultNote)
	}
}

func TestExtractIgnoresOperatorLevelsInIdentity(t *testing.T) {
	base := &voice.Instrument{
		Name:       "lead_patch",
		Op1:        voice.Operator{Attack: 1, Decay: 2, Sustain: 3, Release: 4, Level: 5, Multiply: 6, KeyScaleRate: true, KeyScaleLevel: 1, Tremolo: true, Vibrato: true, Sustaining: true, Waveform: 2},
		Op2:        voice.Operator{Attack: 7, Decay: 8, Sustain: 9, Release: 10, Level: 11, Multiply: 12, KeyScaleRate: true, KeyScaleLevel: 2, Tremolo: true, Vibrato: true, Sustaining: true, Waveform: 3},
		Feedback:   3,
		Connection: 1,
	}
	levelOnlyDiff := *base
	levelOnlyDiff.Name = "lead_patch_alt"
	levelOnlyDiff.Op1.Level = 27
	levelOnlyDiff.Op2.Level = 42

	file := extractFromVoiceInstruments([]*voice.Instrument{base, &levelOnlyDiff})
	if len(file.Instruments) != 1 {
		t.Fatalf("expected 1 instrument after level-insensitive dedupe, got %d", len(file.Instruments))
	}
	if len(file.Instruments[0].Variants) != 1 {
		t.Fatalf("expected 1 variant after level-insensitive dedupe, got %d", len(file.Instruments[0].Variants))
	}
}
