// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package patches

import (
	"github.com/jebbisson/spice-synth/voice"
)

// Spice style patches based on typical OPL2 sounds from that era.
// These are tuned for the grungy, aggressive FM sound of early 90s DOS games
// like Dune II, using correct OPL2 register addressing.
func Spice() []*voice.Instrument {
	return []*voice.Instrument{
		{
			// Heavy FM bass with feedback-driven grit. Modulator feedback
			// creates the crunchy harmonics, carrier stays clean at 1x multiply
			// for deep low end. Fast attack for punchy transients.
			Name: "desert_bass",
			Op1: voice.Operator{
				Attack: 15, Decay: 4, Sustain: 2, Release: 6,
				Level: 18, Multiply: 1, Waveform: 0,
				Sustaining: true,
			},
			Op2: voice.Operator{
				Attack: 15, Decay: 3, Sustain: 1, Release: 8,
				Level: 0, Multiply: 1, Waveform: 0,
				Sustaining: true,
			},
			Feedback:   6, // Heavy feedback for grungy character
			Connection: 0, // FM: modulator drives carrier
		},
		{
			// Nasal, cutting lead. Higher carrier multiply (3x) for a brighter
			// timbre with moderate feedback. Half-sine modulator adds edge.
			Name: "mystic_lead",
			Op1: voice.Operator{
				Attack: 14, Decay: 5, Sustain: 3, Release: 5,
				Level: 24, Multiply: 1, Waveform: 1, // half-sine
				Sustaining: true,
			},
			Op2: voice.Operator{
				Attack: 14, Decay: 6, Sustain: 2, Release: 7,
				Level: 0, Multiply: 3, Waveform: 0,
				Sustaining: true,
			},
			Feedback:   3,
			Connection: 0,
		},
		{
			// Slow-attack pad with additive synthesis for a fuller sound.
			Name: "industrial_pad",
			Op1: voice.Operator{
				Attack: 10, Decay: 4, Sustain: 2, Release: 8,
				Level: 8, Multiply: 1, Waveform: 2, // abs-sine
				Sustaining: true,
			},
			Op2: voice.Operator{
				Attack: 8, Decay: 5, Sustain: 3, Release: 10,
				Level: 4, Multiply: 1, Waveform: 0,
				Sustaining: true,
			},
			Feedback:   0,
			Connection: 1, // Additive
		},
		{
			// Short percussive hit. High modulator multiply (6x) and heavy
			// feedback create metallic/noisy character. Fast decay with no
			// sustain for a sharp transient.
			Name: "fm_perc",
			Op1: voice.Operator{
				Attack: 15, Decay: 8, Sustain: 15, Release: 8,
				Level: 14, Multiply: 6, Waveform: 0,
			},
			Op2: voice.Operator{
				Attack: 15, Decay: 9, Sustain: 15, Release: 9,
				Level: 0, Multiply: 1, Waveform: 0,
			},
			Feedback:   7, // Max feedback for noise-like modulation
			Connection: 0,
		},
		{
			// Sustained noise/wind texture. High modulator multiply and max
			// feedback create a harsh, static-like timbre that sustains
			// indefinitely. Useful for atmospheric beds and wind effects
			// when combined with a slow volume LFO.
			Name: "desert_wind",
			Op1: voice.Operator{
				Attack: 12, Decay: 4, Sustain: 2, Release: 10,
				Level: 20, Multiply: 7, Waveform: 0,
				Sustaining: true,
			},
			Op2: voice.Operator{
				Attack: 10, Decay: 3, Sustain: 1, Release: 12,
				Level: 0, Multiply: 1, Waveform: 0,
				Sustaining: true,
			},
			Feedback:   7, // Max feedback for noise character
			Connection: 0,
		},
		{
			// Bright metallic chime. High carrier multiply for a bell-like
			// overtone series, moderate modulation for shimmer. Fast attack,
			// medium decay produces a plucked/struck quality that rings out.
			Name: "spice_chime",
			Op1: voice.Operator{
				Attack: 15, Decay: 6, Sustain: 6, Release: 6,
				Level: 22, Multiply: 3, Waveform: 1, // half-sine
				Sustaining: false,
			},
			Op2: voice.Operator{
				Attack: 15, Decay: 7, Sustain: 10, Release: 7,
				Level: 0, Multiply: 4, Waveform: 0,
				Sustaining: false,
			},
			Feedback:   2,
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
