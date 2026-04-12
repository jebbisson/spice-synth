// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package main

import (
	"fmt"
	"github.com/jebbisson/spice-synth/stream"
)

func main() {
	fmt.Println("SpiceSynth Basic Demo")

	// Initialize the stream
	s := stream.New(44100)

	fmt.Printf("Stream initialized with sample rate: %d\n", 44100)
	_ = s
}
