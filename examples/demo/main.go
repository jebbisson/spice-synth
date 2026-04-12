package main

import (
	"fmt"
	"os"

	"github.com/jebbisson/spice-synth/patches"
	"github.com/jebbisson/spice-synth/sequencer"
	"github.com/jebbisson/spice-synth/stream"
)

func main() {
	fmt.Println("SpiceSynth: Style Demo")
	fmt.Println("Generating 5 seconds of audio to 'output.raw' (S16LE Stereo)...")

	// 1. Initialize the stream
	s := stream.New(44100)

	// 2. Load Spice instrument bank
	s.Voices().LoadBank("spice", patches.Spice())

	// 3. Set up a simple multi-channel pattern
	seq := s.Sequencer()
	seq.SetBPM(110)

	// Bassline: C2 -> Eb2 -> F2 -> G2
	bass := sequencer.NewPattern(16).
		Instrument("desert_bass").
		Note(0, "C2").
		Note(4, "Eb2").
		Note(8, "F2").
		Note(12, "G2")

	// Lead: Simple melody
	lead := sequencer.NewPattern(16).
		Instrument("mystic_lead").
		Note(0, "C4").
		Note(2, "Eb4").
		Note(4, "F4").
		Note(7, "G4")

	// Percussion: Four-on-the-floor
	perc := sequencer.NewPattern(16).
		Instrument("fm_perc").
		Hit(0).Hit(4).Hit(8).Hit(12)

	seq.SetPattern(0, bass)
	seq.SetPattern(1, lead)
	seq.SetPattern(2, perc)

	// 4. Render to file (5 seconds)
	f, err := os.Create("output.raw")
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer f.Close()

	// Write 5 seconds of audio
	// 44100 samples/sec * 4 bytes/sample = 176,400 bytes/sec
	buf := make([]byte, 44100*4)
	for i := 0; i < 5; i++ {
		n, err := s.Read(buf)
		if err != nil {
			fmt.Printf("Error reading stream: %v\n", err)
			break
		}
		f.Write(buf[:n])
	}

	fmt.Println("Done! You can play 'output.raw' using:")
	fmt.Println("ffplay -f s16le -ar 44100 -ac 2 output.raw")
}
