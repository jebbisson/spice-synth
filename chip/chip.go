// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package chip

/*
#cgo CFLAGS: -I${SRCDIR}/opl3
#include <stdlib.h>
#include "opl3.h"
#include "opl3.c"

// Helper for sizeof since Go can't call sizeof directly on a type in some contexts
static size_t get_opl3_chip_size() {
    return sizeof(opl3_chip);
}
*/
import "C"
import (
	"errors"
	"unsafe"
)

// OPL3 represents an OPL3/OPL2 chip emulator instance.
type OPL3 struct {
	chip *C.opl3_chip
	rate uint32
}

// New creates a new OPL3 chip emulator at the given sample rate.
func New(sampleRate uint32) *OPL3 {
	cChip := (*C.opl3_chip)(C.malloc(C.size_t(C.get_opl3_chip_size())))
	C.OPL3_Reset(cChip, C.uint32_t(sampleRate))
	return &OPL3{
		chip: cChip,
		rate: sampleRate,
	}
}

// WriteRegister writes a value to an OPL register. The port parameter selects
// the register bank: 0 for the primary bank (OPL2 registers), 1 for the
// secondary bank (OPL3 extended registers). The port is encoded into bit 8 of
// the 16-bit register address expected by the Nuked-OPL3 emulator.
func (o *OPL3) WriteRegister(port uint16, reg uint8, val uint8) {
	fullReg := uint16(reg) | ((port & 0x01) << 8)
	C.OPL3_WriteReg(o.chip, C.uint16_t(fullReg), C.uint8_t(val))
}

// GenerateSamples generates n stereo sample frames.
func (o *OPL3) GenerateSamples(n int) ([]int16, error) {
	if n <= 0 {
		return nil, errors.New("n must be greater than 0")
	}

	numSamples := n * 2
	samples := make([]int16, numSamples)

	C.OPL3_GenerateStream(o.chip, (*C.int16_t)(unsafe.Pointer(&samples[0])), C.uint32_t(n))

	return samples, nil
}

// Close frees the C-allocated OPL3 chip memory. The OPL3 instance must not be
// used after calling Close.
func (o *OPL3) Close() {
	if o.chip != nil {
		C.free(unsafe.Pointer(o.chip))
		o.chip = nil
	}
}

// Reset resets the chip to its initial state.
func (o *OPL3) Reset() {
	C.OPL3_Reset(o.chip, C.uint32_t(o.rate))
}
