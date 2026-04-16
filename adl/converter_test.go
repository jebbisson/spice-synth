// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package adl

import (
	"math"
	"os"
	"testing"

	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/stream"
)

func TestConvertDUNE1Subsong2(t *testing.T) {
	f, err := os.Open("../examples/adl/DUNE1.ADL")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	defer f.Close()

	adlFile, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := Convert(adlFile, 2, 30) // 30 seconds max
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if result.Song == nil {
		t.Fatal("song is nil")
	}

	if len(result.Channels) == 0 {
		t.Fatal("no active channels")
	}

	if result.TicksUsed == 0 {
		t.Fatal("no ticks used")
	}

	tracks := result.Song.Tracks()
	if len(tracks) == 0 {
		t.Fatal("no tracks in song")
	}

	t.Logf("Converted subsong 2: %d channels, %d ticks, BPM=%.0f",
		len(result.Channels), result.TicksUsed, result.BPM)
	t.Logf("Active channels: %v", result.Channels)

	totalEvents := 0
	for _, tr := range tracks {
		events := tr.Events()
		totalEvents += len(events)
		t.Logf("  Channel %d: %d events", tr.Channel(), len(events))
	}
	t.Logf("Total events: %d", totalEvents)

	if totalEvents == 0 {
		t.Fatal("no events in any track")
	}
}

func TestConvertInvalidSubsong(t *testing.T) {
	f, err := os.Open("../examples/adl/DUNE1.ADL")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	defer f.Close()

	adlFile, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = Convert(adlFile, -1, 10)
	if err == nil {
		t.Error("expected error for invalid subsong")
	}

	_, err = Convert(adlFile, 999, 10)
	if err == nil {
		t.Error("expected error for out of range subsong")
	}
}

func TestConvertNilFile(t *testing.T) {
	_, err := Convert(nil, 0, 10)
	if err == nil {
		t.Error("expected error for nil file")
	}
}

func TestFreqToNoteName(t *testing.T) {
	tests := []struct {
		freq float64
		want string
	}{
		{440.0, "A4"},
		{261.63, "C4"},
		{329.63, "E4"},
		{0, "C0"},
		{-1, "C0"},
	}

	for _, tt := range tests {
		got := freqToNoteName(tt.freq)
		if got != tt.want {
			t.Errorf("freqToNoteName(%f) = %q, want %q", tt.freq, got, tt.want)
		}
	}
}

func TestRegToFreq(t *testing.T) {
	// Test with known OPL2 register values
	// Block 4, F-number 0x134 (308) = C in octave 4
	// From the ADL freqTable: freqTable[0] = 0x134
	fnum := uint16(0x134)
	block := uint8(4)
	regAx := uint8(fnum & 0xFF)
	regBx := uint8(block<<2) | uint8((fnum>>8)&0x03)

	freq := regToFreq(regAx, regBx)
	if freq < 200 || freq > 400 {
		t.Errorf("regToFreq for C4 gave %f Hz, expected ~261 Hz", freq)
	}

	// Zero f-number should return 0
	if regToFreq(0, 0) != 0 {
		t.Error("expected 0 for zero f-number")
	}
}

// TestConvertRoundTripAudio verifies that a converted Song, when played
// through a stream, produces non-zero PCM audio. This is the end-to-end
// validation that the converter output actually drives the OPL2 chip.
func TestConvertRoundTripAudio(t *testing.T) {
	f, err := os.Open("../examples/adl/DUNE1.ADL")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	defer f.Close()

	adlFile, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := Convert(adlFile, 2, 30)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	// Create a stream and play the converted song through it.
	const sampleRate = 44100
	s := stream.New(sampleRate)
	defer s.Close()

	if err := result.Song.Play(s); err != nil {
		t.Fatalf("Song.Play: %v", err)
	}

	// Read ~0.5 seconds of audio (enough for notes to sound).
	// 44100 samples/sec * 4 bytes/frame * 0.5 sec = 88200 bytes
	buf := make([]byte, 88200)
	n, err := s.Read(buf)
	if err != nil {
		t.Fatalf("stream.Read: %v", err)
	}
	if n == 0 {
		t.Fatal("stream returned 0 bytes")
	}

	// Check for non-zero samples (signed 16-bit little-endian stereo).
	var maxAmp float64
	nonZeroSamples := 0
	for i := 0; i < n-1; i += 2 {
		sample := int16(buf[i]) | int16(buf[i+1])<<8
		amp := math.Abs(float64(sample))
		if amp > maxAmp {
			maxAmp = amp
		}
		if sample != 0 {
			nonZeroSamples++
		}
	}

	t.Logf("Read %d bytes, max amplitude: %.0f, non-zero samples: %d/%d",
		n, maxAmp, nonZeroSamples, n/2)

	if maxAmp == 0 {
		t.Fatal("all samples are zero — converted song produced no audio")
	}

	// Expect at least some reasonable amplitude (not just noise floor).
	if maxAmp < 100 {
		t.Errorf("max amplitude %.0f is suspiciously low, expected audible output", maxAmp)
	}

	// Expect a meaningful fraction of samples to be non-zero.
	nonZeroRatio := float64(nonZeroSamples) / float64(n/2)
	if nonZeroRatio < 0.1 {
		t.Errorf("only %.1f%% of samples are non-zero, expected more audio activity", nonZeroRatio*100)
	}

	t.Logf("Round-trip audio validation passed: %.1f%% non-zero, peak amplitude %.0f/32768",
		nonZeroRatio*100, maxAmp)
}

// TestConvertVsADLPlayer compares audio output from the original ADL player
// against the converted DSL song. Both should produce non-silent audio for
// the same subsong. We don't expect sample-identical output (different code
// paths) but both must be audibly active.
func TestConvertVsADLPlayer(t *testing.T) {
	f, err := os.Open("../examples/adl/DUNE1.ADL")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	defer f.Close()

	adlFile, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// --- ADL player output ---
	adlPlayer := NewPlayer(44100, adlFile)
	adlPlayer.SetSubsong(2)
	adlPlayer.Play()
	defer adlPlayer.Close()

	adlBuf := make([]byte, 88200) // ~0.5s
	adlN, err := adlPlayer.Read(adlBuf)
	if err != nil {
		t.Fatalf("ADL player Read: %v", err)
	}

	var adlMax float64
	for i := 0; i < adlN-1; i += 2 {
		sample := int16(adlBuf[i]) | int16(adlBuf[i+1])<<8
		amp := math.Abs(float64(sample))
		if amp > adlMax {
			adlMax = amp
		}
	}

	// --- DSL converted output ---
	result, err := Convert(adlFile, 2, 30)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	s := stream.New(44100)
	defer s.Close()
	if err := result.Song.Play(s); err != nil {
		t.Fatalf("Song.Play: %v", err)
	}

	dslBuf := make([]byte, 88200)
	dslN, err := s.Read(dslBuf)
	if err != nil {
		t.Fatalf("DSL stream Read: %v", err)
	}

	var dslMax float64
	for i := 0; i < dslN-1; i += 2 {
		sample := int16(dslBuf[i]) | int16(dslBuf[i+1])<<8
		amp := math.Abs(float64(sample))
		if amp > dslMax {
			dslMax = amp
		}
	}

	t.Logf("ADL player: %d bytes, peak amplitude %.0f", adlN, adlMax)
	t.Logf("DSL song:   %d bytes, peak amplitude %.0f", dslN, dslMax)

	if adlMax == 0 {
		t.Error("ADL player produced silent output")
	}
	if dslMax == 0 {
		t.Error("DSL song produced silent output")
	}

	// Both should have significant amplitude.
	if adlMax > 0 && dslMax > 0 {
		t.Logf("Both ADL player and DSL converter produce audible output — comparison passed")
	}
}

func TestConvertDUNE1Subsong6PreservesRepeatedChannel0Retriggers(t *testing.T) {
	f, err := os.Open("../examples/adl/DUNE1.ADL")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	defer f.Close()

	adlFile, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	result, err := Convert(adlFile, 6, 20)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	var ch0 *dsl.Track
	for _, tr := range result.Song.Tracks() {
		if tr.Channel() == 0 {
			ch0 = tr
			break
		}
	}
	if ch0 == nil {
		t.Fatal("converted song is missing channel 0 track")
	}

	noteOns := 0
	lastTick := -1
	shortSpacingCount := 0
	for _, ev := range ch0.Events() {
		if ev.Type != dsl.TrackNoteOn {
			continue
		}
		noteOns++
		if lastTick >= 0 && ev.Tick-lastTick <= 20 {
			shortSpacingCount++
		}
		lastTick = ev.Tick
	}

	if noteOns < 40 {
		t.Fatalf("expected dense repeated note-ons on channel 0, got %d", noteOns)
	}
	if shortSpacingCount < 20 {
		t.Fatalf("expected many tightly spaced retriggers on channel 0, got %d", shortSpacingCount)
	}
}
