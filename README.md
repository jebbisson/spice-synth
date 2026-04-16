# SpiceSynth

[![Go Reference](https://pkg.go.dev/badge/github.com/jebbisson/spice-synth.svg)](https://pkg.go.dev/github.com/jebbisson/spice-synth)
[![CI](https://github.com/jebbisson/spice-synth/actions/workflows/ci.yml/badge.svg)](https://github.com/jebbisson/spice-synth/actions/workflows/ci.yml)

SpiceSynth is an LGPL-2.1-or-later Go library for programmatic OPL2/OPL3 FM synthesis. It produces authentic AdLib-era game music in real-time and streams signed 16-bit stereo PCM audio via a standard `io.Reader` interface.

Inspired by the grungy, aggressive FM sound of Dune II on PC -- the crunchy bass lines, metallic leads, and raw OPL2 character of Westwood Studios' AdLib driver. SpiceSynth can play back the original Dune II ADL music files directly, render General MIDI through OPL2, or compose new FM music from scratch using a fluent DSL.

## Features

- **Authentic FM Sound** -- Cycle-accurate emulation via the [Nuked-OPL3](https://github.com/nukeykt/Nuked-OPL3) engine.
- **Dune II ADL Playback** -- Full bytecode VM player for Westwood Studios' ADL music format, with subsong classification and instrument extraction.
- **General MIDI via OPL2** -- MIDI file parser and multi-chip player with an embedded DMXOPL General MIDI bank (OP2 format).
- **Fluent DSL** -- Compose FM patterns with a [Strudel](https://strudel.cc)-inspired method-chaining API, including LFO, envelope, and ramp modulators.
- **`io.Reader` Output** -- Drop-in compatible with audio backends like [Ebiten](https://ebitengine.org/) and [Oto](https://github.com/ebitengine/oto).
- **High-Level Shared Library Packaging** -- The project can be built as a top-level shared library or linkable archive for downstream products.

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

## Distribution Modes

SpiceSynth supports two usage patterns:

1. **Direct Go module use**
- import `github.com/jebbisson/spice-synth` into another Go project
- best for development and internal use

2. **Packaged library distribution**
- build `spice-synth` itself as a shared library or linkable archive
- ship that artifact with your product
- recommended when you want a straightforward LGPL distribution story

### Shared Library Build

Build a shared library from the top-level wrapper target:

```bash
go build -buildmode=c-shared -o libspicesynth.so ./cmd/spicesynthshared
```

Common outputs by platform:

- Linux: `libspicesynth.so`
- macOS: `libspicesynth.dylib`
- Windows: `spicesynth.dll`

See `cmd/spicesynthshared/README.md` for the exported ABI and packaging notes.

### Linkable Archive Build

Build a linkable archive instead:

```bash
go build -buildmode=c-archive -o libspicesynth.a ./cmd/spicesynthshared
```

This produces a compiled archive plus a generated C header. Use this mode if
your product wants to link SpiceSynth into another binary while still keeping a
relinkable deliverable.

### Packaging Expectations

For packaged commercial distribution, the intended setup is:

1. build `spice-synth` as `c-shared` or `c-archive`
2. bundle the generated library artifact with your product
3. include the project license and third-party notices
4. document or preserve a relinkable path for the library artifact

The source repository does not ship prebuilt binaries by default. Your product
build should create the platform-specific artifact it needs.

## Using SpiceSynth From Another Project

### Direct Go module use

If you just want to import the library in another Go project:

```bash
go get github.com/jebbisson/spice-synth
```

Then use it as a normal Go dependency.

### Shared library / archive use

If you want to package SpiceSynth as a runtime library for another project:

1. build `cmd/spicesynthshared` as `c-shared` or `c-archive`
2. include the produced library artifact in your product build or installer
3. compile or load against the generated C header
4. include the SpiceSynth `LICENSE` and `THIRD_PARTY_LICENSES`

This is the recommended packaging path for distributed commercial products that
want a straightforward LGPL compliance story.

## Quick Start

### Play a Dune II ADL file

```go
package main

import (
    "os"

    "github.com/jebbisson/spice-synth/adl"
)

func main() {
    f, _ := os.Open("DUNE1.ADL")
    defer f.Close()

    af, _ := adl.Parse(f)
    p := adl.NewPlayer(44100, af)

    // Auto-select the first music track.
    for _, info := range af.NonEmptySubsongs() {
        if info.Type == adl.SubsongMusic {
            p.SetSubsong(info.Index)
            break
        }
    }
    p.Play()

    // 'p' implements io.Reader -- pass it to your audio backend.
}
```

### Compose with the DSL

```go
package main

import (
    "github.com/jebbisson/spice-synth/patches"
    "github.com/jebbisson/spice-synth/sequencer"
    "github.com/jebbisson/spice-synth/stream"
)

func main() {
    s := stream.New(44100)
    defer s.Close()

    s.Voices().LoadBank("spice", patches.Spice())

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

```
┌─────────────────────────────────────────────────────┐
│  stream       io.Reader PCM output                  │
├─────────────────────────────────────────────────────┤
│  dsl          Fluent method-chaining composition    │
│  sequencer    Tick-based pattern engine             │
│  player       MIDI → OPL2 multi-chip renderer       │
│  adl          Dune II ADL bytecode VM player        │
├─────────────────────────────────────────────────────┤
│  voice        Notes & instruments → OPL registers   │
├─────────────────────────────────────────────────────┤
│  chip         CGo wrapper for Nuked-OPL3            │
└─────────────────────────────────────────────────────┘
```

| Package | Description |
|---------|-------------|
| [`chip`](chip/) | CGo wrapper around Nuked-OPL3. Handles register writes and sample generation. |
| [`voice`](voice/) | Translates notes and instrument definitions into OPL2 register values. Manages melodic channels. |
| [`sequencer`](sequencer/) | Tick-based timing engine. Triggers NoteOn/NoteOff events from looping patterns. |
| [`stream`](stream/) | Top-level `io.Reader`. Drives the sequencer and chip in sync with audio buffer requests. |
| [`dsl`](dsl/) | Fluent, Strudel-inspired API for composing FM patterns with modulation (LFO, envelope, ramp). |
| [`player`](player/) | MIDI file player. Manages multiple OPL3 chip instances for unlimited polyphony. |
| [`midi`](midi/) | Standard MIDI File (SMF) parser. Supports Format 0 and Format 1 files. |
| [`op2`](op2/) | OP2 bank parser (DMX GENMIDI format). Embeds a high-quality DMXOPL General MIDI bank. |
| [`adl`](adl/) | Westwood Studios ADL format parser and 72Hz bytecode VM player. Targets Dune II v2/v3. |
| [`patches`](patches/) | Predefined FM instrument banks (SpiceSynth originals, GM via OP2). |

## Examples

Working examples are in the [`examples/`](examples/) directory. See the [examples README](examples/README.md) for details.

| Example | Description | Audio Backend |
|---------|-------------|---------------|
| [`adl_player`](examples/adl_player/) | Play Dune II ADL files with subsong browsing | Ebiten |
| [`midi_player`](examples/midi_player/) | Play MIDI files through OPL2 FM synthesis | Ebiten |
| [`ebiten_player`](examples/ebiten_player/) | DSL-composed arrangement with real-time playback | Ebiten |
| [`demo`](examples/demo/) | Render a multi-channel arrangement to a raw PCM file | Headless |
| [`basic`](examples/basic/) | Render a single note to a raw PCM file | Headless |

### ADL Player (Dune II)

```bash
cd examples/adl_player
go run .                    # plays DUNE1.ADL, auto-selects first music track
go run . ../adl/DUNE9.ADL   # plays a specific file
```

Controls: Left/Right = prev/next subsong, Up/Down = prev/next file, Space = pause, Q = quit.

### MIDI Player

```bash
cd examples/midi_player
go run . ../midi/Title.mid
```

### DSL Live Playback

```bash
cd examples/ebiten_player
go run .
```

### File Rendering (CLI)

Renders audio to a raw PCM file -- useful for headless environments or CI.

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

### Playing ADL Files

```go
af, _ := adl.ParseBytes(data)
p := adl.NewPlayer(44100, af)
p.SetSubsong(2)
p.Play()

buf := make([]byte, 4096)
p.Read(buf) // signed 16-bit stereo LE PCM
```

### Playing MIDI Files

```go
mf, _ := midi.ParseBytes(data)
bank := op2.DefaultBank()
p := player.New(44100, mf, bank)
p.Play()

buf := make([]byte, 4096)
p.Read(buf)
```

### Reading Audio

```go
buf := make([]byte, 4096)
n, err := s.Read(buf) // fills buf with S16LE stereo PCM
```

## Contributing

Contributions are welcome. Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Licensing

SpiceSynth is distributed under **LGPL-2.1-or-later**.

This repository builds on top of:

- [`spice-opl3-nuked`](https://github.com/jebbisson/spice-opl3-nuked)
- [`spice-adl-adplug`](https://github.com/jebbisson/spice-adl-adplug)

For full attribution and dependency notes, see [THIRD_PARTY_LICENSES](THIRD_PARTY_LICENSES).

### Static Linking Notice

The Nuked-OPL3 C source is compiled directly into this library via CGo (static linking). Since SpiceSynth is distributed as source code, users who build from source can freely modify and recompile the vendored C files in `chip/opl3/`. If you distribute pre-compiled binaries that incorporate this library, you must comply with LGPL-2.1 Section 6 by providing the LGPL source code and a mechanism for relinking.
