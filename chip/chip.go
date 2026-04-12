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
	"fmt"
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

// WriteRegister writes a value to an OPL register.
func (o *OPL3) WriteRegister(port uint16, reg uint8, val uint8) {
	C.OPL3_WriteReg(o.chip, C.uint16_t(reg), C.uint8_t(val))
}

// GenerateSamples generates n stereo sample frames.
func (o *OPL3) GenerateSamples(n int) ([]int16, error) {
	if n <= 0 {
		return nil, errors.New("n must be greater than 0")
	}

	numSamples := n * 2
	samples := make([]int16, numSamples)

	C.OPL3_GenerateStream(o.chip, (*C.int16_t)(unsafe.Pointer(&samples[0])), C.uint32_t(n))

	for i := 0; i < len(samples); i++ {
		if samples[i] != 0 {
			fmt.Printf("[DEBUG-CHIP] Non-zero sample found: %d at index %d\n", samples[i], i)
			return samples, nil
		}
	}
	fmt.Printf("[DEBUG-CHIP] All %d samples are silent\n", numSamples)

	return samples, nil
}

// Reset resets the chip to its initial state.
func (o *OPL3) Reset() {
	C.OPL3_Reset(o.chip, C.uint32_t(o.rate))
}
