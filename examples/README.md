# Examples

This directory contains examples of how to use SpiceSynth in different contexts.

## Ebiten Live Player

The `ebiten_player` is a standalone application that demonstrates real-time audio synthesis using the Ebiten game engine. It plays a pre-arranged musical piece in the "Spice" style and displays current status on screen.

### How to Run

Since this example uses Ebiten, it requires a windowing system (X11/Wayland on Linux, Cocoa on macOS, or Win32 on Windows) and a C compiler for audio drivers.

1. **Navigate to the player directory**:
   ```bash
   cd examples/ebiten_player
   ```

2. **Run the application**:
   ```bash
   go run main.go
   ```

### What it demonstrates:
- **Real-time streaming**: Direct integration of `stream.Stream` into Ebiten's audio context.
- **Complex Arrangement**: A multi-channel pattern (Bass, Lead, and Percussion) playing in sync.
- **Low Latency**: Configured buffer size for responsive playback.

## CLI Raw Demo

A simpler demo that renders audio to a file instead of playing it live. This is useful for CI/CD or environments without audio hardware.

1. **Run the demo**:
   ```bash
   go run examples/demo/main.go
   ```

2. **Play the output**:
   ```bash
   ffplay -f s16le -ar 44100 -ac 2 output.raw
   ```
