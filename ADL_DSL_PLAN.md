# ADL to DSL Conversion Plan

## Goal

Build a separate tool that converts `.ADL` files into Go code using the `dsl` package, while expanding the DSL and its runtime so that all currently supported musical behavior in the `adl` implementation can be represented in a Strudel-inspired way consistent with `DSL_DESIGN.md`.

## Principles

- [ ] Keep the public DSL aligned with `DSL_DESIGN.md`
- [ ] Prefer musical and compositional DSL concepts over ADL opcode-shaped public APIs
- [ ] Allow lower-level internal runtime/compiler constructs where needed to preserve ADL behavior
- [ ] Keep the converter separate from examples and core library usage
- [ ] Ensure generated code produces playable whole songs, not just fragments
- [ ] Validate against real ADL files in `examples/adl`

## Current State Summary

### ADL currently supports

- [x] 72 Hz bytecode-driven playback
- [x] multi-channel orchestration with a control channel
- [x] per-channel timing and tempo behavior
- [x] note/rest playback and duration handling
- [x] instrument loading from raw ADL patch data
- [x] dynamic timbre and level changes during playback
- [x] slide, vibrato, pitch bend, note randomness
- [x] beat-based waits and control flow
- [x] rhythm section opcodes
- [x] whole-song playback through `adl.Player`

### DSL currently supports

- [x] single-pattern note-oriented builder flow
- [x] static patch selection and patch overrides
- [x] partial ADSR/FM support
- [x] limited signal modulation
- [x] basic sequencer note on/off events
- [x] direct playback and simple sequenced playback

### Main gap

- [ ] DSL runtime is not yet expressive enough to represent full ADL song behavior
- [ ] DSL playback path is still prototype-level compared to the ADL player

## Desired End State

- [ ] `dsl` can represent all currently supported ADL musical behaviors needed for song playback
- [ ] public DSL remains Strudel-inspired
- [ ] internal DSL/runtime can express lower-level timing/control requirements
- [ ] a separate `adl` conversion tool emits Go source using the DSL
- [ ] generated output can play complete songs from the included ADL examples

## Scope Boundaries

### In scope

- [ ] melodic ADL song conversion
- [ ] DSL/runtime expansion needed for supported ADL playback features
- [ ] generated instrument banks from ADL patch data
- [ ] generated multi-track song structures
- [ ] validation against included ADL files

### Out of scope for first pass unless required by target songs

- [ ] exact emulation of every obscure/unknown ADL callback behavior
- [ ] perfect reproduction of queue/retry semantics for SFX-focused cases
- [ ] broad OPL3-specific expansion unrelated to ADL conversion
- [ ] converter support for unsupported ADL variants beyond what the current parser/player handles

## Architecture Direction

### Public layer

- [ ] extend `dsl` with song-level composition primitives
- [ ] keep API musical and compositional
- [ ] align new surface area with `DSL_DESIGN.md` tiers, especially Tiers 8-10

### Internal layer

- [ ] add an internal event/timeline/control representation powerful enough for ADL lowering
- [ ] allow generated code to target stable DSL/runtime constructs
- [ ] avoid exposing raw ADL opcodes as the main user-facing API

### Tooling layer

- [ ] add a separate converter tool, likely `cmd/adl2dsl`
- [ ] input: `.ADL` path and subsong selection
- [ ] output: Go source that constructs a DSL song and embedded/generated instruments

## Work Plan

## Phase 0: Planning and Baseline

- [x] create this tracking document in the repo
- [x] identify first benchmark ADL songs/subsongs for development

### Suggested benchmark set

- [ ] `examples/adl/DUNE1.ADL` subsong 2
- [ ] one additional song with different arrangement complexity
- [ ] one song using features that stress timing/modulation

## Phase 1: Fix and Stabilize Current DSL Playback

### DSL correctness fixes

- [ ] fix sequenced DSL instrument registration so `dsl.Play()` can resolve instruments reliably
- [ ] verify cloned/overridden instruments are actually available to the sequencer
- [ ] verify `velocity` is either implemented or removed until supported
- [ ] add tests covering sequenced DSL playback with instrument overrides

### Runtime confidence

- [ ] verify `dsl` can produce audible output through `stream.Stream`
- [ ] add tests for note on/off and sustained note behavior
- [ ] add tests for signal-driven modulation in sequenced playback

## Phase 2: Add Song-Level DSL Runtime

### Core song model

- [ ] introduce a `dsl.Song` type
- [ ] introduce a `dsl.Track` type for per-channel event sequences
- [ ] support multiple tracks/layers in one song
- [ ] support deterministic scheduling of multiple patterns
- [ ] support explicit channel allocation
- [ ] support generated instrument registration for a song

### Pattern/timeline expansion

- [ ] support note durations and explicit note off behavior
- [ ] support rests and silence
- [ ] support multi-event phrases instead of single-note patterns
- [ ] support reusable track fragments that can be layered/concatenated

### DSL_DESIGN.md alignment

- [ ] implement `Stack()` for simultaneous layers (Tier 8)
- [ ] implement `Seq()` for sequential composition (Tier 8)
- [ ] implement runtime support for `.Slow()` / `.Fast()` time modifiers (Tier 9)

## Phase 3: Add Internal Lowering Model For ADL Semantics

### Timing/control representation

- [ ] define an internal tick-based event model for timed note, patch, level, and control changes
- [ ] define how ADL 72 Hz timing maps into DSL/runtime timing
- [ ] support per-track timing behavior beyond one global BPM assumption
- [ ] define lowering rules from ADL control flow into finite/looping event timelines

### Required ADL-derived capabilities

- [ ] control-track orchestration lowered into `Stack` of tracks
- [ ] dynamic patch or level state changes over time
- [ ] pitch movement events (slide, vibrato, pitch bend)
- [ ] channel level/timbre automation events

## Phase 4: Feature Mapping From ADL To DSL

### Must-have mappings

- [ ] note events with frequency
- [ ] rests
- [ ] note duration and release behavior
- [ ] instrument setup from ADL instruments
- [ ] base note / base octave / transposition
- [ ] pitch bend
- [ ] slide effect
- [ ] vibrato effect
- [ ] dynamic level changes (extra levels)
- [ ] feedback/connection/waveform settings
- [ ] global tempo and channel tempo handling
- [ ] repeat/jump/subroutine lowering into generated structure
- [ ] control-channel-driven multi-track setup

### Evaluate for first pass

- [ ] fractional note spacing
- [ ] duration randomness
- [ ] note randomness
- [ ] AM depth / vibrato depth toggles
- [ ] secondary effect register-sequence behavior
- [ ] rhythm section support

## Phase 5: Implement Converter Tool

### Tool shape

- [ ] add `cmd/adl2dsl/main.go`
- [ ] support CLI args for input path
- [ ] support subsong selection
- [ ] support package name/output path selection

### Converter responsibilities

- [ ] parse ADL file using existing `adl` package
- [ ] extract instruments and convert them into Go definitions
- [ ] analyze the selected subsong/program graph
- [ ] lower ADL control flow into song/track structures
- [ ] emit readable Go code using DSL builders

### Output design

- [ ] generated code constructs a `dsl.Song`
- [ ] generated code registers instruments
- [ ] generated code exposes clear entry points
- [ ] generated code is readable and deterministic

## Phase 6: Validation and Fidelity Checks

### Unit/integration coverage

- [ ] add tests for new DSL song/runtime pieces
- [ ] add tests for converter output generation
- [ ] add tests that generated code compiles
- [ ] add tests that generated songs produce non-zero audio

### Behavioral validation

- [ ] compare generated-song playback against `adl.Player` on benchmark songs
- [ ] validate extracted instruments against ADL raw instrument interpretation

### Audio validation

- [ ] run end-to-end playback using the stream path
- [ ] run `go test ./...` to verify all packages pass
- [ ] run example playback for generated songs

## Deliverables

- [ ] `ADL_DSL_PLAN.md` tracking file
- [ ] stable `dsl` song/runtime support for full-song composition
- [ ] fixed sequenced DSL playback path
- [ ] `cmd/adl2dsl` converter tool
- [ ] generated output for benchmark ADL songs
- [ ] tests covering runtime and converter behavior

## Success Criteria

- [ ] at least one benchmark ADL song converts into readable Go DSL code
- [ ] generated code plays a whole song through the standard stream/DSL path
- [ ] generated playback is recognizably faithful to the original ADL playback
- [ ] the public DSL remains consistent with the Strudel-inspired direction in `DSL_DESIGN.md`
- [ ] the converter is separate from examples and core runtime use
- [ ] `go test ./...` passes
