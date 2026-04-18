// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package instrument_test

import (
	"strings"
	"testing"

	"github.com/jebbisson/spice-synth/instrument"
)

func TestResolveCaseInsensitive(t *testing.T) {
	yaml := `instruments:
  - name: Desert_Bass
    variants:
      - name: default
        op1: { a: 0 }
        op2: { a: 0 }
`
	f, err := instrument.LoadFileBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFileBytes: %v", err)
	}

	// Should find with different case
	v, err := f.Resolve("desert_bass")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v.Name != "Desert_Bass.default" {
		t.Errorf("expected 'Desert_Bass.default', got %q", v.Name)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := instrument.LoadFile("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadFileFromReader(t *testing.T) {
	r := strings.NewReader(sampleYAML)
	f, err := instrument.LoadFileFromReader(r)
	if err != nil {
		t.Fatalf("LoadFileFromReader: %v", err)
	}
	if len(f.Instruments) != 2 {
		t.Errorf("expected 2 instruments, got %d", len(f.Instruments))
	}
}

func TestLoadFileFromNilReader(t *testing.T) {
	_, err := instrument.LoadFileFromReader(nil)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestLoadFileBytesInvalidYAML(t *testing.T) {
	_, err := instrument.LoadFileBytes([]byte("{{invalid yaml: ["))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestToInstrumentDefNil(t *testing.T) {
	def := instrument.ToInstrumentDef(nil)
	if def != nil {
		t.Error("expected nil for nil instrument")
	}
}

func TestVariantKeyEmpty(t *testing.T) {
	f := &instrument.File{}
	key := f.VariantKey("", "")
	if key != "." {
		t.Errorf("expected '.', got %q", key)
	}
}

func TestFindByVariantKeyEmpty(t *testing.T) {
	f := &instrument.File{}
	_, err := f.FindByVariantKey("")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestLoadGroupEmpty(t *testing.T) {
	f := &instrument.File{
		Instruments: []instrument.FileInstrument{
			{Name: "test"},
		},
	}
	grouped := f.LoadGroup("nonexistent")
	if len(grouped.Instruments) != 0 {
		t.Errorf("expected 0 instruments, got %d", len(grouped.Instruments))
	}
}
