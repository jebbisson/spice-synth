// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package dsl

import (
	"math"
	"testing"

	"github.com/jebbisson/spice-synth/stream"
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

// ---------------------------------------------------------------------------
// Velocity tests
// ---------------------------------------------------------------------------

func TestApplyVelocity(t *testing.T) {
	tests := []struct {
		name      string
		vel       float64
		baseLevel uint8
		wantLevel uint8
	}{
		{"full velocity", 1.0, 0, 0},
		{"half velocity", 0.5, 0, 31},   // (1-0.5)*63 ≈ 31 additional attenuation
		{"zero velocity", 0.0, 0, 63},   // silent
		{"full with base", 1.0, 10, 10}, // no change to base
		{"half with base", 0.5, 10, 41}, // 10 + 31 = 41
		{"clamped at 63", 0.5, 50, 63},  // 50 + 31 > 63, clamped
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Note("C4").Velocity(tt.vel)
			inst := p.defaultInstrument()
			inst.Op2.Level = tt.baseLevel
			p.applyVelocity(inst)

			if inst.Op2.Level != tt.wantLevel {
				t.Errorf("level = %d, want %d", inst.Op2.Level, tt.wantLevel)
			}
		})
	}
}

func TestVelocityNotSetDoesNothing(t *testing.T) {
	p := Note("C4")
	inst := p.defaultInstrument()
	origLevel := inst.Op2.Level
	p.applyVelocity(inst)
	if inst.Op2.Level != origLevel {
		t.Errorf("level changed from %d to %d without velocity set", origLevel, inst.Op2.Level)
	}
}

// ---------------------------------------------------------------------------
// Instrument registration tests
// ---------------------------------------------------------------------------

func TestHasOverrides(t *testing.T) {
	// No overrides
	p := Note("C4").S("sine")
	if p.hasOverrides() {
		t.Error("plain pattern should not have overrides")
	}

	// With attack override
	p2 := Note("C4").Attack(0.1)
	if !p2.hasOverrides() {
		t.Error("pattern with attack should have overrides")
	}

	// With velocity override
	p3 := Note("C4").Velocity(0.5)
	if !p3.hasOverrides() {
		t.Error("pattern with velocity should have overrides")
	}
}

// ---------------------------------------------------------------------------
// Integration tests: full Play() path through stream.Stream
// ---------------------------------------------------------------------------

func TestPlayDefaultInstrument(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	// Play a default sine note — this should register the instrument and
	// create a sequencer pattern without error.
	err := Note("C4").Play(s, 0)
	if err != nil {
		t.Fatalf("Play() failed: %v", err)
	}

	// Verify the instrument was registered in the voice manager.
	_, err = s.Voices().GetInstrument("dsl_default")
	if err != nil {
		t.Errorf("default instrument not registered: %v", err)
	}
}

func TestPlayRawWaveform(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	err := Note("C4").S("halfsine").Play(s, 0)
	if err != nil {
		t.Fatalf("Play() failed: %v", err)
	}

	// Raw waveforms with no overrides use their name directly.
	_, err = s.Voices().GetInstrument("halfsine")
	if err != nil {
		t.Errorf("halfsine instrument not registered: %v", err)
	}
}

func TestPlayWithOverridesCreatesUniqueInstrument(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	err := Note("C4").S("sine").FM(6).Play(s, 0)
	if err != nil {
		t.Fatalf("Play() failed: %v", err)
	}

	// With overrides, instrument should be registered under a unique name.
	_, err = s.Voices().GetInstrument("_dsl_sine_ch0")
	if err != nil {
		t.Errorf("overridden instrument not registered: %v", err)
	}
}

func TestPlayNamedInstrumentFromBank(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	// Pre-load a named instrument into the voice manager bank.
	testInst := &voice.Instrument{
		Name: "test_bass",
		Op1: voice.Operator{
			Attack: 15, Decay: 4, Sustain: 0, Release: 8,
			Level: 20, Multiply: 1, Waveform: 0, Sustaining: true,
		},
		Op2: voice.Operator{
			Attack: 15, Decay: 4, Sustain: 2, Release: 8,
			Level: 0, Multiply: 1, Waveform: 0, Sustaining: true,
		},
		Feedback: 3, Connection: 0,
	}
	s.Voices().LoadBank("test", []*voice.Instrument{testInst})

	// Play using the named instrument with overrides.
	err := Note("C2").S("test_bass").Attack(0.0).Play(s, 0)
	if err != nil {
		t.Fatalf("Play() failed: %v", err)
	}

	// Should have registered an overridden copy.
	inst, err := s.Voices().GetInstrument("_dsl_test_bass_ch0")
	if err != nil {
		t.Fatalf("overridden instrument not registered: %v", err)
	}
	// The override should have set attack to instant (15).
	if inst.Op2.Attack != 15 {
		t.Errorf("attack = %d, want 15", inst.Op2.Attack)
	}
	// But the original should remain untouched.
	origInst, err := s.Voices().GetInstrument("test_bass")
	if err != nil {
		t.Fatalf("original instrument lost: %v", err)
	}
	if origInst.Op2.Attack != 15 {
		t.Errorf("original attack mutated: %d", origInst.Op2.Attack)
	}
}

func TestPlayProducesAudibleOutput(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	// Play a note using PlayDirect (bypasses sequencer timing complexity).
	err := Note("C4").FM(6).Feedback(4).Attack(0.0).Sustaining(true).PlayDirect(s, 0)
	if err != nil {
		t.Fatalf("PlayDirect() failed: %v", err)
	}

	// Generate some audio and check that it's not all zeros.
	buf := make([]byte, 44100*4) // 1 second of stereo 16-bit PCM
	n, err := s.Read(buf)
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	if n == 0 {
		t.Fatal("Read() returned 0 bytes")
	}

	// Check for non-zero samples (skip the first few frames for fade-in).
	hasNonZero := false
	for i := 1000; i < n-1; i += 2 {
		sample := int16(buf[i])<<8 | int16(buf[i-1])
		if sample != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("PlayDirect produced all-zero audio — no audible output")
	}
}

func TestPlaySequencedProducesAudio(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	// Play via sequencer path.
	err := Note("C4").FM(6).Feedback(4).Attack(0.0).Sustaining(true).Play(s, 0)
	if err != nil {
		t.Fatalf("Play() failed: %v", err)
	}

	// Generate audio — the sequencer should trigger the note.
	buf := make([]byte, 44100*4) // 1 second
	n, err := s.Read(buf)
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	if n == 0 {
		t.Fatal("Read() returned 0 bytes")
	}

	// Check for non-zero samples.
	hasNonZero := false
	for i := 1000; i < n-1; i += 2 {
		sample := int16(buf[i])<<8 | int16(buf[i-1])
		if sample != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("sequenced Play() produced all-zero audio — instrument likely not registered")
	}
}

func TestPlayMultipleChannels(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	// Play different notes on different channels.
	if err := Note("C2").FM(6).Play(s, 0); err != nil {
		t.Fatalf("Play ch0 failed: %v", err)
	}
	if err := Note("E4").FM(3).Play(s, 1); err != nil {
		t.Fatalf("Play ch1 failed: %v", err)
	}
	if err := Note("G4").Play(s, 2); err != nil {
		t.Fatalf("Play ch2 failed: %v", err)
	}

	// Just verify it doesn't panic and produces some output.
	buf := make([]byte, 4410*4) // 0.1 second
	n, err := s.Read(buf)
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	if n == 0 {
		t.Fatal("Read() returned 0 bytes")
	}
}

// ---------------------------------------------------------------------------
// Song and Track tests
// ---------------------------------------------------------------------------

func TestSongBasic(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	// Create a simple song with one instrument and one track.
	inst := &voice.Instrument{
		Name: "test_inst",
		Op1:  voice.Operator{Attack: 15, Decay: 0, Sustain: 0, Release: 8, Level: 63, Multiply: 1, Sustaining: true},
		Op2:  voice.Operator{Attack: 15, Decay: 4, Sustain: 2, Release: 8, Level: 0, Multiply: 1, Sustaining: true},
	}

	track := NewTrack(0).SetInstrument("test_inst")
	track.NoteOnOff(0, "C4", 8)

	song := NewSong(120).
		AddInstrument(inst).
		AddTrack(track)

	err := song.Play(s)
	if err != nil {
		t.Fatalf("Song.Play() failed: %v", err)
	}

	// Generate audio.
	buf := make([]byte, 44100*4)
	n, err := s.Read(buf)
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
	if n == 0 {
		t.Fatal("Read() returned 0 bytes")
	}
}

func TestSongMultiTrack(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	inst := &voice.Instrument{
		Name: "multi_inst",
		Op1:  voice.Operator{Attack: 15, Decay: 0, Sustain: 0, Release: 8, Level: 40, Multiply: 1, Sustaining: true},
		Op2:  voice.Operator{Attack: 15, Decay: 4, Sustain: 2, Release: 8, Level: 0, Multiply: 1, Sustaining: true},
	}

	t1 := NewTrack(0).SetInstrument("multi_inst")
	t1.NoteOnOff(0, "C2", 16)

	t2 := NewTrack(1).SetInstrument("multi_inst")
	t2.NoteOnOff(0, "E4", 8)
	t2.NoteOnOff(8, "G4", 8)

	song := NewSong(120).
		AddInstrument(inst).
		AddTrack(t1).
		AddTrack(t2)

	err := song.Play(s)
	if err != nil {
		t.Fatalf("Song.Play() failed: %v", err)
	}

	// Generate some audio — just verify no panic.
	buf := make([]byte, 4410*4)
	_, err = s.Read(buf)
	if err != nil {
		t.Fatalf("Read() failed: %v", err)
	}
}

func TestTrackCompileAutoLength(t *testing.T) {
	track := NewTrack(0).SetInstrument("test")
	track.NoteOnOff(0, "C4", 8)
	track.NoteOnOff(10, "E4", 4)

	pat, err := track.compile()
	if err != nil {
		t.Fatalf("compile() failed: %v", err)
	}

	// Auto-length should be max(tick+1) = max(0+1, 10+1, 8+1, 14+1) = 15
	// (NoteOff at tick 14 is the last event)
	if pat.Steps < 15 {
		t.Errorf("Steps = %d, want >= 15", pat.Steps)
	}

	// Should have 4 events: 2 NoteOn + 2 NoteOff
	if len(pat.Events) != 4 {
		t.Errorf("events count = %d, want 4", len(pat.Events))
	}
}

func TestTrackVolumeChange(t *testing.T) {
	track := NewTrack(0).SetInstrument("test")
	track.NoteOn(0, "C4")
	track.SetVolumeAt(4, 0.5)

	pat, err := track.compile()
	if err != nil {
		t.Fatalf("compile() failed: %v", err)
	}

	// Should have a NoteOn and a VolumeChange event
	if len(pat.Events) != 2 {
		t.Fatalf("events count = %d, want 2", len(pat.Events))
	}
}

func TestTrackInstrumentChange(t *testing.T) {
	track := NewTrack(0).SetInstrument("inst_a")
	track.NoteOnOff(0, "C4", 8)
	track.SetInstrumentAt(8, "inst_b")
	track.NoteOnOff(8, "E4", 8)

	pat, err := track.compile()
	if err != nil {
		t.Fatalf("compile() failed: %v", err)
	}

	// 2 NoteOn + 2 NoteOff + 1 InstrumentChange = 5
	if len(pat.Events) != 5 {
		t.Errorf("events count = %d, want 5", len(pat.Events))
	}
}

func TestStackCreatesMultiTrackSong(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	// Pre-register instruments that Stack's patterns will reference.
	// Stack with default instruments will use "dsl_default".
	song := Stack(
		Note("C2"),
		Note("E4"),
		Note("G4"),
	)

	if len(song.Tracks()) != 3 {
		t.Fatalf("Stack produced %d tracks, want 3", len(song.Tracks()))
	}

	// Verify channel assignments.
	for i, tr := range song.Tracks() {
		if tr.Channel() != i {
			t.Errorf("track %d channel = %d, want %d", i, tr.Channel(), i)
		}
	}
}

func TestSeqCreatesSequentialSong(t *testing.T) {
	song := Seq(
		Note("C4"),
		Note("E4"),
		Note("G4"),
	)

	if len(song.Tracks()) != 1 {
		t.Fatalf("Seq produced %d tracks, want 1", len(song.Tracks()))
	}

	track := song.Tracks()[0]
	// Should have events for 3 notes: each gets NoteOn + NoteOff + InstrumentChange
	// Actually: 3 InstrumentChange + 3 NoteOn + 3 NoteOff = 9
	events := track.Events()
	noteOns := 0
	noteOffs := 0
	for _, e := range events {
		switch e.Type {
		case TrackNoteOn:
			noteOns++
		case TrackNoteOff:
			noteOffs++
		}
	}
	if noteOns != 3 {
		t.Errorf("noteOns = %d, want 3", noteOns)
	}
	if noteOffs != 3 {
		t.Errorf("noteOffs = %d, want 3", noteOffs)
	}
}

func TestSongInvalidChannel(t *testing.T) {
	s := stream.New(44100)
	defer s.Close()

	song := NewSong(120)
	song.AddTrack(NewTrack(10)) // invalid channel

	err := song.Play(s)
	if err == nil {
		t.Error("expected error for invalid channel 10")
	}
}

func TestSilencePattern(t *testing.T) {
	p := Silence()
	if p.noteSet {
		t.Error("Silence() should not have noteSet")
	}
	if p.sound != "" {
		t.Errorf("Silence() sound = %q, want empty", p.sound)
	}
}
