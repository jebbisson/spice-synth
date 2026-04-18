// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// demo renders a multi-channel arrangement to a raw PCM file using the DSL.
// Three voices play simultaneously: a bassline, a lead melody, and percussion.
//
// Usage: go run ./examples/demo
// Play:  ffplay -f s16le -ar 44100 -ac 2 output.raw
package main

import (
	"fmt"
	"os"

	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/stream"
	"github.com/jebbisson/spice-synth/voice"
)

func main() {
	fmt.Println("SpiceSynth: DSL Demo")
	fmt.Println("Generating 5 seconds of audio to 'output.raw' (S16LE Stereo)...")

	// 1. Initialize the stream.
	s := stream.New(44100)
	defer s.Close()
	s.Voices().LoadBank("spice", spiceBank())
	s.Sequencer().SetBPM(110)

	// 2. Define voices using the DSL.
	//
	// Bassline: grungy desert bass with heavy feedback.
	bass := dsl.Note("C2").S("desert_bass").
		FM(6).Feedback(6).
		Attack(0.0).Sustaining(true)

	// Lead: nasal cutting melody.
	lead := dsl.Note("C4").S("mystic_lead").
		Attack(0.0).Sustaining(true)

	// Percussion: short metallic hit.
	perc := dsl.Note("C2").S("fm_perc")

	// 3. Play each on a separate channel.
	if err := bass.Play(s, 0); err != nil {
		fmt.Printf("bass error: %v\n", err)
		return
	}
	if err := lead.Play(s, 1); err != nil {
		fmt.Printf("lead error: %v\n", err)
		return
	}
	if err := perc.Play(s, 2); err != nil {
		fmt.Printf("perc error: %v\n", err)
		return
	}

	// 4. Render to file (5 seconds).
	f, err := os.Create("output.raw")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer f.Close()

	buf := make([]byte, 44100*4) // 1 second per read
	for i := 0; i < 5; i++ {
		n, err := s.Read(buf)
		if err != nil {
			fmt.Printf("Error reading stream: %v\n", err)
			break
		}
		f.Write(buf[:n])
	}

	fmt.Println("Done! Play with:")
	fmt.Println("  ffplay -f s16le -ar 44100 -ac 2 output.raw")
}

func spiceBank() []*voice.Instrument {
	return []*voice.Instrument{
		{
			Name:       "desert_bass",
			Op1:        voice.Operator{Attack: 15, Decay: 4, Sustain: 2, Release: 6, Level: 18, Multiply: 1, Waveform: 0, Sustaining: true},
			Op2:        voice.Operator{Attack: 15, Decay: 3, Sustain: 1, Release: 8, Level: 0, Multiply: 1, Waveform: 0, Sustaining: true},
			Feedback:   6,
			Connection: 0,
		},
		{
			Name:       "mystic_lead",
			Op1:        voice.Operator{Attack: 14, Decay: 5, Sustain: 3, Release: 5, Level: 24, Multiply: 1, Waveform: 1, Sustaining: true},
			Op2:        voice.Operator{Attack: 14, Decay: 6, Sustain: 2, Release: 7, Level: 0, Multiply: 3, Waveform: 0, Sustaining: true},
			Feedback:   3,
			Connection: 0,
		},
		{
			Name:       "fm_perc",
			Op1:        voice.Operator{Attack: 15, Decay: 8, Sustain: 15, Release: 8, Level: 14, Multiply: 6, Waveform: 0},
			Op2:        voice.Operator{Attack: 15, Decay: 9, Sustain: 15, Release: 9, Level: 0, Multiply: 1, Waveform: 0},
			Feedback:   7,
			Connection: 0,
		},
	}
}
