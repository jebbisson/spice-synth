// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package voice

import (
	"errors"
	"fmt"
	"math"

	"github.com/jebbisson/spice-synth/chip"
)

// OPL2 operator offset table. Channels 0-8 map to non-contiguous register offsets.
var oplOperatorOffsets = [9][2]uint8{
	{0x00, 0x03}, // Channel 0: modulator=0x00, carrier=0x03
	{0x01, 0x04}, // Channel 1: modulator=0x01, carrier=0x04
	{0x02, 0x05}, // Channel 2: modulator=0x02, carrier=0x05
	{0x08, 0x0B}, // Channel 3: modulator=0x08, carrier=0x0B
	{0x09, 0x0C}, // Channel 4: modulator=0x09, carrier=0x0C
	{0x0A, 0x0D}, // Channel 5: modulator=0x0A, carrier=0x0D
	{0x10, 0x13}, // Channel 6: modulator=0x10, carrier=0x13
	{0x11, 0x14}, // Channel 7: modulator=0x11, carrier=0x14
	{0x12, 0x15}, // Channel 8: modulator=0x12, carrier=0x15
}

// Note represents a musical note.
type Note float64

// Instrument defines an OPL2 FM instrument.
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

// OperatorOverride applies absolute operator parameter overrides to a base
// instrument at note start.
type OperatorOverride struct {
	Attack        *uint8
	Decay         *uint8
	Sustain       *uint8
	Release       *uint8
	Level         *uint8
	Multiply      *uint8
	KeyScaleRate  *bool
	KeyScaleLevel *uint8
	Tremolo       *bool
	Vibrato       *bool
	Sustaining    *bool
	Waveform      *uint8
}

func (o OperatorOverride) empty() bool {
	return o.Attack == nil && o.Decay == nil && o.Sustain == nil && o.Release == nil &&
		o.Level == nil && o.Multiply == nil && o.KeyScaleRate == nil && o.KeyScaleLevel == nil &&
		o.Tremolo == nil && o.Vibrato == nil && o.Sustaining == nil && o.Waveform == nil
}

// InstrumentOverride applies absolute instrument parameter overrides to a base
// instrument at note start.
type InstrumentOverride struct {
	Op1        OperatorOverride
	Op2        OperatorOverride
	Feedback   *uint8
	Connection *uint8
}

// Empty reports whether the override changes anything.
func (o *InstrumentOverride) Empty() bool {
	if o == nil {
		return true
	}
	return o.Op1.empty() && o.Op2.empty() && o.Feedback == nil && o.Connection == nil
}

// Clone returns a deep copy of the instrument.
func (i *Instrument) Clone() *Instrument {
	if i == nil {
		return nil
	}
	clone := *i
	clone.Op1 = i.Op1
	clone.Op2 = i.Op2
	return &clone
}

// ApplyTo returns a cloned instrument with the override applied.
func (o *InstrumentOverride) ApplyTo(base *Instrument) *Instrument {
	if base == nil {
		return nil
	}
	if o == nil || o.Empty() {
		return base.Clone()
	}
	out := base.Clone()
	applyOperatorOverride(&out.Op1, o.Op1)
	applyOperatorOverride(&out.Op2, o.Op2)
	if o.Feedback != nil {
		out.Feedback = *o.Feedback
	}
	if o.Connection != nil {
		out.Connection = *o.Connection
	}
	return out
}

func applyOperatorOverride(dst *Operator, override OperatorOverride) {
	if dst == nil {
		return
	}
	if override.Attack != nil {
		dst.Attack = *override.Attack
	}
	if override.Decay != nil {
		dst.Decay = *override.Decay
	}
	if override.Sustain != nil {
		dst.Sustain = *override.Sustain
	}
	if override.Release != nil {
		dst.Release = *override.Release
	}
	if override.Level != nil {
		dst.Level = *override.Level
	}
	if override.Multiply != nil {
		dst.Multiply = *override.Multiply
	}
	if override.KeyScaleRate != nil {
		dst.KeyScaleRate = *override.KeyScaleRate
	}
	if override.KeyScaleLevel != nil {
		dst.KeyScaleLevel = *override.KeyScaleLevel
	}
	if override.Tremolo != nil {
		dst.Tremolo = *override.Tremolo
	}
	if override.Vibrato != nil {
		dst.Vibrato = *override.Vibrato
	}
	if override.Sustaining != nil {
		dst.Sustaining = *override.Sustaining
	}
	if override.Waveform != nil {
		dst.Waveform = *override.Waveform
	}
}

// Manager handles OPL2's 9 melodic channels and their real-time modulators.
type Manager struct {
	chip        chip.Backend
	sampleRate  int
	channels    [9]*channel
	instruments map[string]*Instrument
}

type channel struct {
	id          int
	currentNote Note
	currentInst *Instrument
	active      bool
	mods        []Modulator // active per-channel modulators
}

// NewManager creates a new voice manager attached to the chip.
func NewManager(c chip.Backend, sampleRate int) *Manager {
	m := &Manager{
		chip:        c,
		sampleRate:  sampleRate,
		instruments: make(map[string]*Instrument),
	}
	for i := 0; i < 9; i++ {
		m.channels[i] = &channel{id: i}
	}

	// Enable waveform select (register 0x01 bit 5). Without this, all
	// operators are limited to pure sine regardless of the Waveform field.
	if c != nil {
		c.WriteRegisterBuffered(0, 0x01, 0x20)
	}

	return m
}

// NoteOn triggers a note on the specified channel with the given instrument.
func (m *Manager) NoteOn(channelID int, note Note, inst *Instrument) error {
	return m.NoteOnWithOverride(channelID, note, inst, nil)
}

// NoteOnWithOverride triggers a note using a base instrument plus optional
// note-start overrides, applying the effective instrument before key-on.
func (m *Manager) NoteOnWithOverride(channelID int, note Note, inst *Instrument, override *InstrumentOverride) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	if inst == nil {
		return errors.New("instrument cannot be nil")
	}
	if override != nil && !override.Empty() {
		inst = override.ApplyTo(inst)
	}

	ch := m.channels[channelID]
	retrigger := ch.active
	ch.currentNote = note
	ch.currentInst = inst
	ch.active = true

	m.applyInstrument(ch, inst, retrigger)
	return nil
}

// NoteOff releases the note on the channel by clearing the key-on bit.
func (m *Manager) NoteOff(channelID int) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	ch := m.channels[channelID]
	ch.active = false

	// Clear the key-on bit (bit 5) in register 0xB0+channel to release the note.
	cid := uint8(ch.id)
	fnum, block := FNumberAndBlock(float64(ch.currentNote))
	m.writeRegister(0xB0+cid, uint8(block<<2)|uint8((fnum>>8)&0x03))

	return nil
}

func (m *Manager) applyInstrument(ch *channel, inst *Instrument, retrigger bool) {
	if ch.currentNote == 0 && !ch.active {
		return
	}

	cid := uint8(ch.id)
	modOff := oplOperatorOffsets[cid][0]
	carOff := oplOperatorOffsets[cid][1]

	// 1. Set Operator parameters
	// Register 0x20+offset: AM/VIB/EG/KSR/Multiply
	m.writeOperatorAMVIB(0x20+modOff, &inst.Op1)
	m.writeOperatorAMVIB(0x20+carOff, &inst.Op2)

	// Register 0x40+offset: KSL/Total Level (attenuation)
	m.writeRegister(0x40+modOff, (inst.Op1.KeyScaleLevel<<6)|inst.Op1.Level)
	m.writeRegister(0x40+carOff, (inst.Op2.KeyScaleLevel<<6)|inst.Op2.Level)

	// Register 0x60+offset: Attack/Decay
	m.writeRegister(0x60+modOff, (inst.Op1.Attack<<4)|inst.Op1.Decay)
	m.writeRegister(0x60+carOff, (inst.Op2.Attack<<4)|inst.Op2.Decay)

	// Register 0x80+offset: Sustain/Release
	m.writeRegister(0x80+modOff, (inst.Op1.Sustain<<4)|inst.Op1.Release)
	m.writeRegister(0x80+carOff, (inst.Op2.Sustain<<4)|inst.Op2.Release)

	// Register 0xE0+offset: Waveform Select
	m.writeRegister(0xE0+modOff, inst.Op1.Waveform&0x03)
	m.writeRegister(0xE0+carOff, inst.Op2.Waveform&0x03)

	// 2. Set Channel-level parameters (Feedback and Connection)
	connBit := uint8(0)
	if inst.Connection != 0 {
		connBit = 1
	}
	m.writeRegister(0xC0+cid, (inst.Feedback<<1)|connBit)

	// 3. Set Frequency and Key-On
	fnum, block := FNumberAndBlock(float64(ch.currentNote))
	m.writeRegister(0xA0+cid, uint8(fnum&0xFF))

	if ch.active {
		b0 := uint8(block<<2) | uint8((fnum>>8)&0x03)
		if retrigger {
			// Repeated note-ons on the same channel must pulse key-off before
			// key-on again so extracted ADL beats keep their attack thump.
			m.writeRegister(0xB0+cid, b0)
		}
		m.writeRegister(0xB0+cid, b0|0x20)
	} else {
		m.writeRegister(0xB0+cid, uint8(block<<2)|uint8((fnum>>8)&0x03))
	}
}

// writeOperatorAMVIB writes the AM/VIB/EG/KSR/Multiply register for an operator.
func (m *Manager) writeOperatorAMVIB(reg uint8, op *Operator) {
	val := op.Multiply & 0x0F
	if op.KeyScaleRate {
		val |= 0x10
	}
	if op.Sustaining {
		val |= 0x20
	}
	if op.Vibrato {
		val |= 0x40
	}
	if op.Tremolo {
		val |= 0x80
	}
	m.writeRegister(reg, val)
}

func (m *Manager) writeRegister(reg uint8, val uint8) {
	// Keep DSL/stream playback on the same buffered OPL timing path as ADL.
	if m.chip == nil {
		return
	}
	m.chip.WriteRegisterBuffered(0, reg, val)
}

// GetInstrument retrieves an instrument by name from the manager's bank.
func (m *Manager) GetInstrument(name string) (*Instrument, error) {
	inst, ok := m.instruments[name]
	if !ok {
		return nil, fmt.Errorf("instrument not found: %s", name)
	}
	return inst, nil
}

// LoadBank loads a set of instruments into the manager, indexed by each
// instrument's Name field. The bankName parameter is reserved for future use.
func (m *Manager) LoadBank(bankName string, insts []*Instrument) {
	for _, inst := range insts {
		m.instruments[inst.Name] = inst
	}
	_ = bankName
}

// ---------------------------------------------------------------------------
// Real-time parameter control (no retrigger)
// ---------------------------------------------------------------------------

// SetLevel writes the total level (attenuation) register for a single operator
// on a sounding channel without retriggering the note.
//
//   - op 0 = modulator (Op1), op 1 = carrier (Op2)
//   - level 0 = loudest, 63 = silent
//
// The key-scale level bits are preserved from the current instrument.
func (m *Manager) SetLevel(channelID int, op int, level uint8) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	if op < 0 || op > 1 {
		return fmt.Errorf("invalid operator: %d (must be 0 or 1)", op)
	}
	if level > 63 {
		level = 63
	}

	ch := m.channels[channelID]
	cid := uint8(ch.id)
	off := oplOperatorOffsets[cid][op]

	// Preserve KSL bits from the current instrument if available.
	var ksl uint8
	if ch.currentInst != nil {
		if op == 0 {
			ksl = ch.currentInst.Op1.KeyScaleLevel
		} else {
			ksl = ch.currentInst.Op2.KeyScaleLevel
		}
	}

	m.writeRegister(0x40+off, (ksl<<6)|level)
	return nil
}

// SetFrequency changes the pitch of a sounding channel without retriggering.
// The key-on state is preserved.
func (m *Manager) SetFrequency(channelID int, note Note) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}

	ch := m.channels[channelID]
	cid := uint8(ch.id)
	fnum, block := FNumberAndBlock(float64(note))

	m.writeRegister(0xA0+cid, uint8(fnum&0xFF))
	b0 := uint8(block<<2) | uint8((fnum>>8)&0x03)
	if ch.active {
		b0 |= 0x20 // preserve key-on
	}
	m.writeRegister(0xB0+cid, b0)
	return nil
}

// SetFeedback changes the feedback amount on a sounding channel without
// retriggering. The connection mode is preserved from the current instrument.
func (m *Manager) SetFeedback(channelID int, fb uint8) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	if fb > 7 {
		fb = 7
	}

	ch := m.channels[channelID]
	cid := uint8(ch.id)

	connBit := uint8(0)
	if ch.currentInst != nil && ch.currentInst.Connection != 0 {
		connBit = 1
	}
	m.writeRegister(0xC0+cid, (fb<<1)|connBit)
	return nil
}

// SetTremoloDepth sets the global hardware tremolo depth.
// shallow=false: ~1 dB, shallow=true: ~4.8 dB.
func (m *Manager) SetTremoloDepth(deep bool) {
	m.writeGlobalBD(deep, false)
}

// SetVibratoDepth sets the global hardware vibrato depth.
// deep=false: ~7 cents, deep=true: ~14 cents.
func (m *Manager) SetVibratoDepth(deep bool) {
	m.writeGlobalBD(false, deep)
}

// writeGlobalBD writes register 0xBD (tremolo/vibrato depth flags).
func (m *Manager) writeGlobalBD(deepTremolo, deepVibrato bool) {
	var val uint8
	if deepTremolo {
		val |= 0x80
	}
	if deepVibrato {
		val |= 0x40
	}
	m.writeRegister(0xBD, val)
}

// ---------------------------------------------------------------------------
// Modulator management
// ---------------------------------------------------------------------------

// AttachMod adds a modulator to the specified channel. Multiple modulators
// can target different parameters simultaneously (e.g. an LFO on carrier
// level and a ramp on feedback).
func (m *Manager) AttachMod(channelID int, mod Modulator) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	m.channels[channelID].mods = append(m.channels[channelID].mods, mod)
	return nil
}

// ClearMods removes all modulators from the specified channel.
func (m *Manager) ClearMods(channelID int) error {
	if channelID < 0 || channelID >= 9 {
		return fmt.Errorf("invalid channel: %d", channelID)
	}
	m.channels[channelID].mods = nil
	return nil
}

// Tick advances all active modulators by the given number of samples and
// applies their outputs to the corresponding OPL2 registers. This should be
// called by the stream layer between sub-blocks of sample generation.
//
// When multiple modulators target the same parameter, their normalised
// outputs are multiplied together before being applied. This allows, for
// example, an envelope controlling the overall volume contour while an LFO
// adds wobble on top.
func (m *Manager) Tick(samples int) {
	for _, ch := range m.channels {
		if len(ch.mods) == 0 {
			continue
		}

		// Collect per-target combined values. Start at 1.0 (multiplicative
		// identity) and multiply each modulator's output into it.
		combined := make(map[ModTarget]float64)
		active := make(map[ModTarget]bool)

		alive := ch.mods[:0]
		for _, mod := range ch.mods {
			val := mod.Tick(samples, m.sampleRate)
			t := mod.Target()
			if !active[t] {
				combined[t] = val
				active[t] = true
			} else {
				combined[t] *= val
			}
			if !mod.Done() {
				alive = append(alive, mod)
			}
		}
		ch.mods = alive

		// Apply the combined value for each target once.
		for t, val := range combined {
			m.applyModValue(ch, t, val)
		}
	}
}

// Close releases the underlying OPL3 chip reference. The Manager must not be
// used after calling Close.
func (m *Manager) Close() {
	if m.chip == nil {
		return
	}
	m.chip.Close()
	m.chip = nil
}

// applyModValue maps a normalised modulator output (0.0–1.0) to the correct
// OPL2 register write for the given target.
func (m *Manager) applyModValue(ch *channel, target ModTarget, val float64) {
	cid := uint8(ch.id)

	switch target {
	case ModCarrierLevel:
		// 0.0 → level 63 (silent), 1.0 → level 0 (loudest).
		// This inversion matches the OPL2 convention where 0 = loudest.
		level := uint8((1.0 - val) * 63.0)
		// Add the instrument's base attenuation if available.
		if ch.currentInst != nil {
			total := int(level) + int(ch.currentInst.Op2.Level)
			if total > 63 {
				total = 63
			}
			level = uint8(total)
		}
		off := oplOperatorOffsets[cid][1]
		var ksl uint8
		if ch.currentInst != nil {
			ksl = ch.currentInst.Op2.KeyScaleLevel
		}
		m.writeRegister(0x40+off, (ksl<<6)|level)

	case ModModulatorLevel:
		level := uint8((1.0 - val) * 63.0)
		if ch.currentInst != nil {
			total := int(level) + int(ch.currentInst.Op1.Level)
			if total > 63 {
				total = 63
			}
			level = uint8(total)
		}
		off := oplOperatorOffsets[cid][0]
		var ksl uint8
		if ch.currentInst != nil {
			ksl = ch.currentInst.Op1.KeyScaleLevel
		}
		m.writeRegister(0x40+off, (ksl<<6)|level)

	case ModFeedback:
		// 0.0 → feedback 0, 1.0 → feedback 7.
		fb := uint8(val * 7.0)
		if fb > 7 {
			fb = 7
		}
		connBit := uint8(0)
		if ch.currentInst != nil && ch.currentInst.Connection != 0 {
			connBit = 1
		}
		m.writeRegister(0xC0+cid, (fb<<1)|connBit)

	case ModFrequency:
		// val 0.5 = base pitch (no change), 0.0 = -1 octave, 1.0 = +1 octave.
		// Implemented as semitone offset: (val - 0.5) * 24 semitones.
		if ch.currentNote == 0 {
			return
		}
		semitoneOffset := (val - 0.5) * 24.0
		baseFreq := float64(ch.currentNote)
		ratio := 1.0
		if semitoneOffset != 0 {
			ratio = math.Pow(2.0, semitoneOffset/12.0)
		}
		newFreq := baseFreq * ratio
		fnum, block := FNumberAndBlock(newFreq)
		m.writeRegister(0xA0+cid, uint8(fnum&0xFF))
		b0 := uint8(block<<2) | uint8((fnum>>8)&0x03)
		if ch.active {
			b0 |= 0x20
		}
		m.writeRegister(0xB0+cid, b0)
	}
}
