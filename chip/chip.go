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

static void opl3_generate_stream_with_meters(opl3_chip *chip, int16_t *sndptr, uint32_t numsamples, uint16_t *meters, uint32_t meter_count, int32_t solo_channel) {
    uint32_t i;
    uint32_t ch;
    uint16_t saved_cha[18];
    uint16_t saved_chb[18];
    uint16_t saved_chc[18];
    uint16_t saved_chd[18];
    int32_t mix[4];

    for (ch = 0; ch < meter_count; ch++) {
        meters[ch] = 0;
    }

    if (solo_channel >= 0 && solo_channel < 18) {
        for (ch = 0; ch < 18; ch++) {
            saved_cha[ch] = chip->channel[ch].cha;
            saved_chb[ch] = chip->channel[ch].chb;
            saved_chc[ch] = chip->channel[ch].chc;
            saved_chd[ch] = chip->channel[ch].chd;
            if ((int32_t)ch != solo_channel) {
                chip->channel[ch].cha = 0;
                chip->channel[ch].chb = 0;
                chip->channel[ch].chc = 0;
                chip->channel[ch].chd = 0;
            }
        }
    }

    for (i = 0; i < numsamples; i++) {
        int16_t sample[2];
        OPL3_GenerateResampled(chip, sample);
        sndptr[i * 2] = sample[0];
        sndptr[i * 2 + 1] = sample[1];

        for (ch = 0; ch < meter_count && ch < 18; ch++) {
            opl3_channel *channel = &chip->channel[ch];
            int32_t accm = *channel->out[0] + *channel->out[1] + *channel->out[2] + *channel->out[3];

            mix[0] = (int16_t)(accm & channel->cha);
            mix[1] = (int16_t)(accm & channel->chb);
            mix[2] = (int16_t)(accm & channel->chc);
            mix[3] = (int16_t)(accm & channel->chd);

            int32_t peak = mix[0];
            if (peak < 0) peak = -peak;
            for (int idx = 1; idx < 4; idx++) {
                int32_t val = mix[idx];
                if (val < 0) val = -val;
                if (val > peak) peak = val;
            }

            if (peak > 65535) {
                peak = 65535;
            }
            if ((uint16_t)peak > meters[ch]) {
                meters[ch] = (uint16_t)peak;
            }
        }
    }

    if (solo_channel >= 0 && solo_channel < 18) {
        for (ch = 0; ch < 18; ch++) {
            chip->channel[ch].cha = saved_cha[ch];
            chip->channel[ch].chb = saved_chb[ch];
            chip->channel[ch].chc = saved_chc[ch];
            chip->channel[ch].chd = saved_chd[ch];
        }
    }
}
*/
import "C"
import (
	"errors"
	"unsafe"
)

// OPL3 represents an OPL3/OPL2 chip emulator instance.
type OPL3 struct {
	chip        *C.opl3_chip
	rate        uint32
	lastMeters  [18]uint16
	soloChannel int
}

// New creates a new OPL3 chip emulator at the given sample rate.
func New(sampleRate uint32) *OPL3 {
	cChip := (*C.opl3_chip)(C.malloc(C.size_t(C.get_opl3_chip_size())))
	C.OPL3_Reset(cChip, C.uint32_t(sampleRate))
	return &OPL3{
		chip:        cChip,
		rate:        sampleRate,
		soloChannel: -1,
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

// WriteRegisterBuffered writes a value to an OPL register using the emulator's
// buffered hardware-timing path.
func (o *OPL3) WriteRegisterBuffered(port uint16, reg uint8, val uint8) {
	fullReg := uint16(reg) | ((port & 0x01) << 8)
	C.OPL3_WriteRegBuffered(o.chip, C.uint16_t(fullReg), C.uint8_t(val))
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

// GenerateSamplesWithMeters generates n stereo sample frames and returns peak
// per-channel output levels for the 18 OPL channels over the same frame block.
func (o *OPL3) GenerateSamplesWithMeters(n int) ([]int16, []uint16, error) {
	if n <= 0 {
		return nil, nil, errors.New("n must be greater than 0")
	}

	numSamples := n * 2
	samples := make([]int16, numSamples)
	meters := make([]uint16, 18)

	C.opl3_generate_stream_with_meters(
		o.chip,
		(*C.int16_t)(unsafe.Pointer(&samples[0])),
		C.uint32_t(n),
		(*C.uint16_t)(unsafe.Pointer(&meters[0])),
		C.uint32_t(len(meters)),
		C.int32_t(o.soloChannel),
	)
	copy(o.lastMeters[:], meters)

	return samples, meters, nil
}

// SetSoloChannel masks audio output to a single OPL channel. Pass -1 to hear
// the full mix again.
func (o *OPL3) SetSoloChannel(ch int) {
	if ch < -1 || ch >= len(o.lastMeters) {
		ch = -1
	}
	o.soloChannel = ch
}

// SoloChannel returns the currently audible solo channel, or -1 for full mix.
func (o *OPL3) SoloChannel() int {
	return o.soloChannel
}

// ChannelMeter returns the most recent normalized peak output level observed
// for the given OPL channel during GenerateSamplesWithMeters.
func (o *OPL3) ChannelMeter(ch int) float64 {
	if ch < 0 || ch >= len(o.lastMeters) {
		return 0
	}
	level := float64(o.lastMeters[ch]) / 65535.0
	if level < 0 {
		return 0
	}
	if level > 1 {
		return 1
	}
	return level
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
	o.soloChannel = -1
	for i := range o.lastMeters {
		o.lastMeters[i] = 0
	}
}
