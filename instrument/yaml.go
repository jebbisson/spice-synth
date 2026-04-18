// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

// Package instrument loads and manages instrument definitions from YAML files.
//
// Each instrument has a name, optional group membership, and a list of variants
// (each with operator parameters). Variants are grouped under a parent instrument
// name for selection purposes.
package instrument

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jebbisson/spice-synth/internal/instrumentyaml"
	"github.com/jebbisson/spice-synth/voice"
	"gopkg.in/yaml.v3"
)

// OperatorDef holds OPL2 operator parameters for YAML serialization.
type OperatorDef = instrumentyaml.Operator

// InstrumentDef represents a single instrument variant definition.
type InstrumentDef struct {
	Name       string      `yaml:"name"`
	Op1        OperatorDef `yaml:"op1"`
	Op2        OperatorDef `yaml:"op2"`
	Feedback   uint8       `yaml:"feedback"`
	Connection uint8       `yaml:"connection"`
}

// Validate checks the instrument values fit the supported OPL2 ranges.
func (d *InstrumentDef) Validate() error {
	if d == nil {
		return fmt.Errorf("instrument: nil instrument definition")
	}
	if d.Name == "" {
		return fmt.Errorf("instrument: variant name cannot be empty")
	}
	v := instrumentyaml.Variant{Name: d.Name, Op1: d.Op1, Op2: d.Op2, Feedback: d.Feedback, Connection: d.Connection}
	if err := v.Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	return nil
}

func fromYAMLFile(f *instrumentyaml.File) *File {
	if f == nil {
		return nil
	}
	out := &File{Instruments: make([]FileInstrument, 0, len(f.Instruments))}
	for _, inst := range f.Instruments {
		entry := FileInstrument{Name: inst.Name, Group: inst.Group, DefaultNote: inst.DefaultNote, Variants: make([]InstrumentDef, 0, len(inst.Variants))}
		for _, variant := range inst.Variants {
			entry.Variants = append(entry.Variants, InstrumentDef{
				Name:       variant.Name,
				Op1:        variant.Op1,
				Op2:        variant.Op2,
				Feedback:   variant.Feedback,
				Connection: variant.Connection,
			})
		}
		out.Instruments = append(out.Instruments, entry)
	}
	return out
}

func (f *File) toYAMLFile() *instrumentyaml.File {
	if f == nil {
		return nil
	}
	out := &instrumentyaml.File{Instruments: make([]instrumentyaml.FileInstrument, 0, len(f.Instruments))}
	for _, inst := range f.Instruments {
		entry := instrumentyaml.FileInstrument{Name: inst.Name, Group: inst.Group, DefaultNote: inst.DefaultNote, Variants: make([]instrumentyaml.Variant, 0, len(inst.Variants))}
		for _, variant := range inst.Variants {
			entry.Variants = append(entry.Variants, instrumentyaml.Variant{
				Name:       variant.Name,
				Op1:        variant.Op1,
				Op2:        variant.Op2,
				Feedback:   variant.Feedback,
				Connection: variant.Connection,
			})
		}
		out.Instruments = append(out.Instruments, entry)
	}
	return out
}

// FileInstrument is an instrument entry in the YAML file.
type FileInstrument struct {
	Name        string          `yaml:"name"`
	Group       string          `yaml:"group,omitempty"`
	DefaultNote string          `yaml:"default_note,omitempty"`
	Variants    []InstrumentDef `yaml:"variants"`
}

// File represents a loaded instruments YAML file.
type File struct {
	Instruments []FileInstrument `yaml:"instruments"`
}

// ToInstrument converts an InstrumentDef to a voice.Instrument.
func (d *InstrumentDef) ToInstrument() *voice.Instrument {
	return &voice.Instrument{
		Name:       d.Name,
		Op1:        voice.Operator{Attack: d.Op1.Attack, Decay: d.Op1.Decay, Sustain: d.Op1.Sustain, Release: d.Op1.Release, Level: d.Op1.Level, Multiply: d.Op1.Multiply, KeyScaleRate: d.Op1.KeyScaleRate, KeyScaleLevel: d.Op1.KeyScaleLevel, Tremolo: d.Op1.Tremolo, Vibrato: d.Op1.Vibrato, Sustaining: d.Op1.Sustaining, Waveform: d.Op1.Waveform},
		Op2:        voice.Operator{Attack: d.Op2.Attack, Decay: d.Op2.Decay, Sustain: d.Op2.Sustain, Release: d.Op2.Release, Level: d.Op2.Level, Multiply: d.Op2.Multiply, KeyScaleRate: d.Op2.KeyScaleRate, KeyScaleLevel: d.Op2.KeyScaleLevel, Tremolo: d.Op2.Tremolo, Vibrato: d.Op2.Vibrato, Sustaining: d.Op2.Sustaining, Waveform: d.Op2.Waveform},
		Feedback:   d.Feedback,
		Connection: d.Connection,
	}
}

// ToInstrumentWithParent converts an InstrumentDef to a voice.Instrument
// with the name set to "parentName.variantName".
func (d *InstrumentDef) ToInstrumentWithParent(parentName string) *voice.Instrument {
	inst := d.ToInstrument()
	inst.Name = parentName + "." + inst.Name
	return inst
}

// ToInstrumentDef converts a voice.Instrument to an InstrumentDef.
func ToInstrumentDef(inst *voice.Instrument) *InstrumentDef {
	if inst == nil {
		return nil
	}
	name := inst.Name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return &InstrumentDef{
		Name:       name,
		Op1:        toOperatorDef(inst.Op1),
		Op2:        toOperatorDef(inst.Op2),
		Feedback:   inst.Feedback,
		Connection: inst.Connection,
	}
}

func toOperatorDef(op voice.Operator) OperatorDef {
	return OperatorDef{
		Attack:        op.Attack,
		Decay:         op.Decay,
		Sustain:       op.Sustain,
		Release:       op.Release,
		Level:         op.Level,
		Multiply:      op.Multiply,
		KeyScaleRate:  op.KeyScaleRate,
		KeyScaleLevel: op.KeyScaleLevel,
		Tremolo:       op.Tremolo,
		Vibrato:       op.Vibrato,
		Sustaining:    op.Sustaining,
		Waveform:      op.Waveform,
	}
}

// LoadFile loads a YAML instrument file from disk.
func LoadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("instrument: read %s: %w", path, err)
	}
	return LoadFileBytes(data)
}

// LoadFileBytes loads a YAML instrument file from bytes.
func LoadFileBytes(data []byte) (*File, error) {
	f, err := instrumentyaml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("instrument: %w", err)
	}
	return fromYAMLFile(f), nil
}

// LoadFileFromReader loads a YAML instrument file from a reader.
func LoadFileFromReader(r io.Reader) (*File, error) {
	if r == nil {
		return nil, fmt.Errorf("instrument: nil reader")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("instrument: read: %w", err)
	}
	return LoadFileBytes(data)
}

// SaveFile saves an instrument file to disk.
func SaveFile(path string, f *File) error {
	if f == nil {
		return fmt.Errorf("instrument: nil file")
	}
	if err := f.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(f.toYAMLFile())
	if err != nil {
		return fmt.Errorf("instrument: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("instrument: write %s: %w", path, err)
	}
	return nil
}

// Validate checks the entire file for required names and supported value ranges.
func (f *File) Validate() error {
	if f == nil {
		return fmt.Errorf("instrument: nil file")
	}
	if err := f.toYAMLFile().Validate(); err != nil {
		return fmt.Errorf("instrument: %w", err)
	}
	return nil
}
