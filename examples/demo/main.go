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
	"github.com/jebbisson/spice-synth/patches"
	"github.com/jebbisson/spice-synth/stream"
)

func main() {
	fmt.Println("SpiceSynth: DSL Demo")
	fmt.Println("Generating 5 seconds of audio to 'output.raw' (S16LE Stereo)...")

	// 1. Initialize the stream.
	s := stream.New(44100)
	s.Voices().LoadBank("spice", patches.Spice())
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
