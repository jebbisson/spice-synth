// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package chip

import nukedopl3 "github.com/jebbisson/spice-opl3-nuked"

// OPL3 is an alias for the extracted LGPL-backed implementation.
type OPL3 = nukedopl3.OPL3

// New creates a new OPL3 chip emulator at the given sample rate.
func New(sampleRate uint32) *OPL3 {
	return nukedopl3.New(sampleRate)
}
