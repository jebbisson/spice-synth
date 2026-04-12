package stream

import (
	"fmt"
	"github.com/jebbisson/spice-synth/chip"
	"github.com/jebbisson/spice-synth/sequencer"
	"github.com/jebbisson/spice-synth/voice"
)

// Stream produces PCM audio from the synth engine.
type Stream struct {
	chip       *chip.OPL3
	voices     *voice.Manager
	seq        *sequencer.Sequencer
	sampleRate int
	masterVol  float64
	gain       float64
}

// New creates a new audio stream at the given sample rate.
func New(sampleRate int) *Stream {
	c := chip.New(uint32(sampleRate))
	v := voice.NewManager(c)
	return &Stream{
		chip:       c,
		voices:     v,
		seq:        sequencer.New(v, 120.0, sampleRate),
		sampleRate: sampleRate,
		masterVol:  1.0,
		gain:       1000.0,
	}
}

// Read fills b with signed 16-bit stereo little-endian PCM data.
func (s *Stream) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	// Each sample frame is 4 bytes (2 channels * 2 bytes per channel)
	numFrames := len(b) / 4
	if numFrames == 0 {
		return 0, nil
	}

	s.seq.Advance(numFrames)

	samples, err := s.chip.GenerateSamples(numFrames)
	if err != nil {
		return 0, err
	}

	// DEBUG: Check if samples are silent
	for i := 0; i < len(samples) && i < 10; i++ {
		if samples[i] != 0 {
			fmt.Printf("[DEBUG-STREAM] Non-zero sample found: %d at index %d\n", samples[i], i)
			break
		}
	}
	if samples[0] == 0 && samples[len(samples)-1] == 0 {
		// fmt.Println("[DEBUG-STREAM] Samples are silent") // Too noisy, but maybe for first few frames
	}

	// Convert int16 slice to byte slice (little endian) with master volume and gain applied
	for i, sample := range samples {
		idx := i * 2
		// Apply gain first to boost signal, then master volume scaling
		scaledSampleVal := float64(sample) * s.gain * s.masterVol

		// Clip to avoid overflow
		if scaledSampleVal > 32767 {
			scaledSampleVal = 32767
		} else if scaledSampleVal < -32768 {
			scaledSampleVal = -32768
		}

		scaledSample := int16(scaledSampleVal)
		if idx+1 < len(b) {
			b[idx] = byte(scaledSample & 0xFF)
			b[idx+1] = byte(scaledSample >> 8)
		}
	}

	return numFrames * 4, nil
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
