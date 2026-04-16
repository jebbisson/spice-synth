// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// Package op2 parses OP2 bank files (DMX GENMIDI format) into voice.Instrument
// definitions suitable for OPL2/OPL3 FM synthesis.
//
// The OP2 format stores 175 instrument definitions (128 General MIDI melodic
// instruments + 47 percussion instruments) along with OPL2 register values
// for each operator.
//
// A high-quality DMXOPL bank (MIT-licensed, by sneakernets) is embedded and
// available via DefaultBank().
package op2

import (
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jebbisson/spice-synth/voice"
)

//go:embed GENMIDI.op2
var embeddedBank embed.FS

const (
	headerMagic    = "#OPL_II#"
	headerSize     = 8
	instrumentSize = 36
	nameSize       = 32
	numInstruments = 175
	numMelodic     = 128
	numPercussion  = 47
)

// Instrument flags in the OP2 format.
const (
	FlagFixedNote   = 0x0001 // Use fixed note number instead of MIDI note
	FlagUnknown     = 0x0002 // Unknown / unused
	FlagDoubleVoice = 0x0004 // Uses two voices (pseudo 4-op)
)

// Bank holds a parsed OP2 instrument bank.
type Bank struct {
	Melodic    [numMelodic]Patch    // GM instruments 0-127
	Percussion [numPercussion]Patch // Percussion instruments (GM notes 35-81)
}

// Patch is a single OP2 instrument definition. It may contain two voices
// for double-voice (pseudo 4-op) mode.
type Patch struct {
	Name        string
	Flags       uint16
	FineTune    int8  // Fine tuning offset
	FixedNote   uint8 // If FlagFixedNote is set, use this note number
	Voice1      Voice // Primary voice
	Voice2      Voice // Secondary voice (used when FlagDoubleVoice is set)
	DoubleVoice bool  // Convenience flag derived from Flags
}

// Voice holds the OPL2 register values for a single 2-operator voice.
type Voice struct {
	Modulator  Operator
	Carrier    Operator
	Feedback   uint8 // 0-7: feedback strength
	AddSynth   bool  // true = additive (connection=1), false = FM (connection=0)
	NoteOffset int16 // Note offset for this voice
}

// Operator holds raw OPL2 register values for a single operator.
type Operator struct {
	// Register 0x20: AM/VIB/EG/KSR/Multiply
	Tremolo      bool
	Vibrato      bool
	Sustaining   bool
	KeyScaleRate bool
	Multiply     uint8 // 0-15

	// Register 0x40: KSL/Total Level
	KeyScaleLevel uint8 // 0-3
	Level         uint8 // 0-63

	// Register 0x60: Attack/Decay
	Attack uint8 // 0-15
	Decay  uint8 // 0-15

	// Register 0x80: Sustain/Release
	Sustain uint8 // 0-15
	Release uint8 // 0-15

	// Register 0xE0: Waveform Select
	Waveform uint8 // 0-7 (OPL3) or 0-3 (OPL2)
}

// DefaultBank returns the embedded DMXOPL bank, parsed and ready for use.
func DefaultBank() (*Bank, error) {
	f, err := embeddedBank.Open("GENMIDI.op2")
	if err != nil {
		return nil, fmt.Errorf("op2: failed to open embedded bank: %w", err)
	}
	defer f.Close()
	return Load(f)
}

// Load parses an OP2 bank from the given reader.
func Load(r io.Reader) (*Bank, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("op2: read error: %w", err)
	}

	expectedSize := headerSize + numInstruments*instrumentSize + numInstruments*nameSize
	if len(data) < expectedSize {
		return nil, fmt.Errorf("op2: file too small: got %d bytes, need %d", len(data), expectedSize)
	}

	// Validate magic header.
	if string(data[:headerSize]) != headerMagic {
		return nil, errors.New("op2: invalid header magic")
	}

	bank := &Bank{}

	// Parse instrument data (175 instruments × 36 bytes).
	instData := data[headerSize:]
	for i := 0; i < numInstruments; i++ {
		offset := i * instrumentSize
		patch, err := parsePatch(instData[offset : offset+instrumentSize])
		if err != nil {
			return nil, fmt.Errorf("op2: instrument %d: %w", i, err)
		}

		// Parse name from the name table.
		nameOffset := headerSize + numInstruments*instrumentSize + i*nameSize
		name := parseString(data[nameOffset : nameOffset+nameSize])
		patch.Name = name

		if i < numMelodic {
			bank.Melodic[i] = *patch
		} else {
			bank.Percussion[i-numMelodic] = *patch
		}
	}

	return bank, nil
}

// parsePatch parses a single 36-byte instrument entry.
//
// Layout (36 bytes):
//
//	[0:2]   flags (uint16 LE)
//	[2]     fine tune (int8)
//	[3]     fixed note number (uint8)
//	[4:20]  voice 1 (16 bytes)
//	[20:36] voice 2 (16 bytes)
//
// Each voice (16 bytes):
//
//	[0]  modulator characteristic (reg 0x20)
//	[1]  modulator attack/decay (reg 0x60)
//	[2]  modulator sustain/release (reg 0x80)
//	[3]  modulator waveform (reg 0xE0)
//	[4]  modulator key scale level (reg 0x40 bits 6-7)
//	[5]  modulator output level (reg 0x40 bits 0-5)
//	[6]  feedback/connection (reg 0xC0)
//	[7]  carrier characteristic (reg 0x20)
//	[8]  carrier attack/decay (reg 0x60)
//	[9]  carrier sustain/release (reg 0x80)
//	[10] carrier waveform (reg 0xE0)
//	[11] carrier key scale level (reg 0x40 bits 6-7)
//	[12] carrier output level (reg 0x40 bits 0-5)
//	[13] reserved
//	[14:16] note offset (int16 LE)
func parsePatch(data []byte) (*Patch, error) {
	if len(data) < instrumentSize {
		return nil, errors.New("insufficient data")
	}

	flags := binary.LittleEndian.Uint16(data[0:2])

	p := &Patch{
		Flags:       flags,
		FineTune:    int8(data[2]),
		FixedNote:   data[3],
		DoubleVoice: flags&FlagDoubleVoice != 0,
	}

	p.Voice1 = parseVoice(data[4:20])
	p.Voice2 = parseVoice(data[20:36])

	return p, nil
}

// parseVoice parses a 16-byte voice definition.
func parseVoice(data []byte) Voice {
	v := Voice{
		Modulator: parseOperator(data[0], data[1], data[2], data[3], data[4], data[5]),
		Carrier:   parseOperator(data[7], data[8], data[9], data[10], data[11], data[12]),
	}

	// Feedback/connection byte (reg 0xC0 format).
	fbConn := data[6]
	v.Feedback = (fbConn >> 1) & 0x07
	v.AddSynth = fbConn&0x01 != 0

	// Note offset (int16 LE).
	v.NoteOffset = int16(binary.LittleEndian.Uint16(data[14:16]))

	return v
}

// parseOperator parses the raw register bytes into an Operator struct.
func parseOperator(char, attackDecay, sustainRelease, waveform, ksl, level uint8) Operator {
	return Operator{
		// Characteristic byte (reg 0x20): TVSK MMMM
		Tremolo:      char&0x80 != 0,
		Vibrato:      char&0x40 != 0,
		Sustaining:   char&0x20 != 0,
		KeyScaleRate: char&0x10 != 0,
		Multiply:     char & 0x0F,

		// KSL and Level (reg 0x40)
		KeyScaleLevel: ksl & 0x03,
		Level:         level & 0x3F,

		// Attack/Decay (reg 0x60)
		Attack: (attackDecay >> 4) & 0x0F,
		Decay:  attackDecay & 0x0F,

		// Sustain/Release (reg 0x80)
		Sustain: (sustainRelease >> 4) & 0x0F,
		Release: sustainRelease & 0x0F,

		// Waveform (reg 0xE0)
		Waveform: waveform & 0x07,
	}
}

// parseString extracts a null-terminated string from a fixed-size buffer.
func parseString(data []byte) string {
	n := 0
	for n < len(data) && data[n] != 0 {
		n++
	}
	return strings.TrimSpace(string(data[:n]))
}

// ToInstrument converts an OP2 Patch's primary voice to a voice.Instrument.
func (p *Patch) ToInstrument() *voice.Instrument {
	return voiceToInstrument(p.Name, &p.Voice1)
}

// ToInstruments converts an OP2 Patch to one or two voice.Instrument values.
// Double-voice patches return two instruments; single-voice patches return one.
func (p *Patch) ToInstruments() []*voice.Instrument {
	result := []*voice.Instrument{voiceToInstrument(p.Name, &p.Voice1)}
	if p.DoubleVoice {
		result = append(result, voiceToInstrument(p.Name+"_b", &p.Voice2))
	}
	return result
}

// voiceToInstrument converts an OP2 Voice to a voice.Instrument.
func voiceToInstrument(name string, v *Voice) *voice.Instrument {
	conn := uint8(0)
	if v.AddSynth {
		conn = 1
	}
	return &voice.Instrument{
		Name: name,
		Op1: voice.Operator{
			Attack:        v.Modulator.Attack,
			Decay:         v.Modulator.Decay,
			Sustain:       v.Modulator.Sustain,
			Release:       v.Modulator.Release,
			Level:         v.Modulator.Level,
			Multiply:      v.Modulator.Multiply,
			KeyScaleRate:  v.Modulator.KeyScaleRate,
			KeyScaleLevel: v.Modulator.KeyScaleLevel,
			Tremolo:       v.Modulator.Tremolo,
			Vibrato:       v.Modulator.Vibrato,
			Sustaining:    v.Modulator.Sustaining,
			Waveform:      v.Modulator.Waveform,
		},
		Op2: voice.Operator{
			Attack:        v.Carrier.Attack,
			Decay:         v.Carrier.Decay,
			Sustain:       v.Carrier.Sustain,
			Release:       v.Carrier.Release,
			Level:         v.Carrier.Level,
			Multiply:      v.Carrier.Multiply,
			KeyScaleRate:  v.Carrier.KeyScaleRate,
			KeyScaleLevel: v.Carrier.KeyScaleLevel,
			Tremolo:       v.Carrier.Tremolo,
			Vibrato:       v.Carrier.Vibrato,
			Sustaining:    v.Carrier.Sustaining,
			Waveform:      v.Carrier.Waveform,
		},
		Feedback:   v.Feedback,
		Connection: conn,
	}
}

// MelodicInstrument returns the voice.Instrument for a GM program number (0-127).
func (b *Bank) MelodicInstrument(program uint8) *voice.Instrument {
	if program >= numMelodic {
		program = 0
	}
	return b.Melodic[program].ToInstrument()
}

// PercussionInstrument returns the voice.Instrument for a GM percussion note.
// GM percussion notes range from 35-81, mapped to percussion slots 0-46.
func (b *Bank) PercussionInstrument(note uint8) *voice.Instrument {
	if note < 35 || note > 81 {
		return b.Percussion[0].ToInstrument()
	}
	return b.Percussion[note-35].ToInstrument()
}

// PercussionNote returns the fixed note number for a GM percussion instrument.
// If the percussion patch has a fixed note set, that is returned; otherwise
// the input note is returned unchanged.
func (b *Bank) PercussionNote(note uint8) uint8 {
	if note < 35 || note > 81 {
		return note
	}
	p := &b.Percussion[note-35]
	if p.Flags&FlagFixedNote != 0 {
		return p.FixedNote
	}
	return note
}
