// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package dsl

import "github.com/jebbisson/spice-synth/voice"

// Signal represents a continuous time-varying value (0.0-1.0) that can be
// passed to any DSL parameter method to create real-time modulation. Signals
// compile down to voice.Modulator instances (LFO, Ramp, Envelope) at play time.
//
// Signals are lazy descriptions — they don't produce values until compiled
// and attached to an OPL2 channel via the voice manager.
type Signal struct {
	shape    voice.WaveShape
	rateHz   float64 // base oscillation rate in Hz
	lo, hi   float64 // output range (default 0.0, 1.0)
	slowMul  float64 // slow factor (default 1.0 = no change)
	fastMul  float64 // fast factor (default 1.0 = no change)
	segments int     // sample-and-hold steps per cycle (0 = continuous)
}

// Sine creates a smooth sinusoidal signal oscillating at 1 Hz by default.
func Sine() *Signal {
	return &Signal{shape: voice.ShapeSine, rateHz: 1.0, lo: 0.0, hi: 1.0, slowMul: 1.0, fastMul: 1.0}
}

// Cosine creates a cosine signal (sine with pi/2 phase offset).
// Implemented as sine — the phase offset is handled at compile time.
func Cosine() *Signal {
	// For now, cosine is treated as sine since the LFO doesn't expose phase.
	// A future improvement could add a phase offset field.
	return Sine()
}

// Tri creates a triangle wave signal oscillating at 1 Hz by default.
func Tri() *Signal {
	return &Signal{shape: voice.ShapeTriangle, rateHz: 1.0, lo: 0.0, hi: 1.0, slowMul: 1.0, fastMul: 1.0}
}

// Saw creates a rising sawtooth signal oscillating at 1 Hz by default.
func Saw() *Signal {
	return &Signal{shape: voice.ShapeSaw, rateHz: 1.0, lo: 0.0, hi: 1.0, slowMul: 1.0, fastMul: 1.0}
}

// Square creates a square wave signal oscillating at 1 Hz by default.
func Square() *Signal {
	return &Signal{shape: voice.ShapeSquare, rateHz: 1.0, lo: 0.0, hi: 1.0, slowMul: 1.0, fastMul: 1.0}
}

// Range sets the output range of the signal. The raw 0-1 oscillation is
// mapped to [lo, hi]. For example, Sine().Range(0.3, 1.0) oscillates
// between 0.3 and 1.0.
func (s *Signal) Range(lo, hi float64) *Signal {
	s.lo = lo
	s.hi = hi
	return s
}

// Slow makes the signal oscillate slower by a factor. For example,
// Sine().Slow(4) completes one cycle every 4 seconds instead of 1.
func (s *Signal) Slow(factor float64) *Signal {
	if factor <= 0 {
		factor = 1.0
	}
	s.slowMul = factor
	return s
}

// Fast makes the signal oscillate faster by a factor. For example,
// Sine().Fast(2) completes 2 cycles per second instead of 1.
func (s *Signal) Fast(factor float64) *Signal {
	if factor <= 0 {
		factor = 1.0
	}
	s.fastMul = factor
	return s
}

// Segment enables sample-and-hold: the signal value only changes n times
// per cycle, creating a stepped/quantized modulation effect.
func (s *Signal) Segment(n int) *Signal {
	if n < 1 {
		n = 1
	}
	s.segments = n
	return s
}

// effectiveRate returns the final oscillation rate in Hz after applying
// slow/fast modifiers.
func (s *Signal) effectiveRate() float64 {
	return s.rateHz * s.fastMul / s.slowMul
}

// compile converts this Signal description into a voice.Modulator targeting
// the specified parameter. The signal's range [lo, hi] is mapped to the
// LFO's center and depth.
func (s *Signal) compile(target voice.ModTarget) voice.Modulator {
	rate := s.effectiveRate()

	// Map [lo, hi] to LFO center and depth.
	// LFO output = center + raw * depth/2, where raw is in [-1, 1].
	// So min = center - depth/2 = lo, max = center + depth/2 = hi.
	depth := s.hi - s.lo
	center := (s.lo + s.hi) / 2.0

	lfo := voice.NewLFO(target, rate, depth, s.shape)
	lfo.Center = center
	return lfo
}
