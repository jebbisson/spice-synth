// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/stream"
)

func main() {
	path := filepath.Join("examples", "instruments_yaml", "instruments.yaml")
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("missing sample YAML at %s: %v\n", path, err)
		os.Exit(1)
	}

	s := stream.New(44100)
	defer s.Close()

	if err := stream.LoadInstrumentsFromYAML(s, path); err != nil {
		fmt.Printf("failed to load YAML instruments: %v\n", err)
		os.Exit(1)
	}

	if err := dsl.Note("C2").Sound("desert_bass.default").Play(s, 0); err != nil {
		fmt.Printf("failed to play note: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create("instruments_yaml_output.raw")
	if err != nil {
		fmt.Printf("failed to create output: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	buf := make([]byte, 44100*4)
	for i := 0; i < 2; i++ {
		n, err := s.Read(buf)
		if err != nil {
			fmt.Printf("stream read failed: %v\n", err)
			os.Exit(1)
		}
		if _, err := f.Write(buf[:n]); err != nil {
			fmt.Printf("output write failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("wrote instruments_yaml_output.raw")
}
