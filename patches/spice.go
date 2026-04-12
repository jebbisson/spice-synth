// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package patches

import (
	"github.com/jebbisson/spice-synth/voice"
)

// Spice style patches based on typical OPL2 sounds from that era.
func Spice() []*voice.Instrument {
	return []*voice.Instrument{
		{
			Name: "desert_bass",
			Op1: voice.Operator{
				Attack: 0, Decay: 8, Sustain: 10, Release: 4,
				Level: 20, Multiply: 1, Waveform: 0,
			},
			Op2: voice.Operator{
				Attack: 0, Decay: 6, Sustain: 12, Release: 8,
				Level: 5, Multiply: 2, Waveform: 0,
			},
			Feedback:   4,
			Connection: 0,
		},
		{
			Name: "mystic_lead",
			Op1: voice.Operator{
				Attack: 2, Decay: 10, Sustain: 5, Release: 6,
				Level: 30, Multiply: 1, Waveform: 1,
			},
			Op2: voice.Operator{
				Attack: 4, Decay: 12, Sustain: 8, Release: 10,
				Level: 15, Multiply: 4, Waveform: 0,
			},
			Feedback:   2,
			Connection: 0,
		},
		{
			Name: "industrial_pad",
			Op1: voice.Operator{
				Attack: 8, Decay: 15, Sustain: 2, Release: 12,
				Level: 40, Multiply: 1, Waveform: 2,
			},
			Op2: voice.Operator{
				Attack: 10, Decay: 15, Sustain: 5, Release: 15,
				Level: 20, Multiply: 1, Waveform: 0,
			},
			Feedback:   0,
			Connection: 1, // Additive
		},
		{
			Name: "fm_perc",
			Op1: voice.Operator{
				Attack: 0, Decay: 2, Sustain: 15, Release: 2,
				Level: 10, Multiply: 8, Waveform: 0,
			},
			Op2: voice.Operator{
				Attack: 0, Decay: 4, Sustain: 15, Release: 4,
				Level: 30, Multiply: 1, Waveform: 0,
			},
			Feedback:   7,
			Connection: 0,
		},
	}
}

// GM style patches (placeholder)
func GM() []*voice.Instrument {
	return []*voice.Instrument{
		{
			Name:       "piano_placeholder",
			Op1:        voice.Operator{Attack: 0, Decay: 4, Sustain: 12, Release: 6, Level: 30, Multiply: 1, Waveform: 0},
			Op2:        voice.Operator{Attack: 0, Decay: 8, Sustain: 15, Release: 10, Level: 40, Multiply: 1, Waveform: 0},
			Feedback:   0,
			Connection: 1,
		},
	}
}
