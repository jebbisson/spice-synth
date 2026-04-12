# SpiceSynth DSL Design — Strudel-Inspired API for OPL2/OPL3

> API inspired by [Strudel](https://strudel.cc) (AGPL-3.0), reimplemented from
> scratch in Go targeting the Nuked-OPL3 FM synthesis chip.

## Overview

The `dsl` package provides a fluent, method-chaining API for composing FM
synthesis patterns. Unlike Strudel (which targets Web Audio), SpiceSynth
targets real OPL2/OPL3 register writes, so every parameter maps to actual
hardware behaviour with the constraints and character that entails.

**Package:** `spice-synth/dsl`  
**Style:** Fluent method chaining (`Note("c2").S("bass").FM(6).Gain(...)`)  
**Target:** OPL2 (9 channels, 4 waveforms, mono) as primary, OPL3 as opt-in  
**License:** MIT (SpiceSynth), inspired by Strudel (AGPL-3.0)

---

## Table of Contents

1. [Tier 1 — Core Sound Generation](#tier-1--core-sound-generation)
2. [Tier 2 — Amplitude Envelope (ADSR)](#tier-2--amplitude-envelope-adsr)
3. [Tier 3 — FM Synthesis Parameters](#tier-3--fm-synthesis-parameters)
4. [Tier 4 — Continuous Signals & LFOs](#tier-4--continuous-signals--lfos)
5. [Tier 5 — Amplitude Modulation (Tremolo)](#tier-5--amplitude-modulation-tremolo)
6. [Tier 6 — Vibrato & Pitch Effects](#tier-6--vibrato--pitch-effects)
7. [Tier 7 — Dynamics & Gain](#tier-7--dynamics--gain)
8. [Tier 8 — Pattern Construction](#tier-8--pattern-construction)
9. [Tier 9 — Time Modifiers](#tier-9--time-modifiers)
10. [Tier 10 — Pattern Combinators](#tier-10--pattern-combinators)
11. [Tier 11 — Waveshaping & Bit Reduction](#tier-11--waveshaping--bit-reduction)
12. [Tier 12 — OPL3 Extensions (Stereo, 4-Op, Extra Waveforms)](#tier-12--opl3-extensions-stereo-4-op-extra-waveforms)
13. [Tier 13 — Filter Approximations](#tier-13--filter-approximations)
14. [Tier 14 — Global Effects (Delay, Reverb)](#tier-14--global-effects-delay-reverb)
15. [Tier 15 — Advanced Pattern Functions](#tier-15--advanced-pattern-functions)
16. [Implementation Plan](#implementation-plan)
17. [OPL2 Register Mapping Reference](#opl2-register-mapping-reference)

---

## Tier 1 — Core Sound Generation

These are the absolute basics: making a sound at a pitch.

### `Note(noteStr)` / `.Note(noteStr)`

Sets the pitch of the pattern.

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `note("c2")` | `Note("c2")` | F-Number + Block via registers `0xA0` + `0xB0`. 10-bit F-Number and 3-bit block encode the frequency. |

**Supported note formats:** `"C2"`, `"Eb4"`, `"F#3"`, MIDI numbers (`48`), Hz values (`65.41`).

**Constraints:** OPL2's frequency resolution is limited by the 10-bit F-Number. Very low notes (below ~32 Hz) may have audible stepping between semitones. The valid range is roughly C0 to B7.

```go
// Go usage
p := dsl.Note("c2")
```

### `S(name)` / `.S(name)` — Sound / Instrument Select

Selects the instrument (patch) to use. In Strudel this selects a sample or
waveform type. In SpiceSynth this selects a named instrument from the loaded
bank, which defines the full 2-operator FM patch.

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `s("sine")` | `S("desert_bass")` | Writes all operator registers: `0x20`, `0x40`, `0x60`, `0x80`, `0xE0` for both operators, plus `0xC0` (feedback/connection). |

**Strudel waveform names mapped to OPL2 waveforms:**

| Strudel name | OPL2 Waveform | Register 0xE0 value | Notes |
|-------------|---------------|---------------------|-------|
| `"sine"` | Sine | 0 | Full sine wave |
| `"halfsine"` | Half-sine | 1 | Positive half only |
| `"abssine"` | Abs-sine | 2 | Full-wave rectified sine |
| `"quartersine"` | Quarter-sine | 3 | Positive quarter, then silence |

When `S()` is given a raw waveform name (above), SpiceSynth creates a minimal
instrument with that waveform on the carrier, default ADSR, and no FM. When
given a named patch (e.g. `"desert_bass"`), it loads the full instrument
definition.

```go
// Named instrument
p := dsl.Note("c2").S("desert_bass")

// Raw waveform (creates minimal carrier-only patch)
p := dsl.Note("c4").S("sine")
```

### `N(step)` / `.N(step)` — Pattern Step Index

Selects a note from a scale by index (0-based). Requires `.Scale()` to be set.

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `n("0 2 4 7")` | `N("0 2 4 7")` | Converted to frequency via scale lookup, then to F-Number + Block. |

```go
p := dsl.N("0 2 4 7").Scale("C4:minor").S("desert_bass")
```

---

## Tier 2 — Amplitude Envelope (ADSR)

OPL2 has hardware ADSR envelopes on every operator. These map almost directly
to Strudel's envelope parameters, but with important differences in resolution
and curve shape.

### OPL2 Hardware ADSR

Each operator has its own 4-bit ADSR (values 0-15) stored in registers `0x60`
(Attack/Decay) and `0x80` (Sustain/Release). The envelope is exponential, not
linear.

| Strudel | SpiceSynth DSL | OPL2 Register | Range | Notes |
|---------|----------------|---------------|-------|-------|
| `.attack(sec)` | `.Attack(sec)` | `0x60` bits 7-4 | 0-15 (4-bit) | Time is quantized to 16 rates. ~0ms (15) to ~10s (1). 0 = no attack. SpiceSynth converts seconds to the nearest OPL2 rate value. |
| `.decay(sec)` | `.Decay(sec)` | `0x60` bits 3-0 | 0-15 (4-bit) | Time from peak to sustain level. Same rate table as attack. |
| `.sustain(level)` | `.Sustain(level)` | `0x80` bits 7-4 | 0-15 (4-bit) | **Inverted from Strudel:** OPL2 0=loudest, 15=silent (~45 dB attenuation). SpiceSynth maps Strudel's 0.0-1.0 range to OPL2's inverted scale. |
| `.release(sec)` | `.Release(sec)` | `0x80` bits 3-0 | 0-15 (4-bit) | Time from sustain to silence after key-off. |

### `.ADSR(a, d, s, r)` — Shorthand

| Strudel | SpiceSynth DSL | Notes |
|---------|----------------|-------|
| `.adsr(".1:.1:.5:.2")` | `.ADSR(0.1, 0.1, 0.5, 0.2)` | Sets all four envelope parameters at once. |

**Constraints:**
- OPL2 envelopes are **exponential**, not linear. The attack curve is concave,
  decay/release are convex. This cannot be changed.
- Only 16 rate values. Fine-grained timing (e.g. 0.073s attack) will be
  quantized to the nearest available rate.
- OPL2 sustain is an attenuation level, not a volume level. The DSL handles
  the inversion transparently.
- Both operators share the same key-on/key-off timing. Independent operator
  envelopes are set via the instrument patch, not per-note.

### `.Sustaining(bool)` — Envelope Hold Mode

| Strudel | SpiceSynth DSL | OPL2 Register | Notes |
|---------|----------------|---------------|-------|
| N/A | `.Sustaining(true)` | `0x20` bit 5 | `true` = envelope holds at sustain level until key-off. `false` = envelope proceeds through sustain to release automatically (percussive mode). |

```go
// Sustained pad — holds until explicitly released
p := dsl.Note("c3").S("sine").Attack(0.5).Sustain(0.8).Sustaining(true)

// Percussive hit — decays automatically
p := dsl.Note("c4").S("sine").Attack(0.0).Decay(0.2).Sustain(0.0).Sustaining(false)
```

---

## Tier 3 — FM Synthesis Parameters

This is where SpiceSynth diverges most from Strudel. Strudel's FM is a
simplified abstraction over Web Audio. SpiceSynth exposes real 2-operator FM
with OPL2's specific parameter set.

### `.FM(depth)` — FM Modulation Depth

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.fm(4)` | `.FM(4)` | **Modulator Total Level** (register `0x40`, operator 1). Strudel's depth (0-inf) is mapped to OPL2's inverted 0-63 attenuation scale. Higher FM value = lower attenuation = more modulation. |

Mapping: `OPL2_level = max(0, 63 - (depth * 6.3))` (so FM=10 ~ level 0 = max modulation).

### `.FMH(ratio)` — FM Harmonicity Ratio

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.fmh(2)` | `.FMH(2)` | **Modulator Frequency Multiplier** (register `0x20` bits 3-0). OPL2 only supports integer multipliers: 0.5, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 12, 12, 15, 15. Non-integer ratios (e.g. 1.5) are rounded to the nearest available value. |

**Constraints:** Unlike Strudel's arbitrary harmonicity ratios, OPL2 is limited
to the 16 values in the frequency multiplier table. This means:
- `FMH(1.5)` rounds to MULT=2 (ratio 2.0) or MULT=1 (ratio 1.0)
- `FMH(1.618)` rounds to MULT=2 (ratio 2.0)
- Inharmonic FM (non-integer ratios) is only achievable through detuning tricks

### `.FMAttack(sec)`, `.FMDecay(sec)`, `.FMSustain(level)` — FM Envelope

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.fmattack(0.1)` | `.FMAttack(0.1)` | **Modulator Attack Rate** (register `0x60` bits 7-4 of Op1). |
| `.fmdecay(0.2)` | `.FMDecay(0.2)` | **Modulator Decay Rate** (register `0x60` bits 3-0 of Op1). |
| `.fmsustain(0.5)` | `.FMSustain(0.5)` | **Modulator Sustain Level** (register `0x80` bits 7-4 of Op1). |

These are separate from the carrier's ADSR. The modulator has its own
independent hardware envelope that shapes the FM modulation depth over time.
This is a genuinely powerful OPL2 feature — the timbre evolves independently
of the volume.

### `.Feedback(fb)` — Modulator Self-Feedback

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| N/A (no Strudel equiv) | `.Feedback(6)` | Register `0xC0` bits 3-1. Range 0-7. 0 = pure sine modulator, 7 = heavily distorted/noise-like modulator. |

This is a uniquely OPL2 parameter with no direct Strudel equivalent. It's the
primary source of "grit" and "crunch" in FM sounds. Higher feedback makes the
modulator increasingly noisy/distorted.

**Real-time modulatable:** Yes, smooth timbral morphing.

### `.Connection(mode)` — Synthesis Algorithm

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| N/A | `.Connection(0)` | Register `0xC0` bit 0. `0` = FM mode (Op1 modulates Op2). `1` = Additive mode (both operators output independently). |

### `.Waveform(carrier, modulator)` — Operator Waveforms

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| N/A | `.Waveform(0, 1)` | Register `0xE0` for each operator. First arg = carrier waveform (0-3), second = modulator waveform (0-3). |

OPL2 waveforms: `0`=sine, `1`=half-sine, `2`=abs-sine, `3`=quarter-sine.

```go
// Classic grungy bass
p := dsl.Note("c2").FM(6).FMH(1).Feedback(6).Attack(0.0).Sustaining(true)

// Bell / chime sound (inharmonic FM)
p := dsl.Note("c5").FM(3).FMH(4).Feedback(2).Decay(0.5).Sustain(0.0)

// Noisy wind texture
p := dsl.Note("c3").FM(8).FMH(7).Feedback(7).Sustaining(true)
```

---

## Tier 4 — Continuous Signals & LFOs

Strudel uses continuous signal functions (`sine`, `saw`, `tri`, `square`,
`rand`, `perlin`) as pattern values that are sampled at event times. SpiceSynth
implements these as software LFOs that write OPL2 registers between sub-blocks
(~689 Hz update rate at 44100 Hz sample rate).

### Signal Types

| Strudel Signal | SpiceSynth Signal | Implementation |
|---------------|-------------------|----------------|
| `sine` | `dsl.Sine()` | Software sine LFO, output 0.0-1.0 |
| `cosine` | `dsl.Cosine()` | Sine with pi/2 phase offset |
| `tri` | `dsl.Tri()` | Software triangle LFO |
| `saw` | `dsl.Saw()` | Software rising sawtooth LFO |
| `square` | `dsl.Square()` | Software square wave LFO |
| `rand` | `dsl.Rand()` | Per-tick random value 0.0-1.0 |
| `perlin` | `dsl.Perlin()` | Smoothed random (Perlin noise) |

### Signal Modifiers

| Strudel | SpiceSynth | Notes |
|---------|------------|-------|
| `sine.range(lo, hi)` | `Sine().Range(0.3, 1.0)` | Maps 0-1 output to `lo-hi` |
| `sine.slow(4)` | `Sine().Slow(4)` | One cycle takes 4 beats (at current BPM) |
| `sine.fast(2)` | `Sine().Fast(2)` | Two cycles per beat |
| `sine.segment(16)` | `Sine().Segment(16)` | Sample-and-hold: value only changes 16 times per cycle |

### Mapping Signals to OPL2 Parameters

Signals can be passed as arguments to any parameter method. The DSL
resolves which OPL2 register to modulate based on the method receiving the
signal:

| Usage | OPL2 Register Target | ModTarget |
|-------|---------------------|-----------|
| `.Gain(sine.Range(...))` | Carrier Total Level (`0x40`) | `ModCarrierLevel` |
| `.FM(sine.Range(...))` | Modulator Total Level (`0x40`) | `ModModulatorLevel` |
| `.Feedback(sine.Range(...))` | Feedback (`0xC0` bits 1-3) | `ModFeedback` |
| `.Note(sine.Range(...))` | Frequency (`0xA0`/`0xB0`) | `ModFrequency` |

```go
// Slow volume wobble (like the Hope Fades bass)
p := dsl.Note("c2").S("desert_bass").Gain(dsl.Sine().Range(0.3, 1.0).Slow(4))

// Timbral morphing via feedback modulation
p := dsl.Note("c2").S("desert_bass").Feedback(dsl.Tri().Range(2, 7).Slow(8))
```

**Constraints:**
- OPL2 registers have limited resolution (6-bit level, 3-bit feedback, 10-bit
  frequency). Fine modulation may show stepping artifacts.
- Update rate is ~689 Hz (every 64 samples). This is fast enough for LFOs up
  to ~50 Hz before aliasing becomes noticeable.
- Hardware tremolo/vibrato (register `0xBD`) are chip-global and fixed-rate
  (~3.7 Hz tremolo, ~6.1 Hz vibrato). Software LFOs provide per-channel,
  arbitrary-rate alternatives.

---

## Tier 5 — Amplitude Modulation (Tremolo)

Two approaches: hardware tremolo (chip-global, fixed rate) and software
tremolo (per-channel, arbitrary rate via signals).

### Hardware Tremolo

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| N/A | `.HWTremolo(true)` | Register `0x20` bit 7 (per-operator AM enable). Depth set globally via `0xBD` bit 7: shallow ~1 dB or deep ~4.8 dB. Rate is fixed at ~3.7 Hz. |
| N/A | `.TremoloDepth(deep)` | Register `0xBD` bit 7. Global — affects all channels with HW tremolo enabled. |

### Software Tremolo (via Signals)

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.tremolo(speed)` | `.Tremolo(speed)` | Software LFO on carrier total level (`0x40`). Arbitrary Hz rate, per-channel. |
| `.tremolodepth(d)` | `.TremoloDepth(d)` | Controls the amplitude of the software tremolo LFO. |
| `.tremoloshape(s)` | `.TremoloShape(s)` | Selects waveform: `"sine"`, `"tri"`, `"square"`, `"saw"`. |

```go
// Software tremolo at 2 Hz with moderate depth
p := dsl.Note("c3").S("desert_bass").Tremolo(2).TremoloDepth(0.5)

// Hardware tremolo (fixed ~3.7 Hz, chip-global)
p := dsl.Note("c3").S("desert_bass").HWTremolo(true)
```

---

## Tier 6 — Vibrato & Pitch Effects

### Hardware Vibrato

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| N/A | `.HWVibrato(true)` | Register `0x20` bit 6 (per-operator VIB enable). Rate fixed at ~6.1 Hz. Depth set globally via `0xBD` bit 6: ~7 cents or ~14 cents. |
| N/A | `.VibratoDepth(deep)` | Register `0xBD` bit 6. Global. |

### Software Vibrato (via Frequency Modulation)

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.vib(hz)` | `.Vib(hz)` | Software LFO on frequency registers (`0xA0`/`0xB0`). Arbitrary rate, per-channel. |
| `.vibmod(semitones)` | `.VibMod(semitones)` | Depth in semitones. Maps to the +/-12 semitone range of `ModFrequency`. |

### Pitch Envelope

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.penv(semitones)` | `.PEnv(semitones)` | Software envelope on frequency registers. One-shot ramp from `note +/- semitones` to `note`. |
| `.pattack(sec)` | `.PAttack(sec)` | Attack time of the pitch envelope ramp. |
| `.pdecay(sec)` | `.PDecay(sec)` | Decay time of the pitch envelope. |

**Constraints:** OPL2 frequency resolution is 10 bits. Very fine pitch
modulation (< 1 cent) may not be reproducible. Pitch bends that cross block
boundaries may have small discontinuities.

```go
// Software vibrato at 5 Hz, +/-0.5 semitone
p := dsl.Note("c4").S("mystic_lead").Vib(5).VibMod(0.5)

// Pitch drop effect
p := dsl.Note("c4").S("fm_perc").PEnv(12).PDecay(0.1)
```

---

## Tier 7 — Dynamics & Gain

### `.Gain(value)` — Output Level

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.gain(0.8)` | `.Gain(0.8)` | Carrier Total Level (register `0x40` Op2). 0.0 = silent (level 63), 1.0 = loudest (level 0). |

Accepts either a static value or a Signal for continuous modulation.

### `.Velocity(value)` — Per-Event Volume

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| `.velocity(0.5)` | `.Velocity(0.5)` | Multiplied with Gain before writing carrier level. |

### `.Ramp(from, to, sec)` — One-Shot Volume Ramp

| Strudel | SpiceSynth DSL | OPL2 Mapping |
|---------|----------------|--------------|
| N/A | `.Ramp(0.0, 1.0, 5.0)` | Software ramp on carrier total level. Multiplied with any concurrent LFO on the same target. |

```go
// Fade-in from silence over 5 seconds with wobble
p := dsl.Note("c2").S("desert_bass").
    Ramp(0.0, 1.0, 5.0).
    Gain(dsl.Sine().Range(0.3, 1.0).Slow(4))
```

---

## Tier 8 — Pattern Construction

These create the rhythmic structure — when notes happen.

### Mini-Notation Strings

SpiceSynth parses a subset of Strudel's mini-notation:

| Notation | Meaning | Example |
|----------|---------|---------|
| `"x y z"` | Sequence (events equally spaced in one cycle) | `"c4 e4 g4"` |
| `"x*n"` | Repeat n times | `"c4*4"` |
| `"[x y]"` | Sub-group (fit into one slot) | `"c4 [e4 g4]"` |
| `"<x y>"` | Alternating per cycle | `"<c4 e4>"` |
| `"~"` | Silence / rest | `"c4 ~ e4 ~"` |
| `"x!n"` | Replicate (same event n times) | `"c4!4"` |
| `"x@n"` | Stretch over n slots | `"c4@3 e4"` |
| `"x(n,m)"` | Euclidean rhythm | `"c4(3,8)"` |
| `"x,y"` | Stack (simultaneous) | `"c4,e4,g4"` |

```go
// Euclidean rhythm bass pattern
p := dsl.Note("c2(3,8)").S("desert_bass")

// Alternating notes
p := dsl.Note("<c4 e4 g4>").S("mystic_lead")
```

### `Seq(items...)` — Sequence

| Strudel | SpiceSynth DSL |
|---------|----------------|
| `seq("c4", "e4", "g4")` | `dsl.Seq(dsl.Note("c4"), dsl.Note("e4"), dsl.Note("g4"))` |

### `Cat(items...)` — Concatenate (one item per cycle)

| Strudel | SpiceSynth DSL |
|---------|----------------|
| `cat("c4", "e4")` | `dsl.Cat(dsl.Note("c4"), dsl.Note("e4"))` |

### `Stack(items...)` — Simultaneous Layers

| Strudel | SpiceSynth DSL |
|---------|----------------|
| `stack(bass, chimes)` | `dsl.Stack(bass, chimes)` |

Each item in a `Stack` is assigned to a separate OPL2 channel (max 9 in OPL2
mode, 18 in OPL3 mode).

### `Silence` — Rest

| Strudel | SpiceSynth DSL |
|---------|----------------|
| `silence` | `dsl.Silence()` |

---

## Tier 9 — Time Modifiers

These change the speed and timing of patterns.

| Strudel | SpiceSynth DSL | Notes |
|---------|----------------|-------|
| `.fast(n)` | `.Fast(n)` | Pattern plays n times faster. |
| `.slow(n)` | `.Slow(n)` | Pattern plays n times slower. |
| `.rev` | `.Rev()` | Reverse the pattern order. |
| `.early(n)` | `.Early(n)` | Shift pattern earlier by n cycles. |
| `.late(n)` | `.Late(n)` | Shift pattern later by n cycles. |

```go
// Fast arpeggiated pattern
p := dsl.Note("c4 e4 g4 c5").S("spice_chime").Fast(2)
```

---

## Tier 10 — Pattern Combinators

More advanced ways to combine and transform patterns.

| Strudel | SpiceSynth DSL | Notes |
|---------|----------------|-------|
| `.struct("x ~ x x")` | `.Struct("x ~ x x")` | Apply a rhythmic structure (boolean pattern) to the values. |
| `.euclid(n, k)` | `.Euclid(n, k)` | Euclidean rhythm — distribute n hits over k steps. |
| `.every(n, fn)` | `.Every(n, fn)` | Apply a transformation every n cycles. |
| `.sometimes(fn)` | `.Sometimes(fn)` | Apply a transformation randomly ~50% of the time. |
| `.jux(fn)` | N/A (requires stereo, see Tier 12) | Applies function to right channel only. Requires OPL3 stereo. |
| `.ply(n)` | `.Ply(n)` | Repeat each event n times. |
| `.iter(n)` | `.Iter(n)` | Shift the pattern by 1/n each cycle. |

### Tonal Functions

| Strudel | SpiceSynth DSL | Notes |
|---------|----------------|-------|
| `.scale("C:minor")` | `.Scale("C:minor")` | Map `N()` indices to a musical scale. Uses the same scale names as Strudel. |
| `.add(note(7))` | `.Add(7)` | Transpose by semitones. |
| `.sub(12)` | `.Sub(12)` | Transpose down by semitones. |

```go
// Euclidean bass with scale
p := dsl.N("0 2 4 7").Scale("C2:minor").S("desert_bass").Euclid(3, 8)

// Every 4th cycle, play an octave higher
p := dsl.Note("c3 e3 g3").S("mystic_lead").Every(4, func(p *dsl.Pattern) *dsl.Pattern {
    return p.Add(12)
})
```

---

## Tier 11 — Waveshaping & Bit Reduction

OPL2 has native waveform selection which provides a limited form of
waveshaping. Software-based effects can approximate some Strudel effects.

### `.Coarse(factor)` — Sample Rate Reduction

| Strudel | SpiceSynth DSL | Implementation |
|---------|----------------|----------------|
| `.coarse(4)` | `.Coarse(4)` | **Software:** Skip every nth sub-block tick, holding the last register values. Creates a lo-fi stepped modulation effect. |

### `.Crush(bits)` — Bit Crush

| Strudel | SpiceSynth DSL | Implementation |
|---------|----------------|----------------|
| `.crush(8)` | `.Crush(8)` | **Software post-processing:** Reduce bit depth of the PCM output stream. Applied in the `stream` layer after OPL2 sample generation. |

### `.Distort(amount)` — Distortion

| Strudel | SpiceSynth DSL | Implementation |
|---------|----------------|----------------|
| `.distort(3)` | `.Distort(3)` | **Dual approach:** For mild distortion, increase OPL2 feedback (register `0xC0`). For heavier distortion, apply software waveshaping to the PCM output. |

**OPL2-native distortion:** Feedback 0-7 provides increasing harmonic
distortion on the modulator. This is the authentic FM crunch. Values 5-7
produce genuinely noisy, gritty tones.

```go
// OPL2-native grit via high feedback
p := dsl.Note("c2").FM(6).Feedback(7)

// Software post-processing distortion
p := dsl.Note("c2").S("desert_bass").Distort(3)
```

---

## Tier 12 — OPL3 Extensions (Stereo, 4-Op, Extra Waveforms)

> **These features require OPL3 native mode** (`register 0x105 bit 0 = 1`).
> They are NOT available in OPL2 compatibility mode. Enable with
> `dsl.EnableOPL3()`.

### `.Pan(value)` — Stereo Panning

| Strudel | SpiceSynth DSL | OPL3 Mapping |
|---------|----------------|--------------|
| `.pan(0.5)` | `.Pan(0.5)` | Register `0xC0` bits 4-5. OPL3 only supports 3 positions: hard left (bit 4 only), center (both bits), hard right (bit 5 only). Values are quantized: 0.0-0.33 = left, 0.33-0.67 = center, 0.67-1.0 = right. |

**Constraints:** OPL3 panning is not continuous — only L, C, R. This is a
fundamental hardware limitation. Smooth panning is not possible.

### `.Jux(fn)` — Stereo Function Application

| Strudel | SpiceSynth DSL | OPL3 Mapping |
|---------|----------------|--------------|
| `.jux(rev)` | `.Jux(Rev)` | Duplicates the pattern onto two OPL3 channels, one panned left and one right, with the function applied to the right channel. Uses 2 channels per voice. |

### Extra Waveforms (OPL3)

| Waveform | Register `0xE0` value | Description |
|----------|----------------------|-------------|
| 4 | Alternating sine | Double-frequency sine, every other half muted |
| 5 | Abs alternating sine | Double-frequency absolute sine |
| 6 | Square | Pure square wave |
| 7 | Derived square | Exponential sawtooth-like |

### 4-Operator Mode

OPL3 can pair channels for 4-operator FM synthesis (register `0x104`),
enabling more complex timbres with 4 routing algorithms.

| Strudel | SpiceSynth DSL | OPL3 Mapping |
|---------|----------------|--------------|
| N/A | `.FourOp(true)` | Register `0x104`. Pairs channels (0+3, 1+4, 2+5, etc.). Reduces available channel count but enables richer timbres. |
| N/A | `.Algorithm(n)` | Sets the 4-op routing algorithm (0-3). Controls how the 4 operators are connected (serial FM, parallel additive, mixed). |

---

## Tier 13 — Filter Approximations

OPL2 has **no hardware filters** (no low-pass, high-pass, or band-pass).
However, some filter-like effects can be approximated:

### `.LPF(freq)` — Low-Pass Filter (Approximation)

| Strudel | SpiceSynth DSL | Implementation |
|---------|----------------|----------------|
| `.lpf(1000)` | `.LPF(1000)` | **Software post-processing** in the stream layer. A simple 1-pole or biquad filter applied to the PCM output after OPL2 generation. |

### Alternative: FM-Based Brightness Control

Instead of filtering, brightness is more idiomatically controlled via FM
parameters:

| Effect | How to Achieve |
|--------|---------------|
| "Darker" sound | Lower modulator level (`.FM(1)`), lower feedback (`.Feedback(1)`) |
| "Brighter" sound | Higher modulator level (`.FM(8)`), higher feedback (`.Feedback(6)`) |
| Brightness sweep | Modulate feedback or modulator level with an LFO |

```go
// "Filter sweep" via feedback modulation (authentic OPL2 approach)
p := dsl.Note("c2").S("desert_bass").
    Feedback(dsl.Saw().Range(1, 7).Slow(4))
```

### `.LPEnv(depth)` — Filter Envelope (Software)

| Strudel | SpiceSynth DSL | Implementation |
|---------|----------------|----------------|
| `.lpenv(4)` | `.LPEnv(4)` | Software: envelope controls a post-processing LPF cutoff. |

**Recommendation:** Prefer FM-based brightness control over software filters
for the most authentic OPL2 sound. Software filters should be marked as
"post-processing" in the API to set user expectations.

---

## Tier 14 — Global Effects (Delay, Reverb)

OPL2 has **no hardware delay or reverb**. These must be implemented entirely in
software post-processing on the PCM output stream.

### `.Delay(level)` / `.DelayTime(sec)` / `.DelayFeedback(fb)`

| Strudel | SpiceSynth DSL | Implementation |
|---------|----------------|----------------|
| `.delay(0.5)` | `.Delay(0.5)` | Software delay line in the stream layer. Ring buffer with feedback. |
| `.delaytime(0.25)` | `.DelayTime(0.25)` | Delay time in seconds. |
| `.delayfeedback(0.7)` | `.DelayFB(0.7)` | Feedback amount (0.0-0.99). |

### `.Room(level)` / `.RoomSize(size)`

| Strudel | SpiceSynth DSL | Implementation |
|---------|----------------|----------------|
| `.room(0.5)` | `.Room(0.5)` | Software reverb (simple Schroeder or FDN algorithm). |
| `.roomsize(4)` | `.RoomSize(4)` | Controls the reverb tail length. |

**Constraints:** Software effects add CPU overhead and latency. They're also
not "authentic" to the OPL2 sound — real DOS games sent OPL2 output directly
to speakers with no effects processing. Consider making these opt-in and
clearly labeled as post-processing.

---

## Tier 15 — Advanced Pattern Functions

These are more exotic Strudel features. Implementation priority is low but
they're documented for completeness.

| Strudel | SpiceSynth DSL | Feasibility |
|---------|----------------|-------------|
| `polymeter(a, b)` | `dsl.Polymeter(a, b)` | Pure pattern logic, no chip dependency. |
| `.mask("x ~ x ~")` | `.Mask("x ~ x ~")` | Pure pattern logic. |
| `.striate(n)` | N/A | Sample-based — not applicable to FM synthesis. |
| `.chop(n)` | N/A | Sample-based — not applicable. |
| `.loopAt(n)` | N/A | Sample-based — not applicable. |
| `.vowel("a e i")` | N/A | Formant filter — would require complex post-processing. Very low priority. |
| `mouseX`, `mouseY` | N/A | Browser-specific input — not applicable to a Go library. |

---

## Implementation Plan

### Phase 1: Foundation (Tiers 1-3)

Create the `dsl` package with the core pattern type and fluent API for sound
generation, ADSR, and FM parameters. This translates directly to existing
`voice.Manager` and `sequencer` calls.

1. Define `Pattern` struct and fluent builder methods
2. `Note()`, `S()`, `N()` — pitch and sound selection
3. `Attack()`, `Decay()`, `Sustain()`, `Release()`, `ADSR()` — hardware envelope
4. `FM()`, `FMH()`, `FMAttack()`, `FMDecay()`, `FMSustain()` — FM parameters
5. `Feedback()`, `Connection()`, `Waveform()` — OPL2-specific parameters
6. Wire the DSL pattern to voice.Manager + sequencer for playback

### Phase 2: Signals & Modulation (Tiers 4-6)

Implement the continuous signal system and map it to the existing modulator
infrastructure.

7. `Signal` type: `Sine()`, `Tri()`, `Saw()`, `Square()`, `Rand()`, `Perlin()`
8. Signal modifiers: `.Range()`, `.Slow()`, `.Fast()`, `.Segment()`
9. Signal-to-modulator compilation (Signal -> voice.LFO/Ramp/Envelope)
10. `Tremolo()`, `TremoloDepth()`, `TremoloShape()` — software amplitude mod
11. `HWTremolo()`, `HWVibrato()` — hardware chip features
12. `Vib()`, `VibMod()` — software vibrato
13. `PEnv()`, `PAttack()`, `PDecay()` — pitch envelope

### Phase 3: Dynamics & Gain (Tier 7)

14. `Gain()`, `Velocity()` — static and signal-driven volume
15. `Ramp()` — one-shot volume automation
16. Gain x Signal multiplication (existing modulator multiplication logic)

### Phase 4: Pattern Language (Tiers 8-10)

Implement the pattern construction and combinators.

17. Mini-notation parser: `"c4 e4 g4"`, `"c4*4"`, `"[x y]"`, `"<x y>"`, `"~"`
18. `Seq()`, `Cat()`, `Stack()`, `Silence()`
19. `.Fast()`, `.Slow()`, `.Rev()`, `.Early()`, `.Late()`
20. `.Struct()`, `.Euclid()`, `.Ply()`, `.Iter()`
21. `.Every()`, `.Sometimes()` — conditional modifiers
22. `.Scale()`, `.Add()`, `.Sub()` — tonal functions

### Phase 5: Post-Processing Effects (Tiers 11, 13-14)

Software effects applied to the PCM output stream.

23. `.Crush()` — bit depth reduction
24. `.Coarse()` — sample rate reduction
25. `.Distort()` — software waveshaping (complement to OPL2 feedback)
26. `.LPF()`, `.HPF()` — software filters
27. `.Delay()`, `.DelayTime()`, `.DelayFB()` — delay line
28. `.Room()`, `.RoomSize()` — reverb

### Phase 6: OPL3 Extensions (Tier 12)

29. `EnableOPL3()` — switch to native mode
30. `.Pan()` — stereo panning (L/C/R)
31. `.Jux()` — stereo function application
32. Extra waveforms (4-7)
33. `.FourOp()`, `.Algorithm()` — 4-operator mode

### Phase 7: Advanced Patterns (Tier 15)

34. `Polymeter()`, `.Mask()` — exotic pattern combinators
35. Mini-notation extensions: `(n,m)` Euclidean, `!n` replicate, `@n` elongate

---

## OPL2 Register Mapping Reference

Quick reference for how DSL parameters map to hardware.

### Per-Operator Registers (x 2 operators x 9 channels)

| Register | Bits | DSL Method | Notes |
|----------|------|------------|-------|
| `0x20+op` | 7: AM | `.HWTremolo()` | Per-operator enable |
| `0x20+op` | 6: VIB | `.HWVibrato()` | Per-operator enable |
| `0x20+op` | 5: EG Type | `.Sustaining()` | Hold or auto-release |
| `0x20+op` | 4: KSR | (instrument patch) | Key scale rate |
| `0x20+op` | 3-0: MULT | `.FMH()` | Frequency multiplier |
| `0x40+op` | 7-6: KSL | (instrument patch) | Key scale level |
| `0x40+op` | 5-0: TL | `.Gain()`, `.FM()`, signals | Total level (attenuation) |
| `0x60+op` | 7-4: AR | `.Attack()`, `.FMAttack()` | Attack rate |
| `0x60+op` | 3-0: DR | `.Decay()`, `.FMDecay()` | Decay rate |
| `0x80+op` | 7-4: SL | `.Sustain()`, `.FMSustain()` | Sustain level |
| `0x80+op` | 3-0: RR | `.Release()` | Release rate |
| `0xE0+op` | 1-0: WF | `.Waveform()`, `.S("sine")` | Waveform select (0-3 OPL2) |

### Per-Channel Registers (x 9 channels)

| Register | Bits | DSL Method | Notes |
|----------|------|------------|-------|
| `0xA0+ch` | 7-0: F-Num low | `.Note()`, `.Vib()` | Frequency low byte |
| `0xB0+ch` | 5: Key-On | (automatic) | Note trigger |
| `0xB0+ch` | 4-2: Block | `.Note()` | Octave select |
| `0xB0+ch` | 1-0: F-Num high | `.Note()` | Frequency high bits |
| `0xC0+ch` | 3-1: Feedback | `.Feedback()` | Modulator self-feedback |
| `0xC0+ch` | 0: Connection | `.Connection()` | FM vs Additive |

### Global Registers

| Register | Bits | DSL Method | Notes |
|----------|------|------------|-------|
| `0x01` | 5: WSE | (auto at init) | Waveform select enable |
| `0xBD` | 7: DAM | `.TremoloDepth()` | Global tremolo depth |
| `0xBD` | 6: DVB | `.VibratoDepth()` | Global vibrato depth |
| `0xBD` | 5: RHY | (future: rhythm mode) | Percussion mode |

---

## What Strudel Features Are NOT Possible on OPL2

| Strudel Feature | Why Not | Alternative |
|----------------|---------|-------------|
| Arbitrary harmonicity ratios | OPL2 MULT is quantized to 16 values | Use available multipliers (0.5, 1-10, 12, 15) |
| Continuous panning | OPL2 is mono; OPL3 only has L/C/R | Use OPL3 mode for basic panning |
| Hardware filters (LPF/HPF) | No filter circuitry on OPL chips | Software post-processing or FM brightness control |
| Sample playback | OPL2 is purely FM synthesis | Use FM patches to approximate timbres |
| Arbitrary waveforms / wavetable | Only 4 waveforms (OPL2) or 8 (OPL3) | Select from available waveforms |
| Phase control | No direct phase register access | Not possible |
| Ring modulation | No ring mod circuitry | Approximate with high-ratio FM |
| Formant filtering (vowel) | No filter hardware | Software post-processing (complex) |
| Granular / striate / chop | Sample-based techniques | Not applicable to FM |
