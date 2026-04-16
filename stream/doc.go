// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// Package stream is the top-level entry point for SpiceSynth. It implements
// [io.Reader] to produce an infinite stream of signed 16-bit stereo
// little-endian PCM audio data. Each call to Read advances the internal
// sequencer and generates samples from the OPL3 chip, making it compatible
// with audio backends such as Ebiten and Oto.
package stream
