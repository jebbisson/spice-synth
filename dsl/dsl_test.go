// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package dsl

import (
	"math"
	"testing"

	"github.com/jebbisson/spice-synth/voice"
)

// ---------------------------------------------------------------------------
// Helper conversion tests
// ---------------------------------------------------------------------------

func TestSecondsToRate(t *testing.T) {
	tests := []struct {
		sec  float64
		want uint8
	}{
		{0.0, 15},   // instant
		{-1.0, 15},  // negative = instant
		{0.15, 14},  // ~0.15s
		{0.21, 13},  // ~0.21s
		{0.30, 12},  // ~0.30s
		{0.42, 11},  // ~0.42s
		{0.60, 10},  // ~0.60s
		{0.84, 9},   // ~0.84s
		{1.2, 8},    // ~1.2s
		{1.7, 7},    // ~1.7s
		{2.4, 6},    // ~2.4s
		{3.4, 5},    // ~3.4s
		{4.8, 4},    // ~4.8s
		{6.0, 3},    // ~6.0s
		{8.0, 2},    // ~8.0s
		{10.0, 1},   // ~10.0s
		{20.0, 1},   // beyond max still maps to slowest
		{0.5, 11},   // between 0.42 and 0.60, closer to 0.42
		{1.0, 9},    // between 0.84 and 1.2, closer to 0.84
		{0.001, 15}, // very fast = instant
	}

	for _, tt := range tests {
		got := secondsToRate(tt.sec)
		if got != tt.want {
			t.Errorf("secondsToRate(%.3f) = %d, want %d", tt.sec, got, tt.want)
		}
	}
}

func TestSustainToOPL(t *testing.T) {
	tests := []struct {
		level float64
		want  uint8
	}{
		{1.0, 0},   // loudest -> OPL2 0
		{0.0, 15},  // silent -> OPL2 15
		{0.5, 8},   // mid-level
		{-1.0, 15}, // below 0 -> silent
		{2.0, 0},   // above 1 -> loudest
	}

	for _, tt := range tests {
		got := sustainToOPL(tt.level)
		if got != tt.want {
			t.Errorf("sustainToOPL(%.1f) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestFMDepthToLevel(t *testing.T) {
	tests := []struct {
		depth float64
		want  uint8
	}{
		{0.0, 63},  // no FM = max attenuation
		{10.0, 0},  // FM=10 = no attenuation (max modulation)
		{5.0, 32},  // FM=5 = roughly mid
		{-1.0, 63}, // negative clamped
		{20.0, 0},  // beyond max still 0
	}

	for _, tt := range tests {
		got := fmDepthToLevel(tt.depth)
		// Allow +/- 1 for rounding
		diff := int(got) - int(tt.want)
		if diff < -1 || diff > 1 {
			t.Errorf("fmDepthToLevel(%.1f) = %d, want ~%d", tt.depth, got, tt.want)
		}
	}
}

func TestFMHToMultiplier(t *testing.T) {
	tests := []struct {
		ratio float64
		want  uint8
	}{
		{0.5, 0},   // exact match
		{1.0, 1},   // exact
		{2.0, 2},   // exact
		{10.0, 10}, // maps to reg 10
		{15.0, 14}, // maps to reg 14 (ratio 15)
		{1.5, 1},   // rounds to 1 (closer to 1.0 than 2.0)
		{1.8, 2},   // rounds to 2
		{11.0, 12}, // between 10 and 12, closer to 12
	}

	for _, tt := range tests {
		got := fmhToMultiplier(tt.ratio)
		// Verify the chosen register maps to the closest available ratio
		gotRatio := fmhRatios[got]
		wantRatio := fmhRatios[tt.want]
		if math.Abs(gotRatio-tt.ratio) > math.Abs(wantRatio-tt.ratio)+0.01 {
			t.Errorf("fmhToMultiplier(%.1f) = %d (ratio %.1f), want %d (ratio %.1f)",
				tt.ratio, got, gotRatio, tt.want, wantRatio)
		}
	}
}

// ---------------------------------------------------------------------------
// Signal tests
// ---------------------------------------------------------------------------

func TestSignalRange(t *testing.T) {
	s := Sine().Range(0.3, 0.8)
	if s.lo != 0.3 || s.hi != 0.8 {
		t.Errorf("Range: lo=%f, hi=%f, want 0.3, 0.8", s.lo, s.hi)
	}
}

func TestSignalSlow(t *testing.T) {
	s := Sine().Slow(4)
	rate := s.effectiveRate()
	if rate != 0.25 {
		t.Errorf("Slow(4): rate=%f, want 0.25", rate)
	}
}

func TestSignalFast(t *testing.T) {
	s := Sine().Fast(3)
	rate := s.effectiveRate()
	if rate != 3.0 {
		t.Errorf("Fast(3): rate=%f, want 3.0", rate)
	}
}

func TestSignalSlowAndFast(t *testing.T) {
	s := Sine().Slow(2).Fast(4)
	rate := s.effectiveRate()
	if rate != 2.0 {
		t.Errorf("Slow(2).Fast(4): rate=%f, want 2.0", rate)
	}
}

func TestSignalCompile(t *testing.T) {
	s := Sine().Range(0.3, 1.0).Slow(4)
	mod := s.compile(voice.ModCarrierLevel)

	lfo, ok := mod.(*voice.LFO)
	if !ok {
		t.Fatalf("compile returned %T, want *voice.LFO", mod)
	}

	if lfo.Target() != voice.ModCarrierLevel {
		t.Errorf("target = %v, want ModCarrierLevel", lfo.Target())
	}

	// Check that depth and center correctly encode the range [0.3, 1.0]
	wantDepth := 0.7   // 1.0 - 0.3
	wantCenter := 0.65 // (0.3 + 1.0) / 2
	if math.Abs(lfo.Depth-wantDepth) > 0.001 {
		t.Errorf("depth = %f, want %f", lfo.Depth, wantDepth)
	}
	if math.Abs(lfo.Center-wantCenter) > 0.001 {
		t.Errorf("center = %f, want %f", lfo.Center, wantCenter)
	}

	// Check rate: 1 Hz base / 4 slow = 0.25 Hz
	if math.Abs(lfo.RateHz-0.25) > 0.001 {
		t.Errorf("rate = %f, want 0.25", lfo.RateHz)
	}
}

// ---------------------------------------------------------------------------
// Pattern builder tests
// ---------------------------------------------------------------------------

func TestPatternFluentChaining(t *testing.T) {
	p := Note("C2").S("desert_bass").FM(6).Feedback(6).Attack(0.0).Sustaining(true)

	if p.noteStr != "C2" {
		t.Errorf("noteStr = %q, want C2", p.noteStr)
	}
	if !p.noteSet {
		t.Error("noteSet should be true")
	}
	if p.sound != "desert_bass" {
		t.Errorf("sound = %q, want desert_bass", p.sound)
	}
	if !p.fm.isPresent || p.fm.static != 6.0 {
		t.Errorf("fm = %v, want 6.0", p.fm)
	}
	if !p.feedback.isPresent || p.feedback.static != 6.0 {
		t.Errorf("feedback = %v, want 6.0", p.feedback)
	}
	if !p.attack.isPresent || p.attack.static != 0.0 {
		t.Errorf("attack = %v, want 0.0", p.attack)
	}
	if p.sustained == nil || !*p.sustained {
		t.Error("sustained should be true")
	}
}

func TestPatternADSR(t *testing.T) {
	p := Note("C4").ADSR(0.1, 0.2, 0.5, 0.3)

	if !p.attack.isPresent || p.attack.static != 0.1 {
		t.Errorf("attack = %v, want 0.1", p.attack)
	}
	if !p.decay.isPresent || p.decay.static != 0.2 {
		t.Errorf("decay = %v, want 0.2", p.decay)
	}
	if !p.sustain.isPresent || p.sustain.static != 0.5 {
		t.Errorf("sustain = %v, want 0.5", p.sustain)
	}
	if !p.release.isPresent || p.release.static != 0.3 {
		t.Errorf("release = %v, want 0.3", p.release)
	}
}

func TestPatternWaveform(t *testing.T) {
	p := Note("C4").Waveform(2, 1)

	if p.carrierWF == nil || *p.carrierWF != 2 {
		t.Errorf("carrierWF = %v, want 2", p.carrierWF)
	}
	if p.modWF == nil || *p.modWF != 1 {
		t.Errorf("modWF = %v, want 1", p.modWF)
	}
}

func TestPatternSignalGain(t *testing.T) {
	sig := Sine().Range(0.3, 1.0).Slow(4)
	p := Note("C2").GainSignal(sig)

	if !p.gain.isPresent || !p.gain.isSignal {
		t.Fatal("gain should be a signal")
	}
	if p.gain.signal != sig {
		t.Error("gain signal mismatch")
	}
}

func TestPatternRamp(t *testing.T) {
	p := Note("C2").Ramp(0.0, 1.0, 5.0)

	if p.rampFrom == nil || *p.rampFrom != 0.0 {
		t.Errorf("rampFrom = %v, want 0.0", p.rampFrom)
	}
	if p.rampTo == nil || *p.rampTo != 1.0 {
		t.Errorf("rampTo = %v, want 1.0", p.rampTo)
	}
	if p.rampSec == nil || *p.rampSec != 5.0 {
		t.Errorf("rampSec = %v, want 5.0", p.rampSec)
	}
}

// ---------------------------------------------------------------------------
// Instrument resolution tests
// ---------------------------------------------------------------------------

func TestResolveRawWaveform(t *testing.T) {
	p := Note("C4").S("halfsine")
	inst, err := p.resolveInstrument(nil)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Op2.Waveform != 1 {
		t.Errorf("carrier waveform = %d, want 1 (half-sine)", inst.Op2.Waveform)
	}
}

func TestResolveDefault(t *testing.T) {
	p := Note("C4")
	inst, err := p.resolveInstrument(nil)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Name != "dsl_default" {
		t.Errorf("name = %q, want dsl_default", inst.Name)
	}
	if inst.Op2.Waveform != 0 {
		t.Errorf("carrier waveform = %d, want 0 (sine)", inst.Op2.Waveform)
	}
}

// ---------------------------------------------------------------------------
// ADSR application tests
// ---------------------------------------------------------------------------

func TestApplyCarrierADSR(t *testing.T) {
	p := Note("C4").Attack(0.0).Decay(0.3).Sustain(0.5).Release(1.2)
	inst := p.defaultInstrument()
	p.applyCarrierADSR(inst)

	if inst.Op2.Attack != 15 {
		t.Errorf("attack rate = %d, want 15 (instant)", inst.Op2.Attack)
	}
	if inst.Op2.Decay != 12 {
		t.Errorf("decay rate = %d, want 12 (~0.30s)", inst.Op2.Decay)
	}
	if inst.Op2.Sustain != 8 {
		t.Errorf("sustain level = %d, want 8 (inverted 0.5)", inst.Op2.Sustain)
	}
	if inst.Op2.Release != 8 {
		t.Errorf("release rate = %d, want 8 (~1.2s)", inst.Op2.Release)
	}
}

func TestApplyFMParams(t *testing.T) {
	p := Note("C2").FM(6).FMH(2).Feedback(5)
	inst := p.defaultInstrument()
	p.applyFMParams(inst)

	// FM(6) -> level = 63 - 6*6.3 = 63 - 37.8 = 25.2 -> 25
	if inst.Op1.Level != 25 {
		t.Errorf("modulator level = %d, want 25", inst.Op1.Level)
	}

	// FMH(2) -> multiplier register 2
	if inst.Op1.Multiply != 2 {
		t.Errorf("modulator multiply = %d, want 2", inst.Op1.Multiply)
	}

	// Feedback(5)
	if inst.Feedback != 5 {
		t.Errorf("feedback = %d, want 5", inst.Feedback)
	}
}

// ---------------------------------------------------------------------------
// Convenience constructor tests
// ---------------------------------------------------------------------------

func TestConstructorS(t *testing.T) {
	p := S("desert_bass")
	if p.sound != "desert_bass" {
		t.Errorf("sound = %q, want desert_bass", p.sound)
	}
}

func TestConstructorN(t *testing.T) {
	p := N("0 2 4 7")
	if p.nStep != "0 2 4 7" {
		t.Errorf("nStep = %q, want '0 2 4 7'", p.nStep)
	}
}

func TestAllSignalShapes(t *testing.T) {
	shapes := []struct {
		name  string
		build func() *Signal
		want  voice.WaveShape
	}{
		{"Sine", Sine, voice.ShapeSine},
		{"Tri", Tri, voice.ShapeTriangle},
		{"Saw", Saw, voice.ShapeSaw},
		{"Square", Square, voice.ShapeSquare},
	}

	for _, tt := range shapes {
		s := tt.build()
		if s.shape != tt.want {
			t.Errorf("%s(): shape = %v, want %v", tt.name, s.shape, tt.want)
		}
		// Default rate should be 1 Hz
		if s.rateHz != 1.0 {
			t.Errorf("%s(): rateHz = %f, want 1.0", tt.name, s.rateHz)
		}
		// Default range [0, 1]
		if s.lo != 0.0 || s.hi != 1.0 {
			t.Errorf("%s(): range = [%f, %f], want [0.0, 1.0]", tt.name, s.lo, s.hi)
		}
	}
}
