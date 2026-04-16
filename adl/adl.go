// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package adl

import (
	"io"

	adplugadl "github.com/jebbisson/spice-adl-adplug"
	"github.com/jebbisson/spice-synth/voice"
)

// File wraps the extracted ADL file model while keeping the SpiceSynth-facing
// API stable.
type File struct {
	*adplugadl.File
}

// RawInstrument keeps the SpiceSynth-facing instrument parsing API stable while
// delegating the underlying ADL interpretation to the extracted module.
type RawInstrument adplugadl.RawInstrument

type SubsongType = adplugadl.SubsongType
type SubsongInfo = adplugadl.SubsongInfo

const (
	SubsongEmpty = adplugadl.SubsongEmpty
	SubsongReset = adplugadl.SubsongReset
	SubsongMusic = adplugadl.SubsongMusic
	SubsongSFX   = adplugadl.SubsongSFX
)

// Parse reads an ADL file from the given reader.
func Parse(r io.Reader) (*File, error) {
	f, err := adplugadl.Parse(r)
	if err != nil {
		return nil, err
	}
	return &File{File: f}, nil
}

// ParseBytes parses an ADL file from raw bytes.
func ParseBytes(data []byte) (*File, error) {
	f, err := adplugadl.ParseBytes(data)
	if err != nil {
		return nil, err
	}
	return &File{File: f}, nil
}

// ParseRawInstrument parses an 11-byte instrument definition.
func ParseRawInstrument(data []byte) (RawInstrument, error) {
	ri, err := adplugadl.ParseRawInstrument(data)
	if err != nil {
		return RawInstrument{}, err
	}
	return RawInstrument(ri), nil
}

// ToVoiceInstrument converts a RawInstrument into the local voice package type.
func (ri RawInstrument) ToVoiceInstrument(name string) *voice.Instrument {
	inst := adplugadl.RawInstrument(ri).ToInstrument(name)
	return convertInstrument(inst)
}

// ExtractInstruments keeps the local voice-oriented API while delegating the
// ADL-specific extraction logic to the extracted module.
func (f *File) ExtractInstruments(prefix string) []*voice.Instrument {
	if f == nil || f.File == nil {
		return nil
	}
	ext := f.File.ExtractInstruments(prefix)
	out := make([]*voice.Instrument, 0, len(ext))
	for _, inst := range ext {
		out = append(out, convertInstrument(inst))
	}
	return out
}

func convertInstrument(inst *adplugadl.Instrument) *voice.Instrument {
	if inst == nil {
		return nil
	}
	return &voice.Instrument{
		Name: inst.Name,
		Op1: voice.Operator{
			Attack:        inst.Op1.Attack,
			Decay:         inst.Op1.Decay,
			Sustain:       inst.Op1.Sustain,
			Release:       inst.Op1.Release,
			Level:         inst.Op1.Level,
			Multiply:      inst.Op1.Multiply,
			KeyScaleRate:  inst.Op1.KeyScaleRate,
			KeyScaleLevel: inst.Op1.KeyScaleLevel,
			Tremolo:       inst.Op1.Tremolo,
			Vibrato:       inst.Op1.Vibrato,
			Sustaining:    inst.Op1.Sustaining,
			Waveform:      inst.Op1.Waveform,
		},
		Op2: voice.Operator{
			Attack:        inst.Op2.Attack,
			Decay:         inst.Op2.Decay,
			Sustain:       inst.Op2.Sustain,
			Release:       inst.Op2.Release,
			Level:         inst.Op2.Level,
			Multiply:      inst.Op2.Multiply,
			KeyScaleRate:  inst.Op2.KeyScaleRate,
			KeyScaleLevel: inst.Op2.KeyScaleLevel,
			Tremolo:       inst.Op2.Tremolo,
			Vibrato:       inst.Op2.Vibrato,
			Sustaining:    inst.Op2.Sustaining,
			Waveform:      inst.Op2.Waveform,
		},
		Feedback:   inst.Feedback,
		Connection: inst.Connection,
	}
}

func toExternalFile(file *File) *adplugadl.File {
	if file == nil {
		return nil
	}
	return file.File
}
