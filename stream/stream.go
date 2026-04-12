// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package stream

import (
	"github.com/jebbisson/spice-synth/chip"
	"github.com/jebbisson/spice-synth/sequencer"
	"github.com/jebbisson/spice-synth/voice"
)

// subBlockSize is the number of sample frames processed between modulator
// ticks. Smaller values give smoother modulation at the cost of more
// overhead. 64 frames ≈ 1.45ms at 44100 Hz.
const subBlockSize = 64

// Stream produces PCM audio from the synth engine.
type Stream struct {
	chip       *chip.OPL3
	voices     *voice.Manager
	seq        *sequencer.Sequencer
	sampleRate int
	masterVol  float64
	gain       float64

	// Fade-in state to prevent the initial volume spike when the chip
	// first starts producing samples.
	fadeInSamples int // total samples in the fade-in ramp
	samplesRead   int // samples read so far
}

// New creates a new audio stream at the given sample rate.
func New(sampleRate int) *Stream {
	c := chip.New(uint32(sampleRate))
	v := voice.NewManager(c, sampleRate)
	return &Stream{
		chip:          c,
		voices:        v,
		seq:           sequencer.New(v, 120.0, sampleRate),
		sampleRate:    sampleRate,
		masterVol:     1.0,
		gain:          5.0,
		fadeInSamples: sampleRate / 10, // 100ms fade-in
	}
}

// Read fills b with signed 16-bit stereo little-endian PCM data.
//
// Audio is generated in small sub-blocks (64 frames). Between each sub-block
// the sequencer is advanced and all active modulators are ticked, so that
// LFOs, ramps and envelopes update at roughly 689 Hz (44100/64) rather than
// once per buffer.
func (s *Stream) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	// Each sample frame is 4 bytes (2 channels × 2 bytes per channel).
	totalFrames := len(b) / 4
	if totalFrames == 0 {
		return 0, nil
	}

	byteOffset := 0
	framesLeft := totalFrames

	for framesLeft > 0 {
		// Determine sub-block size (last block may be smaller).
		n := subBlockSize
		if n > framesLeft {
			n = framesLeft
		}

		// 1. Advance sequencer — fires any events in this sub-block window.
		//    NoteOn writes the instrument's raw register values (including
		//    carrier level at the patch default). We immediately tick
		//    modulators with 0 samples so they override those raw values
		//    before any audio is generated.
		s.seq.Advance(n)
		s.voices.Tick(0) // apply current modulator values without advancing time

		// 2. Tick modulators by the real sub-block duration.
		s.voices.Tick(n)

		// 3. Generate audio samples for this sub-block.
		samples, err := s.chip.GenerateSamples(n)
		if err != nil {
			return byteOffset, err
		}

		// 4. Convert int16 stereo samples to bytes with gain, master vol,
		//    and fade-in applied.
		for i, sample := range samples {
			idx := byteOffset + i*2

			// Fade-in ramp to prevent initial volume spike.
			fadeScale := 1.0
			frameIdx := s.samplesRead + i/2 // frames, not L/R samples
			if frameIdx < s.fadeInSamples {
				fadeScale = float64(frameIdx) / float64(s.fadeInSamples)
			}

			scaled := float64(sample) * s.gain * s.masterVol * fadeScale

			// Clip to int16 range.
			if scaled > 32767 {
				scaled = 32767
			} else if scaled < -32768 {
				scaled = -32768
			}

			out := int16(scaled)
			if idx+1 < len(b) {
				b[idx] = byte(out & 0xFF)
				b[idx+1] = byte(out >> 8)
			}
		}

		s.samplesRead += n
		byteOffset += n * 4 // 4 bytes per stereo frame
		framesLeft -= n
	}

	return totalFrames * 4, nil
}

// Sequencer returns the sequencer for pattern manipulation.
func (s *Stream) Sequencer() *sequencer.Sequencer {
	return s.seq
}

// Voices returns the voice manager for direct instrument control.
func (s *Stream) Voices() *voice.Manager {
	return s.voices
}

// SetMasterVolume sets the master output volume (0.0 - 1.0).
func (s *Stream) SetMasterVolume(v float64) {
	s.masterVol = v
}

// Close releases the underlying OPL3 chip resources. The Stream must not be
// used after calling Close.
func (s *Stream) Close() {
	s.chip.Close()
}
