# Examples

This directory contains examples of how to use SpiceSynth in different contexts.

## ADL Player (Dune II)

The `adl_player` plays Westwood Studios ADL music files from Dune II through OPL2 FM synthesis. It supports keyboard navigation between subsongs (with type labels: MUSIC/SFX/RESET) and switching between ADL files.

```bash
cd examples/adl_player
go run .                    # plays DUNE1.ADL, auto-selects first music track
go run . ../adl/DUNE9.ADL   # plays a specific file
go run . ../adl/DUNE1.ADL 5 # plays a specific subsong
```

**Controls:** Left/Right = prev/next subsong, Up/Down = prev/next file, Space = pause, R = restart, Q = quit.

The `adl/` subdirectory contains 21 Dune II ADL files (DUNE0.ADL through DUNE20.ADL) used by this example.

## MIDI Player

The `midi_player` plays Standard MIDI files through OPL2 FM synthesis using the embedded DMXOPL General MIDI bank. It displays real-time volume and playback status.

```bash
cd examples/midi_player
go run . ../midi/Title.mid
```

**Controls:** Space = pause, Q = quit.

The `midi/` subdirectory contains a sample MIDI file used by this example.

## Ebiten Live Player (DSL)

The `ebiten_player` demonstrates the Strudel-inspired DSL API with a multi-channel arrangement (bass, wind, and chime layers) playing in real-time via Ebiten.

```bash
cd examples/ebiten_player
go run .
```

## CLI Raw Demo

Renders a multi-channel arrangement (bass, lead, and percussion) to a raw PCM file. Useful for CI/CD or environments without audio hardware.

```bash
go run examples/demo/main.go
ffplay -f s16le -ar 44100 -ac 2 output.raw
```

## Basic

Renders a single DSL-defined note to a raw PCM file -- the simplest possible SpiceSynth usage.

```bash
go run examples/basic/main.go
ffplay -f s16le -ar 44100 -ac 2 output.raw
```
