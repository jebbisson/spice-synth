// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// basic demonstrates the simplest possible SpiceSynth DSL usage.
// It renders a single note to a raw PCM file (no ebiten dependency).
//
// Usage: go run ./examples/basic
// Play:  ffplay -f s16le -ar 44100 -ac 2 basic_output.raw
package main

import (
	"fmt"
	"os"

	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/patches"
	"github.com/jebbisson/spice-synth/stream"
)

func main() {
	fmt.Println("SpiceSynth Basic Demo (DSL)")

	// 1. Initialize stream and load instruments.
	s := stream.New(44100)
	s.Voices().LoadBank("spice", patches.Spice())

	// 2. Define a single grungy bass note using the DSL.
	bass := dsl.Note("C2").S("desert_bass").
		FM(6).Feedback(6).
		Attack(0.0).Sustaining(true)

	// 3. Play it on channel 0.
	if err := bass.Play(s, 0); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// 4. Render 3 seconds of audio to file.
	f, err := os.Create("basic_output.raw")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer f.Close()

	buf := make([]byte, 44100*4) // 1 second per read
	for i := 0; i < 3; i++ {
		n, err := s.Read(buf)
		if err != nil {
			fmt.Printf("Error reading stream: %v\n", err)
			break
		}
		f.Write(buf[:n])
	}

	fmt.Println("Wrote 3 seconds to 'basic_output.raw'")
	fmt.Println("Play with: ffplay -f s16le -ar 44100 -ac 2 basic_output.raw")
}
