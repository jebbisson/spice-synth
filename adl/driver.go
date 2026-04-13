// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.
//
// Ported from AdPlug (https://github.com/adplug/adplug), original code by
// Torbjorn Andersson and Johannes Schickel of the ScummVM project.
// Original code is LGPL-2.1. See THIRD_PARTY_LICENSES for details.

package adl

import "github.com/jebbisson/spice-synth/chip"

// callbacksPerSecond is the driver tick rate: 72Hz, matching the original
// Westwood AdLib driver.
const callbacksPerSecond = 72

// channel represents the state of a single ADL channel (0-8 = OPL melodic,
// 9 = control channel).
type channel struct {
	lock      bool
	repeating bool

	opExtraLevel2 uint8

	dataptr         int // Offset into Driver.soundData; -1 = inactive.
	dataptrStack    [4]int
	dataptrStackPos uint8

	duration      uint8
	repeatCounter uint8
	baseOctave    int8
	priority      uint8
	baseNote      int8

	slideTempo uint8
	slideTimer uint8
	slideStep  int16

	vibratoStep           int16
	vibratoStepRange      uint8
	vibratoStepsCountdown uint8
	vibratoNumSteps       uint8
	vibratoDelay          uint8
	vibratoTempo          uint8
	vibratoTimer          uint8
	vibratoDelayCountdown uint8

	opExtraLevel1 uint8
	spacing2      uint8
	baseFreq      uint8
	tempo         uint8
	timer         uint8

	regAx uint8
	regBx uint8

	primaryEffect   int // 0=none, 1=slide, 2=vibrato
	secondaryEffect int // 0=none, 1=secondaryEffect1

	fractionalSpacing uint8
	opLevel1          uint8
	opLevel2          uint8
	opExtraLevel3     uint8
	twoChan           uint8

	unk39 uint8
	unk40 uint8

	spacing1           uint8
	durationRandomness uint8

	secondaryEffectTempo   uint8
	secondaryEffectTimer   uint8
	secondaryEffectSize    int8
	secondaryEffectPos     int8
	secondaryEffectRegbase uint8
	secondaryEffectData    uint16

	tempoReset     uint8
	rawNote        uint8
	pitchBend      int8
	volumeModifier uint8
}

// queueEntry represents a program waiting to be started.
type queueEntry struct {
	dataOffset int // Offset into soundData, -1 = empty.
	id         uint8
	volume     uint8
}

// Driver is the AdLib bytecode interpreter (virtual machine) that runs at
// 72Hz and drives an OPL2 chip. It is a direct port of AdPlug's AdLibDriver.
type Driver struct {
	opl *chip.OPL3

	soundData     []byte
	soundDataSize int
	version       int
	numPrograms   int

	channels     [10]channel
	curChannel   int
	curRegOffset uint8

	tempo         uint8
	callbackTimer uint8
	beatDivider   uint8
	beatDivCnt    uint8
	beatCounter   uint8
	beatWaiting   uint8

	rnd uint16

	soundTrigger uint8

	vibratoAndAMDepthBits uint8
	rhythmSectionBits     uint8

	// Trace callback for debugging. If non-nil, called for key events.
	TraceFunc func(format string, args ...interface{})

	// Rhythm section volume levels.
	opLevelBD, opLevelHH, opLevelSD, opLevelTT, opLevelCY uint8
	opExtraLevel1HH, opExtraLevel2HH                      uint8
	opExtraLevel1CY, opExtraLevel2CY                      uint8
	opExtraLevel2TT, opExtraLevel1TT                      uint8
	opExtraLevel1SD, opExtraLevel2SD                      uint8
	opExtraLevel1BD, opExtraLevel2BD                      uint8

	tablePtr1 int // Offset into soundData for unkTable2 lookups.
	tablePtr2 int

	syncJumpMask uint16

	musicVolume uint8
	sfxVolume   uint8

	programQueue        [16]queueEntry
	programQueueStart   int
	programQueueEnd     int
	programStartTimeout int
	retrySounds         bool

	sfxPointer  int // Offset into soundData for restoring SFX data; -1 = none.
	sfxPriority uint8
	sfxVelocity uint8
}

// NewDriver creates a new ADL bytecode driver attached to the given OPL3 chip.
func NewDriver(opl *chip.OPL3) *Driver {
	d := &Driver{
		opl:           opl,
		rnd:           0x1234,
		callbackTimer: 0xFF,
		musicVolume:   0xFF,
		sfxVolume:     0xFF,
		sfxPointer:    -1,
		tablePtr1:     -1,
		tablePtr2:     -1,
	}
	for i := range d.programQueue {
		d.programQueue[i].dataOffset = -1
	}
	for i := range d.channels {
		d.channels[i].dataptr = -1
	}
	return d
}

func (d *Driver) trace(format string, args ...interface{}) {
	if d.TraceFunc != nil {
		d.TraceFunc(format, args...)
	}
}

// SetVersion sets the ADL format version (1, 3, or 4) and the corresponding
// number of program slots.
func (d *Driver) SetVersion(v int) {
	d.version = v
	switch v {
	case 1:
		d.numPrograms = 150
	case 4:
		d.numPrograms = 500
	default:
		d.numPrograms = 250
	}
}

// SetSoundData sets the raw sound data buffer.
func (d *Driver) SetSoundData(data []byte) {
	d.programQueueStart = 0
	d.programQueueEnd = 0
	d.programQueue[0] = queueEntry{dataOffset: -1}
	d.sfxPointer = -1
	d.soundData = data
	d.soundDataSize = len(data)
}

// StartSound enqueues a program (track) for playback.
func (d *Driver) StartSound(track int, volume uint8) {
	progData := d.getProgram(track)
	if progData < 0 {
		return
	}

	if d.programQueueEnd == d.programQueueStart && d.programQueue[d.programQueueEnd].dataOffset >= 0 {
		return // Queue full.
	}

	d.programQueue[d.programQueueEnd] = queueEntry{
		dataOffset: progData,
		id:         uint8(track),
		volume:     volume,
	}
	d.programQueueEnd = (d.programQueueEnd + 1) & 15
}

// IsChannelPlaying returns true if the given channel (0-9) has active bytecode.
func (d *Driver) IsChannelPlaying(ch int) bool {
	if ch < 0 || ch > 9 {
		return false
	}
	return d.channels[ch].dataptr >= 0
}

// IsChannelRepeating returns true if the channel has reached a backward jump.
func (d *Driver) IsChannelRepeating(ch int) bool {
	if ch < 0 || ch > 9 {
		return false
	}
	return d.channels[ch].repeating
}

// StopAllChannels silences all channels and clears the program queue.
func (d *Driver) StopAllChannels() {
	for ch := 0; ch <= 9; ch++ {
		d.curChannel = ch
		c := &d.channels[ch]
		c.priority = 0
		c.dataptr = -1
		if ch != 9 {
			d.noteOff(c)
		}
	}
	d.retrySounds = false
	d.programQueueStart = 0
	d.programQueueEnd = 0
	d.programQueue[0] = queueEntry{dataOffset: -1}
	d.programStartTimeout = 0
}

// InitDriver resets the OPL state to a clean initial condition.
func (d *Driver) InitDriver() {
	d.resetAdLibState()
}

// Callback executes one 72Hz tick of the driver: starts queued programs,
// executes bytecode for all active channels, and updates the global beat.
func (d *Driver) Callback() {
	if d.programStartTimeout > 0 {
		d.programStartTimeout--
	} else {
		d.setupPrograms()
	}
	d.executePrograms()

	if advance(&d.callbackTimer, d.tempo) {
		d.beatDivCnt--
		if d.beatDivCnt == 0 {
			d.beatDivCnt = d.beatDivider
			d.beatCounter++
		}
	}
}

// --- Internal methods ---

func (d *Driver) getProgram(progID int) int {
	if progID < 0 || progID*2+1 >= d.soundDataSize {
		return -1
	}
	offset := int(d.soundData[progID*2]) | int(d.soundData[progID*2+1])<<8
	if offset == 0 || offset >= d.soundDataSize {
		return -1
	}
	return offset
}

func (d *Driver) getInstrument(instID int) int {
	return d.getProgram(d.numPrograms + instID)
}

func (d *Driver) writeOPL(reg, val uint8) {
	d.opl.WriteRegister(0, reg, val)
}

func (d *Driver) resetAdLibState() {
	d.rnd = 0x1234

	// Enable waveform select.
	d.writeOPL(0x01, 0x20)
	// Select FM music mode (no CSW/NOTE-SEL).
	d.writeOPL(0x08, 0x00)
	// Disable rhythm section.
	d.writeOPL(0xBD, 0x00)

	d.initChannel(&d.channels[9])
	for i := 8; i >= 0; i-- {
		d.writeOPL(0x40+regOffset[i], 0x3F)
		d.writeOPL(0x43+regOffset[i], 0x3F)
		d.initChannel(&d.channels[i])
	}
}

func (d *Driver) initChannel(ch *channel) {
	backupEL2 := ch.opExtraLevel2
	*ch = channel{}
	ch.dataptr = -1
	ch.opExtraLevel2 = backupEL2
	ch.tempo = 0xFF
	ch.spacing1 = 1
	for i := range ch.dataptrStack {
		ch.dataptrStack[i] = -1
	}
}

func (d *Driver) noteOff(ch *channel) {
	if d.curChannel >= 9 {
		return
	}
	if d.rhythmSectionBits != 0 && d.curChannel >= 6 {
		return
	}
	ch.regBx &= 0xDF
	d.writeOPL(0xB0+uint8(d.curChannel), ch.regBx)
}

func (d *Driver) initAdlibChannel(num int) {
	if num >= 9 {
		return
	}
	if d.rhythmSectionBits != 0 && num >= 6 {
		return
	}
	offset := regOffset[num]
	d.writeOPL(0x60+offset, 0xFF)
	d.writeOPL(0x63+offset, 0xFF)
	d.writeOPL(0x80+offset, 0xFF)
	d.writeOPL(0x83+offset, 0xFF)
	d.writeOPL(0xB0+uint8(num), 0x00)
	d.writeOPL(0xB0+uint8(num), 0x20)
}

func (d *Driver) getRandomNr() uint16 {
	d.rnd += 0x9248
	lowBits := d.rnd & 7
	d.rnd >>= 3
	d.rnd |= lowBits << 13
	return d.rnd
}

func (d *Driver) setupDuration(duration uint8, ch *channel) {
	if ch.durationRandomness != 0 {
		ch.duration = duration + uint8(d.getRandomNr()&uint16(ch.durationRandomness))
		return
	}
	if ch.fractionalSpacing != 0 {
		ch.spacing2 = (duration >> 3) * ch.fractionalSpacing
	}
	ch.duration = duration
}

func (d *Driver) setupNote(rawNote uint8, ch *channel, flag bool) {
	if d.curChannel >= 9 {
		return
	}

	ch.rawNote = rawNote

	note := int8(rawNote&0x0F) + ch.baseNote
	octave := int8(((rawNote)+uint8(ch.baseOctave))>>4) & 0x0F

	if note >= 12 {
		octave += int8(note) / 12
		note %= 12
	} else if note < 0 {
		octaves := int8(-(note+1)/12 + 1)
		octave -= octaves
		note += 12 * octaves
	}

	freq := freqTable[note] + uint16(ch.baseFreq)

	if ch.pitchBend != 0 || flag {
		indexNote := rawNote & 0x0F
		if indexNote > 11 {
			indexNote = 11
		}

		if ch.pitchBend >= 0 {
			bend := ch.pitchBend
			if bend > 31 {
				bend = 31
			}
			freq += uint16(pitchBendTables[indexNote+2][bend])
		} else {
			bend := -ch.pitchBend
			if bend > 31 {
				bend = 31
			}
			freq -= uint16(pitchBendTables[indexNote][bend])
		}
	}

	// Clamp octave to 0-7 and shift to bit position.
	if octave < 0 {
		octave = 0
	} else if octave > 7 {
		octave = 7
	}
	octave <<= 2

	ch.regAx = uint8(freq & 0xFF)
	ch.regBx = (ch.regBx & 0x20) | uint8(octave) | uint8((freq>>8)&0x03)

	d.writeOPL(0xA0+uint8(d.curChannel), ch.regAx)
	d.writeOPL(0xB0+uint8(d.curChannel), ch.regBx)
}

func (d *Driver) setupInstrument(regOff uint8, dataOff int, ch *channel) {
	if d.curChannel >= 9 {
		return
	}
	if dataOff < 0 || dataOff+11 > d.soundDataSize {
		return
	}
	data := d.soundData[dataOff:]

	d.writeOPL(0x20+regOff, data[0])
	d.writeOPL(0x23+regOff, data[1])

	temp := data[2]
	d.writeOPL(0xC0+uint8(d.curChannel), temp)
	ch.twoChan = temp & 1

	d.writeOPL(0xE0+regOff, data[3])
	d.writeOPL(0xE3+regOff, data[4])

	ch.opLevel1 = data[5]
	ch.opLevel2 = data[6]

	ol1 := d.calculateOpLevel1(ch)
	ol2 := d.calculateOpLevel2(ch)
	d.writeOPL(0x40+regOff, ol1)
	d.writeOPL(0x43+regOff, ol2)

	d.trace("setupInstrument: ch%d opLevel1=0x%02X opLevel2=0x%02X twoChan=%d "+
		"extraLevel1=0x%02X extraLevel2=0x%02X extraLevel3=0x%02X volMod=0x%02X → reg40=0x%02X reg43=0x%02X",
		d.curChannel, ch.opLevel1, ch.opLevel2, ch.twoChan,
		ch.opExtraLevel1, ch.opExtraLevel2, ch.opExtraLevel3, ch.volumeModifier,
		ol1, ol2)

	d.writeOPL(0x60+regOff, data[7])
	d.writeOPL(0x63+regOff, data[8])

	d.writeOPL(0x80+regOff, data[9])
	d.writeOPL(0x83+regOff, data[10])
}

func (d *Driver) noteOn(ch *channel) {
	if d.curChannel >= 9 {
		return
	}
	d.trace("noteOn: ch%d rawNote=0x%02X regAx=0x%02X regBx=0x%02X", d.curChannel, ch.rawNote, ch.regAx, ch.regBx)
	ch.regBx |= 0x20
	d.writeOPL(0xB0+uint8(d.curChannel), ch.regBx)

	// Update vibrato step based on current frequency.
	shift := int8(9) - clampI8(int8(ch.vibratoStepRange), 0, 9)
	freq := (uint16(ch.regBx)<<8 | uint16(ch.regAx)) & 0x3FF
	ch.vibratoStep = int16((freq >> uint(shift)) & 0xFF)
	ch.vibratoDelayCountdown = ch.vibratoDelay
}

func (d *Driver) adjustVolume(ch *channel) {
	if d.curChannel >= 9 {
		return
	}
	ol2 := d.calculateOpLevel2(ch)
	d.writeOPL(0x43+regOffset[d.curChannel], ol2)
	if ch.twoChan != 0 {
		ol1 := d.calculateOpLevel1(ch)
		d.writeOPL(0x40+regOffset[d.curChannel], ol1)
		d.trace("adjustVolume: ch%d twoChan=1 extraLevel1=0x%02X extraLevel2=0x%02X extraLevel3=0x%02X volMod=0x%02X → reg40=0x%02X reg43=0x%02X",
			d.curChannel, ch.opExtraLevel1, ch.opExtraLevel2, ch.opExtraLevel3, ch.volumeModifier, ol1, ol2)
	} else {
		d.trace("adjustVolume: ch%d twoChan=0 extraLevel1=0x%02X extraLevel2=0x%02X extraLevel3=0x%02X volMod=0x%02X → reg43=0x%02X",
			d.curChannel, ch.opExtraLevel1, ch.opExtraLevel2, ch.opExtraLevel3, ch.volumeModifier, ol2)
	}
}

func (d *Driver) calculateOpLevel1(ch *channel) uint8 {
	value := int16(ch.opLevel1 & 0x3F)
	if ch.twoChan != 0 {
		value += int16(ch.opExtraLevel1)
		value += int16(ch.opExtraLevel2)

		level3 := uint16(ch.opExtraLevel3^0x3F) * uint16(ch.volumeModifier)
		if level3 != 0 {
			level3 += 0x3F
			level3 >>= 8
		}
		value += int16(level3 ^ 0x3F)
	}

	if value < 0 {
		value = 0
	} else if value > 0x3F {
		value = 0x3F
	}

	if ch.volumeModifier == 0 {
		value = 0x3F
	}

	return uint8(value) | (ch.opLevel1 & 0xC0)
}

func (d *Driver) calculateOpLevel2(ch *channel) uint8 {
	value := int16(ch.opLevel2 & 0x3F)
	value += int16(ch.opExtraLevel1)
	value += int16(ch.opExtraLevel2)

	level3 := uint16(ch.opExtraLevel3^0x3F) * uint16(ch.volumeModifier)
	if level3 != 0 {
		level3 += 0x3F
		level3 >>= 8
	}
	value += int16(level3 ^ 0x3F)

	if value < 0 {
		value = 0
	} else if value > 0x3F {
		value = 0x3F
	}

	if ch.volumeModifier == 0 {
		value = 0x3F
	}

	return uint8(value) | (ch.opLevel2 & 0xC0)
}

// --- Primary effects ---

func (d *Driver) primaryEffectSlide(ch *channel) {
	if d.curChannel >= 9 {
		return
	}
	if !advance(&ch.slideTimer, ch.slideTempo) {
		return
	}

	freq := int16((uint16(ch.regBx)&0x03)<<8) | int16(ch.regAx)
	octave := ch.regBx & 0x1C
	noteOn := ch.regBx & 0x20

	step := ch.slideStep
	if step < -0x3FF {
		step = -0x3FF
	} else if step > 0x3FF {
		step = 0x3FF
	}
	freq += step

	if ch.slideStep >= 0 && freq >= 734 {
		freq >>= 1
		if freq&0x3FF == 0 {
			freq++
		}
		octave += 4
	} else if ch.slideStep < 0 && freq < 388 {
		if freq < 0 {
			freq = 0
		}
		freq <<= 1
		if freq&0x3FF == 0 {
			freq--
		}
		octave -= 4
	}

	ch.regAx = uint8(freq & 0xFF)
	ch.regBx = noteOn | (octave & 0x1C) | uint8((freq>>8)&0x03)

	d.writeOPL(0xA0+uint8(d.curChannel), ch.regAx)
	d.writeOPL(0xB0+uint8(d.curChannel), ch.regBx)
}

func (d *Driver) primaryEffectVibrato(ch *channel) {
	if d.curChannel >= 9 {
		return
	}
	if ch.vibratoDelayCountdown > 0 {
		ch.vibratoDelayCountdown--
		return
	}
	if advance(&ch.vibratoTimer, ch.vibratoTempo) {
		ch.vibratoStepsCountdown--
		if ch.vibratoStepsCountdown == 0 {
			ch.vibratoStep = -ch.vibratoStep
			ch.vibratoStepsCountdown = ch.vibratoNumSteps
		}

		freq := (uint16(ch.regBx)<<8 | uint16(ch.regAx)) & 0x3FF
		freq = uint16(int16(freq) + ch.vibratoStep)

		ch.regAx = uint8(freq & 0xFF)
		ch.regBx = (ch.regBx & 0xFC) | uint8(freq>>8)

		d.writeOPL(0xA0+uint8(d.curChannel), ch.regAx)
		d.writeOPL(0xB0+uint8(d.curChannel), ch.regBx)
	}
}

func (d *Driver) secondaryEffect1(ch *channel) {
	if d.curChannel >= 9 {
		return
	}
	if advance(&ch.secondaryEffectTimer, ch.secondaryEffectTempo) {
		ch.secondaryEffectPos--
		if ch.secondaryEffectPos < 0 {
			ch.secondaryEffectPos = ch.secondaryEffectSize
		}
		idx := int(ch.secondaryEffectData) + int(ch.secondaryEffectPos)
		if idx >= 0 && idx < d.soundDataSize {
			d.writeOPL(ch.secondaryEffectRegbase+d.curRegOffset, d.soundData[idx])
		}
	}
}

// --- Program setup and execution ---

func (d *Driver) setupPrograms() {
	entry := &d.programQueue[d.programQueueStart]
	ptr := entry.dataOffset

	if d.programQueueStart == d.programQueueEnd && ptr < 0 {
		return
	}

	var retrySound queueEntry
	retrySound.dataOffset = -1
	if entry.id == 0 {
		d.retrySounds = true
	} else if d.retrySounds {
		retrySound = *entry
	}

	// Clear the queue entry.
	savedEntry := *entry
	entry.dataOffset = -1
	d.programQueueStart = (d.programQueueStart + 1) & 15

	// Safety check.
	if ptr < 0 || ptr+2 > d.soundDataSize {
		return
	}

	chanNum := int(d.soundData[ptr])
	if chanNum > 9 || (chanNum < 9 && ptr+4 > d.soundDataSize) {
		return
	}

	ch := &d.channels[chanNum]

	// Adjust SFX data.
	d.adjustSfxData(ptr, int(savedEntry.volume))
	ptr++

	priority := d.soundData[ptr]
	ptr++

	if priority >= ch.priority {
		d.trace("setupPrograms: starting program id=%d on ch%d (priority=%d, volMod=%d, dataptr=%d)",
			savedEntry.id, chanNum, priority, d.musicVolume, ptr)
		d.initChannel(ch)
		ch.priority = priority
		ch.dataptr = ptr
		ch.tempo = 0xFF
		ch.timer = 0xFF
		ch.duration = 1

		if chanNum <= 5 {
			ch.volumeModifier = d.musicVolume
		} else {
			ch.volumeModifier = d.sfxVolume
		}

		d.initAdlibChannel(chanNum)
		d.programStartTimeout = 2
		retrySound.dataOffset = -1
	} else {
		d.trace("setupPrograms: REJECTED program id=%d on ch%d (priority=%d < existing=%d)",
			savedEntry.id, chanNum, priority, ch.priority)
	}

	if retrySound.dataOffset >= 0 {
		d.StartSound(int(retrySound.id), retrySound.volume)
	}
}

func (d *Driver) adjustSfxData(ptr int, volume int) {
	if d.sfxPointer >= 0 && d.sfxPointer+3 < d.soundDataSize {
		d.soundData[d.sfxPointer+1] = d.sfxPriority
		d.soundData[d.sfxPointer+3] = d.sfxVelocity
		d.sfxPointer = -1
	}

	// Channel 9 is for music control only.
	if ptr < 0 || ptr >= d.soundDataSize || d.soundData[ptr] == 9 {
		return
	}

	d.sfxPointer = ptr
	d.sfxPriority = d.soundData[ptr+1]
	d.sfxVelocity = d.soundData[ptr+3]

	if volume != 0xFF {
		if d.version >= 3 {
			newVal := ((int(d.soundData[ptr+3]) + 63) * volume) >> 8 & 0xFF
			d.soundData[ptr+3] = uint8(-newVal + 63)
			d.soundData[ptr+1] = uint8((int(d.soundData[ptr+1]) * volume) >> 8 & 0xFF)
		} else {
			newVal := (int(d.sfxVelocity)<<2 ^ 0xFF) * volume
			d.soundData[ptr+3] = uint8((newVal >> 10) ^ 0x3F)
			d.soundData[ptr+1] = uint8(newVal >> 11)
		}
	}
}

func (d *Driver) executePrograms() {
	if d.syncJumpMask != 0 {
		for d.curChannel = 9; d.curChannel >= 0; d.curChannel-- {
			if (d.syncJumpMask&(1<<uint(d.curChannel))) != 0 && d.channels[d.curChannel].dataptr >= 0 && !d.channels[d.curChannel].lock {
				break
			}
		}
		if d.curChannel < 0 {
			for d.curChannel = 9; d.curChannel >= 0; d.curChannel-- {
				if d.syncJumpMask&(1<<uint(d.curChannel)) != 0 {
					d.channels[d.curChannel].lock = false
				}
			}
		}
	}

	for d.curChannel = 9; d.curChannel >= 0; d.curChannel-- {
		ch := &d.channels[d.curChannel]
		if ch.dataptr < 0 {
			continue
		}
		if ch.lock && (d.syncJumpMask&(1<<uint(d.curChannel))) != 0 {
			continue
		}

		if d.curChannel == 9 {
			d.curRegOffset = 0
		} else {
			d.curRegOffset = regOffset[d.curChannel]
		}

		if ch.tempoReset != 0 {
			ch.tempo = d.tempo
		}

		result := 1
		if advance(&ch.timer, ch.tempo) {
			ch.duration--
			if ch.duration != 0 {
				if ch.duration == ch.spacing2 {
					d.noteOff(ch)
				}
				if ch.duration == ch.spacing1 && d.curChannel != 9 {
					d.noteOff(ch)
				}
			} else {
				result = 0
			}
		}

		for result == 0 && ch.dataptr >= 0 {
			if ch.dataptr >= d.soundDataSize {
				d.opcodeStopChannel(ch)
				break
			}
			opcode := d.soundData[ch.dataptr]
			ch.dataptr++

			if opcode&0x80 != 0 {
				idx := int(opcode & 0x7F)
				if idx >= len(opcodeParamCount) {
					idx = len(opcodeParamCount) - 1
				}
				nParams := opcodeParamCount[idx]

				if ch.dataptr+nParams > d.soundDataSize {
					d.opcodeStopChannel(ch)
					break
				}

				paramStart := ch.dataptr
				ch.dataptr += nParams
				result = d.executeOpcode(idx, ch, paramStart)
			} else {
				// Note opcode.
				if ch.dataptr >= d.soundDataSize {
					d.opcodeStopChannel(ch)
					break
				}
				duration := d.soundData[ch.dataptr]
				ch.dataptr++

				d.setupNote(opcode, ch, false)
				d.noteOn(ch)
				d.setupDuration(duration, ch)
				if duration != 0 {
					result = 1
				} else {
					result = 0
				}
			}
		}

		if result == 1 {
			d.runPrimaryEffect(ch)
			d.runSecondaryEffect(ch)
		}
	}
}

func (d *Driver) runPrimaryEffect(ch *channel) {
	switch ch.primaryEffect {
	case 1:
		d.primaryEffectSlide(ch)
	case 2:
		d.primaryEffectVibrato(ch)
	}
}

func (d *Driver) runSecondaryEffect(ch *channel) {
	switch ch.secondaryEffect {
	case 1:
		d.secondaryEffect1(ch)
	}
}

// --- Helpers ---

// advance adds tempo to timer, returning true if the timer wraps around.
func advance(timer *uint8, tempo uint8) bool {
	old := *timer
	*timer += tempo
	return *timer < old
}

func clampI8(val, lo, hi int8) int8 {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// readLE16 reads a little-endian uint16 from d.soundData at the given offset.
func (d *Driver) readLE16(offset int) uint16 {
	if offset < 0 || offset+1 >= d.soundDataSize {
		return 0
	}
	return uint16(d.soundData[offset]) | uint16(d.soundData[offset+1])<<8
}

// readBE16 reads a big-endian uint16 from d.soundData at the given offset.
func (d *Driver) readBE16(offset int) uint16 {
	if offset < 0 || offset+1 >= d.soundDataSize {
		return 0
	}
	return uint16(d.soundData[offset])<<8 | uint16(d.soundData[offset+1])
}
