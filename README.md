# SpiceSynth

[![Go Reference](https://pkg.go.dev/badge/github.com/jebbisson/spice-synth.svg)](https://pkg.go.dev/github.com/jebbisson/spice-synth)
[![CI](https://github.com/jebbisson/spice-synth/actions/workflows/ci.yml/badge.svg)](https://github.com/jebbisson/spice-synth/actions/workflows/ci.yml)

SpiceSynth is a Go library for programmatic OPL2/OPL3 FM synthesis. It produces authentic AdLib-era game music in real-time and streams signed 16-bit stereo PCM audio via a standard `io.Reader` interface.

## Features

- **Authentic FM Sound** -- Cycle-accurate emulation via the [Nuked-OPL3](https://github.com/nukeykt/Nuked-OPL3) engine.
- **Fluent Sequencer** -- Define patterns and melodies with a builder-style API.
- **`io.Reader` Output** -- Drop-in compatible with audio backends like [Ebiten](https://ebitengine.org/) and [Oto](https://github.com/ebitengine/oto).
- **Zero Go Dependencies** -- The core library has no external Go dependencies; only a C compiler is required.

## Prerequisites

SpiceSynth uses CGo to compile the vendored Nuked-OPL3 C source. You need:

- **Go 1.21+**
- **A C compiler** (`gcc` or `clang`)
- **`CGO_ENABLED=1`** (the default on most systems)

| Platform | Install command |
|----------|-----------------|
| Linux (Debian/Ubuntu) | `sudo apt install build-essential` |
| macOS | `xcode-select --install` |
| Windows | Install [MinGW-w64](https://www.mingw-w64.org/) or TDM-GCC and add its `bin/` to PATH |

## Installation

```bash
go get github.com/jebbisson/spice-synth
```

## Quick Start

```go
package main

import (
    "github.com/jebbisson/spice-synth/patches"
    "github.com/jebbisson/spice-synth/sequencer"
    "github.com/jebbisson/spice-synth/stream"
)

func main() {
    // Initialize a 44.1 kHz audio stream.
    s := stream.New(44100)
    defer s.Close()

    // Load the built-in instrument bank.
    s.Voices().LoadBank("spice", patches.Spice())

    // Build a bass pattern using the fluent API.
    bass := sequencer.NewPattern(16).
        Instrument("desert_bass").
        Note(0, "C2").
        Note(4, "Eb2").
        Note(8, "F2").
        Note(12, "G2")

    s.Sequencer().SetPattern(0, bass)
    s.Sequencer().SetBPM(120)

    // 's' implements io.Reader -- pass it to your audio backend.
    // Each Read() call returns signed 16-bit stereo little-endian PCM data.
}
```

## Architecture

SpiceSynth is organized as a four-layer stack. Each layer has a single responsibility:

```
┌─────────────────────────────────────────┐
│  stream    io.Reader PCM output         │
├─────────────────────────────────────────┤
│  sequencer Tick-based pattern engine    │
├─────────────────────────────────────────┤
│  voice     Notes & instruments → OPL    │
├─────────────────────────────────────────┤
│  chip      CGo wrapper for Nuked-OPL3   │
└─────────────────────────────────────────┘
```

| Package | Description |
|---------|-------------|
| [`chip`](chip/) | CGo wrapper around Nuked-OPL3. Handles register writes and sample generation. |
| [`voice`](voice/) | Translates notes and instrument definitions into OPL2 register values. Manages 9 melodic channels. |
| [`sequencer`](sequencer/) | Tick-based timing engine. Triggers NoteOn/NoteOff events from looping patterns. |
| [`stream`](stream/) | Top-level `io.Reader`. Drives the sequencer and chip in sync with audio buffer requests. |
| [`patches`](patches/) | Predefined FM instrument banks (Spice style, GM placeholder). |

## Examples

Working examples are in the [`examples/`](examples/) directory:

### Live Playback (Ebiten)

Requires a windowing system and audio drivers.

```bash
cd examples/ebiten_player
go run main.go
```

### File Rendering (CLI)

Renders 5 seconds of audio to a raw PCM file -- useful for headless environments or CI.

```bash
go run examples/demo/main.go
ffplay -f s16le -ar 44100 -ac 2 output.raw
```

## API Overview

### Creating a Stream

```go
s := stream.New(44100) // sample rate in Hz
defer s.Close()        // frees underlying C memory
```

### Loading Instruments

```go
s.Voices().LoadBank("spice", patches.Spice())

// Or define your own:
inst := &voice.Instrument{
    Name: "my_bass",
    Op1:  voice.Operator{Attack: 0, Decay: 8, Sustain: 10, Release: 4, Level: 20, Multiply: 1},
    Op2:  voice.Operator{Attack: 0, Decay: 6, Sustain: 12, Release: 8, Level: 5, Multiply: 2},
    Feedback:   4,
    Connection: 0, // 0 = FM, 1 = Additive
}
```

### Building Patterns

```go
pattern := sequencer.NewPattern(16).    // 16 steps
    Instrument("desert_bass").          // default instrument for this pattern
    Note(0, "C2").                      // step 0: C in octave 2
    Note(4, "Eb2").                     // step 4: E-flat in octave 2
    Hit(8)                              // step 8: percussive hit (C2)

s.Sequencer().SetPattern(0, pattern)    // assign to channel 0
s.Sequencer().SetBPM(120)
```

### Reading Audio

```go
buf := make([]byte, 4096)
n, err := s.Read(buf) // fills buf with S16LE stereo PCM
```

## Contributing

Contributions are welcome. Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Licensing

This project uses a dual-license structure:

- **Go library code**: [MIT License](LICENSE)
- **Nuked-OPL3 C source** (vendored in `chip/opl3/`): [LGPL-2.1-or-later](chip/opl3/COPYING)

For full third-party attribution, see [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES).

### Static Linking Notice

The Nuked-OPL3 C source is compiled directly into this library via CGo (static linking). Since SpiceSynth is distributed as source code, users who build from source can freely modify and recompile the vendored C files in `chip/opl3/`. If you distribute pre-compiled binaries that incorporate this library, you must comply with LGPL-2.1 Section 6 by providing the LGPL source code and a mechanism for relinking.
