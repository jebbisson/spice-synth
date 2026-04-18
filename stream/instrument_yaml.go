// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package stream

import (
	"fmt"

	"github.com/jebbisson/spice-synth/internal/instrumentyaml"
	"github.com/jebbisson/spice-synth/voice"
)

func LoadInstrumentsFromYAML(s *Stream, path string) error {
	if s == nil {
		return fmt.Errorf("stream: nil stream")
	}
	f, err := instrumentyaml.LoadFile(path)
	if err != nil {
		return fmt.Errorf("stream: %w", err)
	}
	return loadYAMLIntoStream(s, f, "")
}

func LoadInstrumentsFromYAMLGroup(s *Stream, path, group string) error {
	if s == nil {
		return fmt.Errorf("stream: nil stream")
	}
	f, err := instrumentyaml.LoadFile(path)
	if err != nil {
		return fmt.Errorf("stream: %w", err)
	}
	return loadYAMLIntoStream(s, f, group)
}

func loadYAMLIntoStream(s *Stream, f *instrumentyaml.File, group string) error {
	insts := make([]*voice.Instrument, 0)
	for _, entry := range f.Instruments {
		if group != "" && entry.Group != group {
			continue
		}
		for _, variant := range entry.Variants {
			insts = append(insts, &voice.Instrument{
				Name: entry.Name + "." + variant.Name,
				Op1: voice.Operator{
					Attack:        variant.Op1.Attack,
					Decay:         variant.Op1.Decay,
					Sustain:       variant.Op1.Sustain,
					Release:       variant.Op1.Release,
					Level:         variant.Op1.Level,
					Multiply:      variant.Op1.Multiply,
					KeyScaleRate:  variant.Op1.KeyScaleRate,
					KeyScaleLevel: variant.Op1.KeyScaleLevel,
					Tremolo:       variant.Op1.Tremolo,
					Vibrato:       variant.Op1.Vibrato,
					Sustaining:    variant.Op1.Sustaining,
					Waveform:      variant.Op1.Waveform,
				},
				Op2: voice.Operator{
					Attack:        variant.Op2.Attack,
					Decay:         variant.Op2.Decay,
					Sustain:       variant.Op2.Sustain,
					Release:       variant.Op2.Release,
					Level:         variant.Op2.Level,
					Multiply:      variant.Op2.Multiply,
					KeyScaleRate:  variant.Op2.KeyScaleRate,
					KeyScaleLevel: variant.Op2.KeyScaleLevel,
					Tremolo:       variant.Op2.Tremolo,
					Vibrato:       variant.Op2.Vibrato,
					Sustaining:    variant.Op2.Sustaining,
					Waveform:      variant.Op2.Waveform,
				},
				Feedback:   variant.Feedback,
				Connection: variant.Connection,
			})
		}
	}
	s.voices.LoadBank("yaml", insts)
	return nil
}
