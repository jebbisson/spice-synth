// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// Package voice translates high-level musical concepts (notes, instruments,
// channels) into the low-level OPL2 register writes required by the chip
// package. It manages 9 melodic channels, each with two operators (modulator
// and carrier), and handles frequency calculation, instrument application,
// and key-on/key-off events.
package voice
