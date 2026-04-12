# SpiceSynth

SpiceSynth is a Go library for programmatic OPL2/OPL3 FM synthesis, producing authentic AdLib-era game music in real-time. It provides a high-level API to define musical patterns and streams signed 16-bit stereo little-endian PCM audio via an `io.Reader`.

## Features

- **Authentic Sound**: Based on the Nuked-OPL3 emulator, targeting the gritty FM crunch of early 90s DOS games.
- **Fluent Sequencer**: Define patterns and melodies using a builder-style API.
- **CGo Integration**: High performance via vendored C source, requiring only a C compiler to build.
- **Easy Integration**: Implements `io.Reader`, making it compatible with audio backends like Ebiten or Oto.

## Installation & Prerequisites

Since SpiceSynth uses CGo to interface with the Nuked-OPL3 emulator, you must have a C compiler installed and `CGO_ENABLED=1` set in your environment.

### Environment Setup
- **Linux**: Install `gcc` (e.g., via `sudo apt install build-essential`).
- **macOS**: Install Xcode Command Line Tools (`xcode-select --install`).
- **Windows**: Install [MinGW-w64](https://www.mingw-w64.org/) or TDM-GCC. Ensure the `bin` folder is in your system PATH.

---

## 🚀 Integration Guide (Embedding in Your App)

To use SpiceSynth as a library in your own Go project:

### 1. Install the package
```bash
go get github.com/jebbisson/spice-synth
```

### 2. Basic Implementation
Initialize the synth stream, load an instrument bank, and define patterns using the fluent API.

```go
import (
    "github.com/jebbisson/spice-synth/stream"
    "github.com/jebbisson/spice-synth/patches"
)

func main() {
    // 1. Initialize stream at 44.1kHz
    s := stream.New(44100)
    
    // 2. Load Spice style instruments
    s.Voices().LoadBank("spice", patches.Spice())
    
    // 3. Define a simple pattern using the fluent API
    bass := s.Sequencer().NewPattern(16).
        Instrument("desert_bass").
        Note(0, "C2").
        Note(4, "Eb2")
        
    s.Sequencer().SetPattern(0, bass)
    s.Sequencer().SetBPM(120)

    // 4. Use 's' as an io.Reader with your audio backend (e.g., Ebiten, Oto)
}
```

---

## 🎧 Testing Guide (Running Standalone Examples)

If you want to test the synthesis engine without writing code, use the provided examples.

### Option A: Live Playback (Ebiten Player)
This requires a windowing system and audio drivers.
1. Navigate to the player directory: `cd examples/ebiten_player`
2. Run the application: `go run main.go`

### Option B: File Rendering (CLI Raw Demo)
Ideal for environments without audio hardware or for CI verification.
1. Run the demo: `go run examples/demo/main.go`
2. Play the resulting file: `ffplay -f s16le -ar 44100 -ac 2 output.raw`

---

## Architecture

SpiceSynth follows a four-layer stack to separate hardware emulation from musical composition:

1. **Chip**: CGo wrapper around Nuked-OPL3 that handles raw register writes and sample generation.
2. **Voice**: Translates high-level notes and instrument definitions into the specific register values required by OPL2/3.
3. **Sequencer**: A tick-based timing engine that triggers musical events (NoteOn, NoteOff) based on BPM.
4. **Stream**: The top-level `io.Reader` implementation. It drives the sequencer and chip in sync with the number of audio samples requested by the consumer.

## Licensing

- **Go Wrapper**: MIT License.
- **Nuked-OPL3 C Source**: LGPL-2.1 (vendored).
