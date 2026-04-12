// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

// Package chip provides a CGo wrapper around the Nuked-OPL3 emulator.
//
// It exposes a minimal interface for initializing the OPL3 chip, writing
// to hardware registers, and generating signed 16-bit stereo PCM samples.
// The vendored C source (chip/opl3/) is compiled directly into the Go binary
// via CGo and requires a C compiler (gcc or clang) with CGO_ENABLED=1.
package chip
