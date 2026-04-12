// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package voice

import "math"

// ModTarget identifies which OPL2 parameter a modulator controls.
type ModTarget int

const (
	// ModCarrierLevel modulates the carrier (Op2) total level register (0x40).
	// This is the primary target for volume swells and tremolo effects.
	ModCarrierLevel ModTarget = iota

	// ModModulatorLevel modulates the modulator (Op1) total level register (0x40).
	// Changing this alters the FM modulation depth / timbre brightness.
	ModModulatorLevel

	// ModFeedback modulates the channel feedback amount (0xC0 bits 1-3).
	ModFeedback

	// ModFrequency modulates the channel frequency (0xA0/0xB0) as a
	// fractional semitone offset from the base note. Useful for vibrato
	// and pitch wobble effects.
	ModFrequency
)

// WaveShape selects the LFO waveform.
type WaveShape int

const (
	// ShapeSine produces a smooth sinusoidal oscillation.
	ShapeSine WaveShape = iota
	// ShapeTriangle produces a linear triangle wave.
	ShapeTriangle
	// ShapeSaw produces a rising sawtooth wave.
	ShapeSaw
	// ShapeSquare produces a square wave (0 or 1, no intermediate values).
	ShapeSquare
)

// Modulator produces a time-varying signal used to drive an OPL2 parameter.
// The value returned by Tick is in the range [0, 1] (bipolar signals are
// centred at 0.5). The voice manager maps this normalised value to the
// appropriate register range for the given ModTarget.
type Modulator interface {
	// Tick advances the modulator by n samples at the given sample rate
	// and returns the current normalised output value (0.0–1.0).
	Tick(samples int, sampleRate int) float64

	// Target returns the parameter this modulator drives.
	Target() ModTarget

	// Done returns true if the modulator has finished (e.g. a one-shot
	// ramp that has reached its destination). Continuous modulators like
	// LFO always return false.
	Done() bool
}

// ---------------------------------------------------------------------------
// LFO — continuous oscillator at an arbitrary Hz rate
// ---------------------------------------------------------------------------

// LFO is a low-frequency oscillator that produces a periodic signal
// independent of sequencer tempo. Rate is in Hz, Depth controls the
// amplitude of the oscillation (0.0–1.0), and Center sets the midpoint.
//
// The output value is: Center + sin(phase) * Depth / 2
// clamped to [0, 1].
type LFO struct {
	RateHz float64   // oscillation frequency in Hz
	Depth  float64   // peak-to-peak amplitude (0.0–1.0)
	Center float64   // midpoint of oscillation (default 0.5)
	Shape  WaveShape // waveform shape
	target ModTarget

	phase float64 // current phase in radians (0–2π)
}

// NewLFO creates a new LFO modulator.
//
//   - target: the OPL2 parameter to modulate
//   - rateHz: oscillation speed in Hz (e.g. 0.3 for a slow throb)
//   - depth: peak-to-peak swing as a fraction of the parameter's full range (0.0–1.0)
//   - shape: waveform shape (ShapeSine, ShapeTriangle, etc.)
func NewLFO(target ModTarget, rateHz, depth float64, shape WaveShape) *LFO {
	return &LFO{
		RateHz: rateHz,
		Depth:  depth,
		Center: 0.5,
		Shape:  shape,
		target: target,
	}
}

// Tick advances the LFO phase and returns the current value.
func (l *LFO) Tick(samples int, sampleRate int) float64 {
	// Advance phase
	phaseInc := 2.0 * math.Pi * l.RateHz * float64(samples) / float64(sampleRate)
	l.phase += phaseInc
	// Keep phase in [0, 2π) to avoid floating-point drift over long runs
	if l.phase >= 2*math.Pi {
		l.phase -= 2 * math.Pi * math.Floor(l.phase/(2*math.Pi))
	}

	// Generate the raw wave value in [-1, 1]
	var raw float64
	switch l.Shape {
	case ShapeSine:
		raw = math.Sin(l.phase)
	case ShapeTriangle:
		// Convert phase to [0, 1) normalised position
		t := l.phase / (2 * math.Pi)
		if t < 0.25 {
			raw = t * 4.0
		} else if t < 0.75 {
			raw = 2.0 - t*4.0
		} else {
			raw = t*4.0 - 4.0
		}
	case ShapeSaw:
		t := l.phase / (2 * math.Pi)
		raw = 2.0*t - 1.0
	case ShapeSquare:
		if l.phase < math.Pi {
			raw = 1.0
		} else {
			raw = -1.0
		}
	default:
		raw = math.Sin(l.phase)
	}

	// Map to [0, 1]: value = center + raw * depth/2
	val := l.Center + raw*l.Depth/2.0
	return clamp01(val)
}

// Target returns the modulation target.
func (l *LFO) Target() ModTarget { return l.target }

// Done always returns false — LFOs run continuously.
func (l *LFO) Done() bool { return false }

// ---------------------------------------------------------------------------
// Ramp — linear interpolation from one value to another over a duration
// ---------------------------------------------------------------------------

// Ramp linearly interpolates a parameter from one normalised value to
// another over a fixed number of samples. Once the destination is reached,
// Done() returns true and the output holds at the final value.
type Ramp struct {
	From, To       float64 // normalised start / end values (0.0–1.0)
	DurationFrames int     // total duration in sample frames
	target         ModTarget

	elapsed int // sample frames elapsed so far
}

// NewRamp creates a one-shot linear ramp.
//
//   - target: the OPL2 parameter to modulate
//   - from, to: start and end values in [0, 1]
//   - durationSec: ramp duration in seconds
//   - sampleRate: audio sample rate (used to convert seconds to frames)
func NewRamp(target ModTarget, from, to, durationSec float64, sampleRate int) *Ramp {
	frames := int(durationSec * float64(sampleRate))
	if frames < 1 {
		frames = 1
	}
	return &Ramp{
		From:           from,
		To:             to,
		DurationFrames: frames,
		target:         target,
	}
}

// Tick advances the ramp and returns the current interpolated value.
func (r *Ramp) Tick(samples int, _ int) float64 {
	r.elapsed += samples
	if r.elapsed >= r.DurationFrames {
		r.elapsed = r.DurationFrames
		return r.To
	}
	t := float64(r.elapsed) / float64(r.DurationFrames)
	return r.From + (r.To-r.From)*t
}

// Target returns the modulation target.
func (r *Ramp) Target() ModTarget { return r.target }

// Done returns true when the ramp has reached its destination.
func (r *Ramp) Done() bool { return r.elapsed >= r.DurationFrames }

// ---------------------------------------------------------------------------
// Envelope — software ADSR envelope (independent of the OPL hardware ADSR)
// ---------------------------------------------------------------------------

// EnvStage identifies the current phase of a software envelope.
type EnvStage int

const (
	EnvAttack EnvStage = iota
	EnvDecay
	EnvSustain
	EnvRelease
	EnvDone
)

// Envelope is a software ADSR envelope generator. All times are in seconds,
// sustain is a normalised level (0.0–1.0). The output rises from 0 to 1
// during attack, falls to Sustain during decay, holds during sustain, then
// falls to 0 during release.
//
// Call Release() to move from the sustain phase to the release phase.
type Envelope struct {
	AttackSec  float64 // time from 0 → 1
	DecaySec   float64 // time from 1 → Sustain
	SustainLvl float64 // hold level (0.0–1.0)
	ReleaseSec float64 // time from Sustain → 0
	target     ModTarget

	stage   EnvStage
	level   float64 // current output level
	elapsed float64 // seconds elapsed in current stage
}

// NewEnvelope creates a new software ADSR envelope.
func NewEnvelope(target ModTarget, attack, decay, sustain, release float64) *Envelope {
	return &Envelope{
		AttackSec:  attack,
		DecaySec:   decay,
		SustainLvl: sustain,
		ReleaseSec: release,
		target:     target,
		stage:      EnvAttack,
	}
}

// Tick advances the envelope and returns the current level.
func (e *Envelope) Tick(samples int, sampleRate int) float64 {
	dt := float64(samples) / float64(sampleRate)
	e.elapsed += dt

	switch e.stage {
	case EnvAttack:
		if e.AttackSec <= 0 {
			e.level = 1.0
			e.stage = EnvDecay
			e.elapsed = 0
		} else {
			e.level = e.elapsed / e.AttackSec
			if e.level >= 1.0 {
				e.level = 1.0
				e.stage = EnvDecay
				e.elapsed = 0
			}
		}
	case EnvDecay:
		if e.DecaySec <= 0 {
			e.level = e.SustainLvl
			e.stage = EnvSustain
			e.elapsed = 0
		} else {
			t := e.elapsed / e.DecaySec
			if t >= 1.0 {
				e.level = e.SustainLvl
				e.stage = EnvSustain
				e.elapsed = 0
			} else {
				e.level = 1.0 - t*(1.0-e.SustainLvl)
			}
		}
	case EnvSustain:
		e.level = e.SustainLvl
	case EnvRelease:
		if e.ReleaseSec <= 0 {
			e.level = 0
			e.stage = EnvDone
		} else {
			t := e.elapsed / e.ReleaseSec
			if t >= 1.0 {
				e.level = 0
				e.stage = EnvDone
			} else {
				e.level = e.SustainLvl * (1.0 - t)
			}
		}
	case EnvDone:
		e.level = 0
	}

	return clamp01(e.level)
}

// ReleaseEnv triggers the release phase of the envelope.
func (e *Envelope) ReleaseEnv() {
	if e.stage != EnvDone {
		e.stage = EnvRelease
		e.elapsed = 0
	}
}

// Target returns the modulation target.
func (e *Envelope) Target() ModTarget { return e.target }

// Done returns true when the envelope has completed its release phase.
func (e *Envelope) Done() bool { return e.stage == EnvDone }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
