// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// Package dsl provides a fluent, method-chaining API for composing FM
// synthesis patterns, inspired by Strudel (https://strudel.cc). Unlike
// Strudel (which targets Web Audio), SpiceSynth targets real OPL2/OPL3
// register writes through the voice and sequencer packages.
//
// Basic usage:
//
//	p := dsl.Note("c2").Sound("desert_bass").FM(6).Feedback(6).Attack(0.0)
//	p.Play(s, 0) // play on channel 0 of a *stream.Stream
package dsl

// Value holds either a static float64 or a Signal for continuous modulation.
// This allows DSL methods to accept both constants and signals.
type Value struct {
	static    float64
	signal    *Signal
	isSignal  bool
	isPresent bool // true if this value was explicitly set
}

// Static creates a Value from a constant.
func Static(v float64) Value {
	return Value{static: v, isPresent: true}
}

// Dynamic creates a Value from a Signal.
func Dynamic(s *Signal) Value {
	return Value{signal: s, isSignal: true, isPresent: true}
}

// Pattern describes a single musical event or voice configuration using
// fluent method chaining. It accumulates parameter settings that are
// resolved when Play() is called.
type Pattern struct {
	// Tier 1: Core
	noteStr string // note string (e.g. "C2", "Eb4")
	noteSet bool
	sound   string // instrument name or raw waveform name
	nStep   string // scale step index string (e.g. "0 2 4 7")
	scale   string // scale name (e.g. "C4:minor")

	// Tier 2: Carrier ADSR (hardware envelope)
	attack    Value // seconds -> OPL2 rate (0-15)
	decay     Value // seconds -> OPL2 rate (0-15)
	sustain   Value // 0.0-1.0 -> inverted OPL2 level (0-15)
	release   Value // seconds -> OPL2 rate (0-15)
	sustained *bool // sustaining (EG type) flag

	// Tier 3: FM parameters
	fm        Value // FM depth -> modulator total level
	fmh       Value // harmonicity ratio -> modulator frequency multiplier
	fmAttack  Value // seconds -> modulator attack rate
	fmDecay   Value // seconds -> modulator decay rate
	fmSustain Value // 0.0-1.0 -> modulator sustain level
	feedback  Value // 0-7 feedback amount
	conn      *int  // connection mode (0=FM, 1=Additive)
	carrierWF *int  // carrier waveform (0-3)
	modWF     *int  // modulator waveform (0-3)

	// Tier 7: Dynamics
	gain     Value // output level (0.0-1.0)
	velocity Value // per-event volume multiplier

	// Ramp parameters (one-shot volume automation)
	rampFrom *float64
	rampTo   *float64
	rampSec  *float64

	// Hardware flags
	hwTremolo *bool
	hwVibrato *bool
}

// Note creates a new Pattern with the given pitch. Accepts note names like
// "C2", "Eb4", "F#3", or MIDI note numbers as strings.
func Note(noteStr string) *Pattern {
	return &Pattern{noteStr: noteStr, noteSet: true}
}

// Note sets the pitch on an existing pattern (method chaining).
func (p *Pattern) Note(noteStr string) *Pattern {
	p.noteStr = noteStr
	p.noteSet = true
	return p
}

// Sound selects the instrument by name. When given a named instrument
// (e.g. "desert_bass"), it loads the full instrument. When given a raw
// waveform name ("sine", "halfsine", "abssine", "quartersine"), it creates
// a minimal carrier-only instrument.
func (p *Pattern) Sound(name string) *Pattern {
	p.sound = name
	return p
}

// N sets the scale step index. Requires Scale() to resolve to a pitch.
func (p *Pattern) N(step string) *Pattern {
	p.nStep = step
	return p
}

// Scale sets the musical scale for N() step resolution.
func (p *Pattern) Scale(s string) *Pattern {
	p.scale = s
	return p
}

// ---------------------------------------------------------------------------
// Tier 2: Carrier ADSR
// ---------------------------------------------------------------------------

// Attack sets the carrier attack time in seconds. Accepts a float64 or
// *Signal for continuous modulation.
func (p *Pattern) Attack(v float64) *Pattern {
	p.attack = Static(v)
	return p
}

// Decay sets the carrier decay time in seconds.
func (p *Pattern) Decay(v float64) *Pattern {
	p.decay = Static(v)
	return p
}

// Sustain sets the carrier sustain level (0.0 = silent, 1.0 = loudest).
// This is transparently inverted for OPL2's native scale.
func (p *Pattern) Sustain(v float64) *Pattern {
	p.sustain = Static(v)
	return p
}

// Release sets the carrier release time in seconds.
func (p *Pattern) Release(v float64) *Pattern {
	p.release = Static(v)
	return p
}

// ADSR sets all four carrier envelope parameters at once.
func (p *Pattern) ADSR(a, d, s, r float64) *Pattern {
	p.attack = Static(a)
	p.decay = Static(d)
	p.sustain = Static(s)
	p.release = Static(r)
	return p
}

// Sustaining controls whether the carrier envelope holds at the sustain
// level until key-off (true) or decays automatically (false/percussive).
func (p *Pattern) Sustaining(v bool) *Pattern {
	p.sustained = &v
	return p
}

// ---------------------------------------------------------------------------
// Tier 3: FM parameters
// ---------------------------------------------------------------------------

// FM sets the FM modulation depth. Higher values = more modulation.
// Maps to modulator total level (register 0x40 Op1), inverted scale.
// Accepts a float64 constant or use FMSignal() for continuous modulation.
func (p *Pattern) FM(depth float64) *Pattern {
	p.fm = Static(depth)
	return p
}

// FMSignal sets the FM modulation depth to a continuous signal.
func (p *Pattern) FMSignal(s *Signal) *Pattern {
	p.fm = Dynamic(s)
	return p
}

// FMH sets the FM harmonicity ratio (modulator frequency multiplier).
// OPL2 only supports: 0.5, 1-10, 12, 15. Non-integer values are rounded.
func (p *Pattern) FMH(ratio float64) *Pattern {
	p.fmh = Static(ratio)
	return p
}

// FMAttack sets the modulator's attack time in seconds.
func (p *Pattern) FMAttack(v float64) *Pattern {
	p.fmAttack = Static(v)
	return p
}

// FMDecay sets the modulator's decay time in seconds.
func (p *Pattern) FMDecay(v float64) *Pattern {
	p.fmDecay = Static(v)
	return p
}

// FMSustain sets the modulator's sustain level (0.0-1.0).
func (p *Pattern) FMSustain(v float64) *Pattern {
	p.fmSustain = Static(v)
	return p
}

// Feedback sets the modulator self-feedback amount (0-7). This is a uniquely
// OPL2 parameter — the primary source of "grit" and "crunch" in FM sounds.
// Accepts a float64 constant or use FeedbackSignal() for modulation.
func (p *Pattern) Feedback(fb float64) *Pattern {
	p.feedback = Static(fb)
	return p
}

// FeedbackSignal sets the feedback to a continuous signal.
func (p *Pattern) FeedbackSignal(s *Signal) *Pattern {
	p.feedback = Dynamic(s)
	return p
}

// Connection sets the synthesis algorithm.
// 0 = FM mode (Op1 modulates Op2). 1 = Additive mode (both operators output).
func (p *Pattern) Connection(mode int) *Pattern {
	p.conn = &mode
	return p
}

// Waveform sets the carrier and modulator waveforms.
// OPL2 values: 0=sine, 1=half-sine, 2=abs-sine, 3=quarter-sine.
func (p *Pattern) Waveform(carrier, modulator int) *Pattern {
	p.carrierWF = &carrier
	p.modWF = &modulator
	return p
}

// ---------------------------------------------------------------------------
// Tier 7: Dynamics & Gain
// ---------------------------------------------------------------------------

// Gain sets the output level (0.0 = silent, 1.0 = loudest). Accepts a
// float64 constant or use GainSignal() for continuous modulation.
func (p *Pattern) Gain(v float64) *Pattern {
	p.gain = Static(v)
	return p
}

// GainSignal sets the output level to a continuous signal for real-time
// volume modulation (e.g. wobble, swell).
func (p *Pattern) GainSignal(s *Signal) *Pattern {
	p.gain = Dynamic(s)
	return p
}

// Velocity sets the per-event volume multiplier (0.0-1.0).
func (p *Pattern) Velocity(v float64) *Pattern {
	p.velocity = Static(v)
	return p
}

// Ramp adds a one-shot volume automation from one level to another over the
// specified duration in seconds. This is multiplied with any concurrent
// gain signal.
func (p *Pattern) Ramp(from, to, sec float64) *Pattern {
	p.rampFrom = &from
	p.rampTo = &to
	p.rampSec = &sec
	return p
}

// ---------------------------------------------------------------------------
// Hardware feature flags
// ---------------------------------------------------------------------------

// HWTremolo enables/disables hardware tremolo on the carrier operator.
// Rate is fixed at ~3.7 Hz, depth is chip-global.
func (p *Pattern) HWTremolo(v bool) *Pattern {
	p.hwTremolo = &v
	return p
}

// HWVibrato enables/disables hardware vibrato on the carrier operator.
// Rate is fixed at ~6.1 Hz, depth is chip-global.
func (p *Pattern) HWVibrato(v bool) *Pattern {
	p.hwVibrato = &v
	return p
}
