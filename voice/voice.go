// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package voice

import (
	"errors"
	"fmt"

	"github.com/jebbisson/spice-synth/chip"
)

// Note represents a musical note.
type Note float64

// Instrument defines an OPL2 FM instrument patch.
type Instrument struct {
	Name string

	// Operator parameters (modulator = Op1, carrier = Op2)
	Op1 Operator
	Op2 Operator

	// Channel-level parameters
	Feedback   uint8 // 0-7: modulator feedback strength
	Connection uint8 // 0 = FM (Op1 modulates Op2), 1 = Additive (both output)
}

// Operator defines a single OPL2 FM operator.
type Operator struct {
	Attack        uint8 // 0-15: attack rate
	Decay         uint8 // 0-15: decay rate
	Sustain       uint8 // 0-15: sustain level (inverted: 0 = max, 15 = min)
	Release       uint8 // 0-15: release rate
	Level         uint8 // 0-63: output level (attenuation, 0 = loudest)
	Multiply      uint8 // 0-15: frequency multiplier
	KeyScaleRate  bool  // envelope rate scales with note
	KeyScaleLevel uint8 // 0-3: output attenuation scales with note
	Tremolo       bool  // amplitude vibrato
	Vibrato       bool  // frequency vibrato
	Sustaining    bool  // if true, sustain phase holds until key-off
	Waveform      uint8 // 0-3: sine, half-sine, abs-sine, quarter-sine
}

// Manager handles OPL2's 9 melodic channels.
type Manager struct {
	chip        *chip.OPL3
	channels    [9]*channel
	instruments map[string]*Instrument
}

type channel struct {
	id          int
	currentNote Note
	currentInst *Instrument
	active      bool
}

// NewManager creates a new voice manager attached to the chip.
func NewManager(c *chip.OPL3) *Manager {
	m := &Manager{
		chip:        c,
		instruments: make(map[string]*Instrument),
	}
	for i := 0; i < 9; i++ {
		m.channels[i] = &channel{id: i}
	}
	return m
}

// NoteOn triggers a note on the specified channel with the enough instrument.
func (m *Manager) NoteOn(channelID int, note Note, inst *Instrument) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	if inst == nil {
		return errors.New("instrument cannot be nil")
	}

	ch := m.channels[channelID]
	ch.currentNote = note
	ch.currentInst = inst
	ch.active = true

	// This is where the magic of translating Instrument struct to OPL registers happens.
	fmt.Printf("[DEBUG] Channel %d NoteOn: %.2f with inst %s\n", channelID, note, inst.Name)
	m.applyInstrument(ch, inst)
	return nil
}

// NoteOff releases the note on the channel.
func (m *Manager) NoteOff(channelID int) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	ch := m.channels[channelID]
	ch.active = false
	return nil
}

func (m *Manager) applyInstrument(ch *channel, inst *Instrument) {
	if ch.currentNote == 0 && !ch.active {
		return
	}

	// 1. Set Operator parameters FIRST
	// OPL2 registers are NOT contiguous per channel.
	// They are grouped by parameter across channels.
	cid := uint8(ch.id)

	// Modulator (Op1)
	m.chip.WriteRegister(0, 0x40+cid, (inst.Op1.Multiply<<4)|(inst.Op1.Waveform&0x0F))
	m.chip.WriteRegister(0, 0x46+cid, inst.Op1.Attack)
	m.chip.WriteRegister(0, 0x4C+cid, inst.Op1.Decay)
	m.chip.WriteRegister(0, 0x52+cid, inst.Op1.Sustain)
	m.chip.WriteRegister(0, 0x58+cid, inst.Op1.Release)
	m.chip.WriteRegister(0, 0x5E+cid, inst.Op1.Level)

	// Carrier (Op2)
	m.chip.WriteRegister(0, 0x60+cid, (inst.Op2.Multiply<<4)|(inst.Op2.Waveform&0x0F))
	m.chip.WriteRegister(0, 0x66+cid, inst.Op2.Attack)
	m.chip.WriteRegister(0, 0x6C+cid, inst.Op2.Decay)
	m.chip.WriteRegister(0, 0x72+cid, inst.Op2.Sustain)
	m.chip.WriteRegister(0, 0x78+cid, inst.Op2.Release)
	m.chip.WriteRegister(0, 0x7E+cid, inst.Op2.Level)

	// 2. Set Channel-level parameters (Feedback and Connection)
	connBit := uint8(0)
	if inst.Connection != 0 {
		connBit = 1
	}
	m.chip.WriteRegister(0, 0xC0+cid, (inst.Feedback<<4)|connBit)

	// 3. Set Frequency and Block LAST to trigger the sound
	fnum, block := FNumberAndBlock(float64(ch.currentNote))
	m.chip.WriteRegister(0, 0xA0+cid, uint8(fnum&0xFF))

	if ch.active {
		m.chip.WriteRegister(0, 0xB0+cid, uint8(block)|uint8(fnum>>8)|0x20)
	} else {
		m.chip.WriteRegister(0, 0xB0+cid, uint8(block)|uint8(fnum>>8))
	}
}

// GetInstrument retrieves an instrument by name from the manager's bank.
func (m *Manager) GetInstrument(name string) (*Instrument, error) {
	inst, ok := m.instruments[name]
	if !ok {
		return nil, fmt.Errorf("instrument not found: %s", name)
	}
	return inst, nil
}

// LoadBank loads a set of instruments into the manager.
func (m *Manager) LoadBank(name string, insts []*Instrument) {
	for _, inst := range insts {
		m.instruments[inst.Name] = inst
	}
	_ = name // Suppress unused variable error
}
