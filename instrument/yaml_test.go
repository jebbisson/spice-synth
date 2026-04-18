// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package instrument_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jebbisson/spice-synth/instrument"
)

const sampleYAML = `instruments:
  - name: desert_bass
    group: bass
    variants:
      - name: default
        op1: { d: 8, s: 10, r: 4, l: 20, m: 1 }
        op2: { d: 6, s: 12, r: 8, l: 5, m: 2 }
        feedback: 4
        connection: 0
      - name: bright
        op1: { d: 4, s: 6, r: 2, l: 10, m: 4 }
        op2: { d: 4, s: 8, r: 4, l: 3, m: 4 }
        feedback: 2
        connection: 0
  - name: electric_guitar
    group: lead
    variants:
      - name: clean
        op1: { d: 10, s: 14, r: 6, l: 30, m: 1 }
        op2: { d: 8, s: 12, r: 4, l: 5, m: 2 }
        feedback: 6
        connection: 0
`

func TestLoadFileBytes(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}
	if len(f.Instruments) != 2 {
		t.Errorf("expected 2 instruments, got %d", len(f.Instruments))
	}

	desert := f.Instruments[0]
	if desert.Name != "desert_bass" {
		t.Errorf("expected name 'desert_bass', got %q", desert.Name)
	}
	if desert.Group != "bass" {
		t.Errorf("expected group 'bass', got %q", desert.Group)
	}
	if len(desert.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(desert.Variants))
	}
}

func TestToInstrument(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	v, err := f.Resolve("desert_bass")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v.Name != "desert_bass.default" {
		t.Errorf("expected 'desert_bass.default', got %q", v.Name)
	}
	if v.Op1.Attack != 0 {
		t.Errorf("expected Op1.Attack 0, got %d", v.Op1.Attack)
	}
	if v.Op2.Multiply != 2 {
		t.Errorf("expected Op2.Multiply 2, got %d", v.Op2.Multiply)
	}
}

func TestToInstrumentDef(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	v, err := f.Resolve("desert_bass")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	def := instrument.ToInstrumentDef(v)
	if def == nil {
		t.Fatal("ToInstrumentDef returned nil")
	}
	if def.Op1.Attack != 0 {
		t.Errorf("expected Op1.Attack 0, got %d", def.Op1.Attack)
	}
	if def.Op2.Level != 5 {
		t.Errorf("expected Op2.Level 5, got %d", def.Op2.Level)
	}
	if def.Feedback != 4 {
		t.Errorf("expected Feedback 4, got %d", def.Feedback)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// Load from sample YAML
	f1, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	// Save to temp file
	tmpFile := t.TempDir() + "/test.yaml"
	if err := instrument.SaveFile(tmpFile, f1); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	// Load back
	f2, err := instrument.LoadFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// Compare instrument counts
	if len(f2.Instruments) != len(f1.Instruments) {
		t.Errorf("expected %d instruments after round-trip, got %d", len(f1.Instruments), len(f2.Instruments))
	}

	// Compare first instrument's first variant
	v1 := f1.Instruments[0].Variants[0]
	v2 := f2.Instruments[0].Variants[0]
	if v1.Name != v2.Name {
		t.Errorf("variant name mismatch: %q vs %q", v1.Name, v2.Name)
	}
	if v1.Op1.Attack != v2.Op1.Attack || v1.Op2.Level != v2.Op2.Level {
		t.Errorf("operator params mismatch after round-trip")
	}
}

func TestVariantKey(t *testing.T) {
	f := &instrument.File{}
	key := f.VariantKey("desert_bass", "default")
	if key != "desert_bass.default" {
		t.Errorf("expected 'desert_bass.default', got %q", key)
	}
}

func TestFindByVariantKey(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	// Find existing variant
	v, err := f.FindByVariantKey("desert_bass.bright")
	if err != nil {
		t.Fatalf("FindByVariantKey: %v", err)
	}
	if v.Name != "desert_bass.bright" {
		t.Errorf("expected full variant key name, got %q", v.Name)
	}
	if v.Op1.Decay != 4 {
		t.Errorf("expected Op1.Decay 4, got %d", v.Op1.Decay)
	}

	// Find non-existing variant
	_, err = f.FindByVariantKey("desert_bass.nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent variant")
	}

	// Invalid key format
	_, err = f.FindByVariantKey("invalidkey")
	if err == nil {
		t.Error("expected error for invalid key format")
	}
}

func TestGroups(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	groups := f.Groups()
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestLoadGroup(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	grouped := f.LoadGroup("bass")
	if len(grouped.Instruments) != 1 {
		t.Errorf("expected 1 instrument in 'bass' group, got %d", len(grouped.Instruments))
	}
	if grouped.Instruments[0].Name != "desert_bass" {
		t.Errorf("expected 'desert_bass', got %q", grouped.Instruments[0].Name)
	}
}

func TestSaveNilFile(t *testing.T) {
	err := instrument.SaveFile("/tmp/test.yaml", nil)
	if err == nil {
		t.Error("expected error for nil file")
	}
}

func TestResolveAll(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	variants, err := f.ResolveAll("desert_bass")
	if err != nil {
		t.Fatalf("ResolveAll: %v", err)
	}
	if len(variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(variants))
	}
}

func TestResolveNotFound(t *testing.T) {
	f := &instrument.File{}
	_, err := f.Resolve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent instrument")
	}
}

func TestResolveAllNotFound(t *testing.T) {
	f := &instrument.File{}
	_, err := f.ResolveAll("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent instrument")
	}
}

func TestGroupsEmptyFile(t *testing.T) {
	f := &instrument.File{}
	groups := f.Groups()
	if groups == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestLoadGroupNoMatch(t *testing.T) {
	f := &instrument.File{}
	grouped := f.LoadGroup("nonexistent")
	if grouped == nil {
		t.Fatal("expected non-nil File")
	}
	if len(grouped.Instruments) != 0 {
		t.Errorf("expected 0 instruments, got %d", len(grouped.Instruments))
	}
}

func TestSaveLoadFile(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	tmpFile := t.TempDir() + "/test.yaml"
	err = instrument.SaveFile(tmpFile, f)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	_, err = os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("file should exist after save: %v", err)
	}
}

func TestCompactedFormat(t *testing.T) {
	yaml := `instruments:
  - name: test_compact
    variants:
      - name: compact
        op1: { a: 12, d: 5, s: 3, r: 4, l: 20, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 14, d: 6, s: 2, r: 5, l: 0, m: 2, kr: false, kl: 0, t: false, v: false, su: true, w: 0 }
        feedback: 5
        connection: 0
`
	f, err := instrument.LoadFileBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	v, err := f.Resolve("test_compact")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if v.Op1.Attack != 12 {
		t.Errorf("expected Op1.Attack 12, got %d", v.Op1.Attack)
	}
	if v.Op1.Decay != 5 {
		t.Errorf("expected Op1.Decay 5, got %d", v.Op1.Decay)
	}
	if v.Op1.Level != 20 {
		t.Errorf("expected Op1.Level 20, got %d", v.Op1.Level)
	}
	if v.Op1.Multiply != 1 {
		t.Errorf("expected Op1.Multiply 1, got %d", v.Op1.Multiply)
	}
	if v.Op1.KeyScaleRate != false {
		t.Errorf("expected Op1.KeyScaleRate false, got %v", v.Op1.KeyScaleRate)
	}
	if v.Op1.KeyScaleLevel != 0 {
		t.Errorf("expected Op1.KeyScaleLevel 0, got %d", v.Op1.KeyScaleLevel)
	}
	if v.Op1.Tremolo != false {
		t.Errorf("expected Op1.Tremolo false, got %v", v.Op1.Tremolo)
	}
	if v.Op1.Vibrato != false {
		t.Errorf("expected Op1.Vibrato false, got %v", v.Op1.Vibrato)
	}
	if v.Op1.Sustaining != false {
		t.Errorf("expected Op1.Sustaining false, got %v", v.Op1.Sustaining)
	}
	if v.Op1.Waveform != 0 {
		t.Errorf("expected Op1.Waveform 0, got %d", v.Op1.Waveform)
	}

	// Check op2
	if v.Op2.Sustaining != true {
		t.Errorf("expected Op2.Sustaining true, got %v", v.Op2.Sustaining)
	}
	if v.Feedback != 5 {
		t.Errorf("expected Feedback 5, got %d", v.Feedback)
	}
	if v.Connection != 0 {
		t.Errorf("expected Connection 0, got %d", v.Connection)
	}
}

func TestCompactedSaveRoundTrip(t *testing.T) {
	yaml := `instruments:
  - name: test_roundtrip
    variants:
      - name: compact
        op1: { a: 12, d: 5, s: 3, r: 4, l: 20, m: 1 }
        op2: { a: 14, d: 6, s: 2, r: 5, l: 0, m: 2, su: true }
        feedback: 5
        connection: 0
`
	f1, err := instrument.LoadFileBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	tmpFile := t.TempDir() + "/test.yaml"
	if err := instrument.SaveFile(tmpFile, f1); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	f2, err := instrument.LoadFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	v1 := f1.Instruments[0].Variants[0]
	v2 := f2.Instruments[0].Variants[0]

	if v1.Op1.Attack != v2.Op1.Attack {
		t.Errorf("Op1.Attack mismatch: %d vs %d", v1.Op1.Attack, v2.Op1.Attack)
	}
	if v1.Op1.Decay != v2.Op1.Decay {
		t.Errorf("Op1.Decay mismatch: %d vs %d", v1.Op1.Decay, v2.Op1.Decay)
	}
	if v1.Op2.Sustaining != v2.Op2.Sustaining {
		t.Errorf("Op2.Sustaining mismatch: %v vs %v", v1.Op2.Sustaining, v2.Op2.Sustaining)
	}
	if v1.Feedback != v2.Feedback {
		t.Errorf("Feedback mismatch: %d vs %d", v1.Feedback, v2.Feedback)
	}
}

func TestSaveEmitsAllCompactFields(t *testing.T) {
	f, err := instrument.LoadFileBytes([]byte(`instruments:
  - name: explicit
    variants:
      - name: full
        op1: { a: 1, d: 2, s: 3, r: 4, l: 5, m: 6, kr: true, kl: 2, t: true, v: false, su: true, w: 3 }
        op2: { a: 0, d: 0, s: 0, r: 0, l: 0, m: 0, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 0
        connection: 0
`))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	tmpFile := t.TempDir() + "/explicit.yaml"
	if err := instrument.SaveFile(tmpFile, f); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	for _, token := range []string{"a:", "d:", "s:", "r:", "l:", "m:", "kr:", "kl:", "t:", "v:", "su:", "w:"} {
		if !strings.Contains(text, token) {
			t.Fatalf("saved YAML missing compact field %q:\n%s", token, text)
		}
	}
	if !strings.Contains(text, "op1: {") {
		t.Fatalf("saved YAML should use compact flow style for op1:\n%s", text)
	}
	if !strings.Contains(text, "op2: {") {
		t.Fatalf("saved YAML should use compact flow style for op2:\n%s", text)
	}
}

func TestUnknownOperatorKeyFails(t *testing.T) {
	_, err := instrument.LoadFileBytes([]byte(`instruments:
  - name: broken
    variants:
      - name: bad
        op1: { a: 1, bad: 2 }
        op2: { a: 1, d: 1, s: 1, r: 1, l: 1, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 0
        connection: 0
`))
	if err == nil {
		t.Fatal("expected error for unknown operator key")
	}
}

func TestUnknownInstrumentKeyFails(t *testing.T) {
	_, err := instrument.LoadFileBytes([]byte(`instruments:
  - name: broken
    bogus: true
    variants:
      - name: bad
        op1: { a: 1, d: 1, s: 1, r: 1, l: 1, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 1, d: 1, s: 1, r: 1, l: 1, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 0
        connection: 0
`))
	if err == nil {
		t.Fatal("expected error for unknown instrument key")
	}
	if !strings.Contains(err.Error(), "unknown instrument key") {
		t.Fatalf("expected unknown instrument key error, got %v", err)
	}
}

func TestUnknownVariantKeyFails(t *testing.T) {
	_, err := instrument.LoadFileBytes([]byte(`instruments:
  - name: broken
    variants:
      - name: bad
        bogus: true
        op1: { a: 1, d: 1, s: 1, r: 1, l: 1, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 1, d: 1, s: 1, r: 1, l: 1, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 0
        connection: 0
`))
	if err == nil {
		t.Fatal("expected error for unknown variant key")
	}
	if !strings.Contains(err.Error(), "unknown variant key") {
		t.Fatalf("expected unknown variant key error, got %v", err)
	}
}

func TestInvalidDefaultNoteFails(t *testing.T) {
	_, err := instrument.LoadFileBytes([]byte(`instruments:
  - name: broken
    default_note: H9
    variants:
      - name: bad
        op1: { a: 1, d: 1, s: 1, r: 1, l: 1, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 1, d: 1, s: 1, r: 1, l: 1, m: 1, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 0
        connection: 0
`))
	if err == nil {
		t.Fatal("expected error for invalid default_note")
	}
	if !strings.Contains(err.Error(), "invalid default_note") {
		t.Fatalf("expected invalid default_note error, got %v", err)
	}
}

func TestInvalidRangesFail(t *testing.T) {
	_, err := instrument.LoadFileBytes([]byte(`instruments:
  - name: broken
    variants:
      - name: bad
        op1: { a: 16, d: 0, s: 0, r: 0, l: 0, m: 0, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 0, d: 0, s: 0, r: 0, l: 0, m: 0, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 0
        connection: 0
`))
	if err == nil {
		t.Fatal("expected error for out-of-range operator value")
	}

	_, err = instrument.LoadFileBytes([]byte(`instruments:
  - name: broken
    variants:
      - name: bad
        op1: { a: 1, d: 0, s: 0, r: 0, l: 0, m: 0, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        op2: { a: 1, d: 0, s: 0, r: 0, l: 0, m: 0, kr: false, kl: 0, t: false, v: false, su: false, w: 0 }
        feedback: 8
        connection: 0
`))
	if err == nil {
		t.Fatal("expected error for out-of-range feedback")
	}
}
