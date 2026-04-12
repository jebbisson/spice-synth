// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.
//
// Ported from AdPlug (https://github.com/adplug/adplug), original code by
// Torbjorn Andersson and Johannes Schickel of the ScummVM project.
// Original code is LGPL-2.1. See THIRD_PARTY_LICENSES for details.

package adl

// executeOpcode dispatches to the appropriate opcode handler.
// Returns 0 (continue), 1 (stop, run effects), or 2 (stop, skip effects).
func (d *Driver) executeOpcode(idx int, ch *channel, paramStart int) int {
	switch idx {
	case 0:
		return d.opcodeSetRepeat(ch, paramStart)
	case 1:
		return d.opcodeCheckRepeat(ch, paramStart)
	case 2:
		return d.opcodeSetupProgram(ch, paramStart)
	case 3:
		return d.opcodeSetNoteSpacing(ch, paramStart)
	case 4:
		return d.opcodeJump(ch, paramStart)
	case 5:
		return d.opcodeJumpToSubroutine(ch, paramStart)
	case 6:
		return d.opcodeReturnFromSubroutine(ch, paramStart)
	case 7:
		return d.opcodeSetBaseOctave(ch, paramStart)
	case 8, 20, 22, 23, 24, 25, 27, 31, 34, 35, 37, 40, 42, 49, 50, 52, 55, 56, 62, 74:
		return d.opcodeStopChannel(ch)
	case 9:
		return d.opcodePlayRest(ch, paramStart)
	case 10:
		return d.opcodeWriteAdLib(ch, paramStart)
	case 11:
		return d.opcodeSetupNoteAndDuration(ch, paramStart)
	case 12:
		return d.opcodeSetBaseNote(ch, paramStart)
	case 13:
		return d.opcodeSetupSecondaryEffect1(ch, paramStart)
	case 14:
		return d.opcodeStopOtherChannel(ch, paramStart)
	case 15:
		return d.opcodeWaitForEndOfProgram(ch, paramStart)
	case 16:
		return d.opcodeSetupInstrument(ch, paramStart)
	case 17:
		return d.opcodeSetupPrimaryEffectSlide(ch, paramStart)
	case 18:
		return d.opcodeRemovePrimaryEffectSlide(ch)
	case 19:
		return d.opcodeSetBaseFreq(ch, paramStart)
	case 21:
		return d.opcodeSetupPrimaryEffectVibrato(ch, paramStart)
	case 26:
		return d.opcodeSetPriority(ch, paramStart)
	case 28:
		return d.opcodeSetBeat(ch, paramStart)
	case 29:
		return d.opcodeWaitForNextBeat(ch, paramStart)
	case 30:
		return d.opcodeSetExtraLevel1(ch, paramStart)
	case 32:
		return d.opcodeSetupDuration(ch, paramStart)
	case 33:
		return d.opcodePlayNote(ch, paramStart)
	case 36:
		return d.opcodeSetFractionalNoteSpacing(ch, paramStart)
	case 38:
		return d.opcodeSetTempo(ch, paramStart)
	case 39:
		return d.opcodeRemoveSecondaryEffect1(ch)
	case 41:
		return d.opcodeSetChannelTempo(ch, paramStart)
	case 43:
		return d.opcodeSetExtraLevel3(ch, paramStart)
	case 44:
		return d.opcodeSetExtraLevel2(ch, paramStart)
	case 45:
		return d.opcodeChangeExtraLevel2(ch, paramStart)
	case 46:
		return d.opcodeSetAMDepth(ch, paramStart)
	case 47:
		return d.opcodeSetVibratoDepth(ch, paramStart)
	case 48:
		return d.opcodeChangeExtraLevel1(ch, paramStart)
	case 51:
		return d.opcodeClearChannel(ch, paramStart)
	case 53:
		return d.opcodeChangeNoteRandomly(ch, paramStart)
	case 54:
		return d.opcodeRemovePrimaryEffectVibrato(ch)
	case 57:
		return d.opcodePitchBend(ch, paramStart)
	case 58:
		return d.opcodeResetToGlobalTempo(ch)
	case 59, 64:
		return d.opcodeNop()
	case 60:
		return d.opcodeSetDurationRandomness(ch, paramStart)
	case 61:
		return d.opcodeChangeChannelTempo(ch, paramStart)
	case 63:
		return d.opcodeCallback46(ch, paramStart)
	case 65:
		return d.opcodeSetupRhythmSection(ch, paramStart)
	case 66:
		return d.opcodePlayRhythmSection(ch, paramStart)
	case 67:
		return d.opcodeRemoveRhythmSection(ch)
	case 68:
		return d.opcodeSetRhythmLevel2(ch, paramStart)
	case 69:
		return d.opcodeChangeRhythmLevel1(ch, paramStart)
	case 70:
		return d.opcodeSetRhythmLevel1(ch, paramStart)
	case 71:
		return d.opcodeSetSoundTrigger(ch, paramStart)
	case 72:
		return d.opcodeSetTempoReset(ch, paramStart)
	case 73:
		return d.opcodeCallback56(ch, paramStart)
	default:
		return d.opcodeStopChannel(ch)
	}
}

// --- Opcode implementations ---

func (d *Driver) opcodeSetRepeat(ch *channel, p int) int {
	ch.repeatCounter = d.soundData[p]
	return 0
}

func (d *Driver) opcodeCheckRepeat(ch *channel, p int) int {
	ch.repeatCounter--
	if ch.repeatCounter != 0 {
		add := int16(d.readLE16(p))
		newPtr := ch.dataptr + int(add)
		if newPtr >= 0 && newPtr < d.soundDataSize {
			ch.dataptr = newPtr
		}
	}
	return 0
}

func (d *Driver) opcodeSetupProgram(ch *channel, p int) int {
	progID := d.soundData[p]
	if progID == 0xFF {
		return 0
	}

	progOffset := d.getProgram(int(progID))
	if progOffset < 0 || progOffset+2 > d.soundDataSize {
		return 0
	}

	chanNum := int(d.soundData[progOffset])
	priority := d.soundData[progOffset+1]

	if chanNum > 9 {
		return 0
	}

	ch2 := &d.channels[chanNum]

	if priority >= ch2.priority {
		// Backup the calling channel's dataptr.
		backupDataptr := ch.dataptr

		d.programStartTimeout = 2
		d.initChannel(ch2)
		ch2.priority = priority
		ch2.dataptr = progOffset + 2
		ch2.tempo = 0xFF
		ch2.timer = 0xFF
		ch2.duration = 1

		if chanNum <= 5 {
			ch2.volumeModifier = d.musicVolume
		} else {
			ch2.volumeModifier = d.sfxVolume
		}

		d.initAdlibChannel(chanNum)

		// Restore the calling channel's dataptr.
		ch.dataptr = backupDataptr
	}

	return 0
}

func (d *Driver) opcodeSetNoteSpacing(ch *channel, p int) int {
	ch.spacing1 = d.soundData[p]
	return 0
}

func (d *Driver) opcodeJump(ch *channel, p int) int {
	add := int16(d.readLE16(p))

	var newPtr int
	if d.version == 1 {
		newPtr = int(add) - 191
		if newPtr < 0 || newPtr >= d.soundDataSize {
			return d.opcodeStopChannel(ch)
		}
	} else {
		// v2/v3: relative to current dataptr (which is past the params).
		newPtr = ch.dataptr + int(add)
		if newPtr < 0 || newPtr >= d.soundDataSize {
			return d.opcodeStopChannel(ch)
		}
	}

	ch.dataptr = newPtr

	chanIdx := d.curChannel
	if d.syncJumpMask&(1<<uint(chanIdx)) != 0 {
		ch.lock = true
	}
	if add < 0 {
		ch.repeating = true
	}
	return 0
}

func (d *Driver) opcodeJumpToSubroutine(ch *channel, p int) int {
	add := int16(d.readLE16(p))

	if ch.dataptrStackPos >= 4 {
		return 0 // Stack overflow.
	}

	ch.dataptrStack[ch.dataptrStackPos] = ch.dataptr
	ch.dataptrStackPos++

	var newPtr int
	if d.version < 3 {
		newPtr = int(add) - 191
	} else {
		newPtr = ch.dataptr + int(add)
	}

	if newPtr < 0 || newPtr >= d.soundDataSize {
		ch.dataptrStackPos--
		ch.dataptr = ch.dataptrStack[ch.dataptrStackPos]
		return 0
	}
	ch.dataptr = newPtr
	return 0
}

func (d *Driver) opcodeReturnFromSubroutine(ch *channel, _ int) int {
	if ch.dataptrStackPos == 0 {
		return d.opcodeStopChannel(ch)
	}
	ch.dataptrStackPos--
	ch.dataptr = ch.dataptrStack[ch.dataptrStackPos]
	return 0
}

func (d *Driver) opcodeSetBaseOctave(ch *channel, p int) int {
	ch.baseOctave = int8(d.soundData[p])
	return 0
}

func (d *Driver) opcodeStopChannel(ch *channel) int {
	ch.priority = 0
	if d.curChannel != 9 {
		d.noteOff(ch)
	}
	ch.dataptr = -1
	return 2
}

func (d *Driver) opcodePlayRest(ch *channel, p int) int {
	dur := d.soundData[p]
	d.setupDuration(dur, ch)
	d.noteOff(ch)
	if dur != 0 {
		return 1
	}
	return 0
}

func (d *Driver) opcodeWriteAdLib(ch *channel, p int) int {
	d.writeOPL(d.soundData[p], d.soundData[p+1])
	return 0
}

func (d *Driver) opcodeSetupNoteAndDuration(ch *channel, p int) int {
	d.setupNote(d.soundData[p], ch, false)
	dur := d.soundData[p+1]
	d.setupDuration(dur, ch)
	if dur != 0 {
		return 1
	}
	return 0
}

func (d *Driver) opcodeSetBaseNote(ch *channel, p int) int {
	ch.baseNote = int8(d.soundData[p])
	return 0
}

func (d *Driver) opcodeSetupSecondaryEffect1(ch *channel, p int) int {
	ch.secondaryEffectTimer = d.soundData[p]
	ch.secondaryEffectTempo = d.soundData[p]
	ch.secondaryEffectSize = int8(d.soundData[p+1])
	ch.secondaryEffectPos = int8(d.soundData[p+1])
	ch.secondaryEffectRegbase = d.soundData[p+2]

	// The data offset is a uint16 with a -191 correction for the original
	// DOS segment offset. This is a known quirk from the original driver.
	offset := int(d.readLE16(p+3)) - 191
	if offset < 0 {
		offset = 0
	}
	ch.secondaryEffectData = uint16(offset)

	ch.secondaryEffect = 1
	return 0
}

func (d *Driver) opcodeStopOtherChannel(ch *channel, p int) int {
	chanNum := int(d.soundData[p])
	if chanNum < 0 || chanNum > 9 {
		return 0
	}
	other := &d.channels[chanNum]
	other.duration = 0
	other.priority = 0
	other.dataptr = -1
	return 0
}

func (d *Driver) opcodeWaitForEndOfProgram(ch *channel, p int) int {
	progID := d.soundData[p]
	progOffset := d.getProgram(int(progID))
	if progOffset < 0 || progOffset >= d.soundDataSize {
		return 0
	}

	chanNum := int(d.soundData[progOffset])
	if chanNum > 9 {
		return 0
	}

	other := &d.channels[chanNum]
	if other.dataptr >= 0 {
		// Still playing — rewind our dataptr to before this opcode and block.
		ch.dataptr -= 2 // Back past the opcode byte + param byte.
		if other.repeating {
			ch.repeating = true
		}
		return 2
	}
	return 0
}

func (d *Driver) opcodeSetupInstrument(ch *channel, p int) int {
	instID := int(d.soundData[p])
	instOffset := d.getInstrument(instID)
	if instOffset < 0 {
		return 0
	}
	d.setupInstrument(d.curRegOffset, instOffset, ch)
	return 0
}

func (d *Driver) opcodeSetupPrimaryEffectSlide(ch *channel, p int) int {
	ch.slideTempo = d.soundData[p]
	ch.slideStep = int16(d.readBE16(p + 1))
	ch.primaryEffect = 1
	return 0
}

func (d *Driver) opcodeRemovePrimaryEffectSlide(ch *channel) int {
	ch.primaryEffect = 0
	ch.slideStep = 0
	return 0
}

func (d *Driver) opcodeSetBaseFreq(ch *channel, p int) int {
	ch.baseFreq = d.soundData[p]
	return 0
}

func (d *Driver) opcodeSetupPrimaryEffectVibrato(ch *channel, p int) int {
	ch.vibratoTempo = d.soundData[p]
	ch.vibratoStepRange = d.soundData[p+1]
	ch.vibratoStepsCountdown = d.soundData[p+2] >> 1
	ch.vibratoNumSteps = d.soundData[p+2]
	ch.vibratoDelay = d.soundData[p+3]
	ch.primaryEffect = 2
	return 0
}

func (d *Driver) opcodeSetPriority(ch *channel, p int) int {
	ch.priority = d.soundData[p]
	return 0
}

func (d *Driver) opcodeSetBeat(ch *channel, p int) int {
	val := d.soundData[p] >> 1
	d.beatDivider = val
	d.beatDivCnt = val
	d.callbackTimer = 0xFF
	d.beatCounter = 0
	d.beatWaiting = 0
	return 0
}

func (d *Driver) opcodeWaitForNextBeat(ch *channel, p int) int {
	mask := d.soundData[p]
	if (d.beatCounter&mask) != 0 && d.beatWaiting != 0 {
		return 0
	}
	// Not ready — rewind and block.
	ch.dataptr -= 2
	d.beatWaiting = 1
	return 2
}

func (d *Driver) opcodeSetExtraLevel1(ch *channel, p int) int {
	ch.opExtraLevel1 = d.soundData[p]
	d.adjustVolume(ch)
	return 0
}

func (d *Driver) opcodeSetupDuration(ch *channel, p int) int {
	dur := d.soundData[p]
	d.setupDuration(dur, ch)
	if dur != 0 {
		return 1
	}
	return 0
}

func (d *Driver) opcodePlayNote(ch *channel, p int) int {
	dur := d.soundData[p]
	d.setupDuration(dur, ch)
	d.noteOn(ch)
	if dur != 0 {
		return 1
	}
	return 0
}

func (d *Driver) opcodeSetFractionalNoteSpacing(ch *channel, p int) int {
	ch.fractionalSpacing = d.soundData[p] & 7
	return 0
}

func (d *Driver) opcodeSetTempo(ch *channel, p int) int {
	d.tempo = d.soundData[p]
	return 0
}

func (d *Driver) opcodeRemoveSecondaryEffect1(ch *channel) int {
	ch.secondaryEffect = 0
	return 0
}

func (d *Driver) opcodeSetChannelTempo(ch *channel, p int) int {
	ch.tempo = d.soundData[p]
	return 0
}

func (d *Driver) opcodeSetExtraLevel3(ch *channel, p int) int {
	ch.opExtraLevel3 = d.soundData[p]
	return 0
}

func (d *Driver) opcodeSetExtraLevel2(ch *channel, p int) int {
	chanNum := int(d.soundData[p])
	if chanNum > 9 {
		return 0
	}
	other := &d.channels[chanNum]
	other.opExtraLevel2 = d.soundData[p+1]
	// Temporarily switch context to adjust the other channel's volume.
	saveCh := d.curChannel
	saveReg := d.curRegOffset
	d.curChannel = chanNum
	if chanNum < 9 {
		d.curRegOffset = regOffset[chanNum]
	}
	d.adjustVolume(other)
	d.curChannel = saveCh
	d.curRegOffset = saveReg
	return 0
}

func (d *Driver) opcodeChangeExtraLevel2(ch *channel, p int) int {
	chanNum := int(d.soundData[p])
	if chanNum > 9 {
		return 0
	}
	other := &d.channels[chanNum]
	other.opExtraLevel2 += d.soundData[p+1]
	saveCh := d.curChannel
	saveReg := d.curRegOffset
	d.curChannel = chanNum
	if chanNum < 9 {
		d.curRegOffset = regOffset[chanNum]
	}
	d.adjustVolume(other)
	d.curChannel = saveCh
	d.curRegOffset = saveReg
	return 0
}

func (d *Driver) opcodeSetAMDepth(ch *channel, p int) int {
	val := d.soundData[p]
	if val != 0 {
		d.vibratoAndAMDepthBits |= 0x80
	} else {
		d.vibratoAndAMDepthBits &^= 0x80
	}
	d.writeOPL(0xBD, (d.vibratoAndAMDepthBits&0xC0)|d.rhythmSectionBits)
	return 0
}

func (d *Driver) opcodeSetVibratoDepth(ch *channel, p int) int {
	val := d.soundData[p]
	if val != 0 {
		d.vibratoAndAMDepthBits |= 0x40
	} else {
		d.vibratoAndAMDepthBits &^= 0x40
	}
	d.writeOPL(0xBD, (d.vibratoAndAMDepthBits&0xC0)|d.rhythmSectionBits)
	return 0
}

func (d *Driver) opcodeChangeExtraLevel1(ch *channel, p int) int {
	ch.opExtraLevel1 += d.soundData[p]
	d.adjustVolume(ch)
	return 0
}

func (d *Driver) opcodeClearChannel(ch *channel, p int) int {
	chanNum := int(d.soundData[p])
	if chanNum > 9 {
		return 0
	}

	// Stop the channel.
	other := &d.channels[chanNum]
	other.duration = 0
	other.priority = 0
	other.dataptr = -1

	if chanNum < 9 {
		// Silence OPL registers.
		d.writeOPL(0xC0+uint8(chanNum), 0)
		d.writeOPL(0x43+regOffset[chanNum], 0x3F)
		d.writeOPL(0x83+regOffset[chanNum], 0xFF)
		d.writeOPL(0xB0+uint8(chanNum), 0)
	}

	other.opExtraLevel2 = 0
	return 0
}

func (d *Driver) opcodeChangeNoteRandomly(ch *channel, p int) int {
	if d.curChannel >= 9 {
		return 0
	}
	mask := d.readBE16(p)
	freq := (uint16(ch.regBx)<<8 | uint16(ch.regAx)) & 0x3FF
	freq += d.getRandomNr() & mask

	ch.regAx = uint8(freq & 0xFF)
	ch.regBx = (ch.regBx & 0xFC) | uint8(freq>>8&0x03)

	d.writeOPL(0xA0+uint8(d.curChannel), ch.regAx)
	d.writeOPL(0xB0+uint8(d.curChannel), ch.regBx)
	return 0
}

func (d *Driver) opcodeRemovePrimaryEffectVibrato(ch *channel) int {
	ch.primaryEffect = 0
	return 0
}

func (d *Driver) opcodePitchBend(ch *channel, p int) int {
	ch.pitchBend = int8(d.soundData[p])
	d.setupNote(ch.rawNote, ch, true)
	return 0
}

func (d *Driver) opcodeResetToGlobalTempo(ch *channel) int {
	ch.tempo = d.tempo
	return 0
}

func (d *Driver) opcodeNop() int {
	return 0
}

func (d *Driver) opcodeSetDurationRandomness(ch *channel, p int) int {
	ch.durationRandomness = d.soundData[p]
	return 0
}

func (d *Driver) opcodeChangeChannelTempo(ch *channel, p int) int {
	add := int16(int8(d.soundData[p]))
	newTempo := int16(ch.tempo) + add
	if newTempo < 1 {
		newTempo = 1
	} else if newTempo > 255 {
		newTempo = 255
	}
	ch.tempo = uint8(newTempo)
	return 0
}

func (d *Driver) opcodeCallback46(ch *channel, p int) int {
	tableIdx := int(d.soundData[p])
	val := d.soundData[p+1]
	if tableIdx >= 6 {
		return 0
	}
	looked := unkTable2Lookup(tableIdx, int(val))
	if looked != 0 && d.curChannel < 9 {
		freq := (uint16(ch.regBx)<<8 | uint16(ch.regAx)) & 0x3FF
		freq += uint16(looked)
		ch.regAx = uint8(freq & 0xFF)
		ch.regBx = (ch.regBx & 0xFC) | uint8(freq>>8&0x03)
		d.writeOPL(0xA0+uint8(d.curChannel), ch.regAx)
		d.writeOPL(0xB0+uint8(d.curChannel), ch.regBx)
	}
	return 0
}

func (d *Driver) opcodeSetupRhythmSection(ch *channel, p int) int {
	// 9 parameters: 3 instrument IDs + 3 pairs of (regBx, regAx).
	instBD := int(d.soundData[p])
	instHH := int(d.soundData[p+1])
	instSD := int(d.soundData[p+2])

	// Set up bass drum on channel 6.
	bdOff := d.getInstrument(instBD)
	if bdOff >= 0 {
		d.curChannel = 6
		d.curRegOffset = regOffset[6]
		d.setupInstrument(d.curRegOffset, bdOff, &d.channels[6])
		d.channels[6].opLevel1 = d.soundData[bdOff] // Wait — that's wrong. opLevel1/2 are set in setupInstrument.
		d.opLevelBD = d.channels[6].opLevel2
	}

	// Set up hi-hat / tom-tom on channel 7.
	hhOff := d.getInstrument(instHH)
	if hhOff >= 0 {
		d.curChannel = 7
		d.curRegOffset = regOffset[7]
		d.setupInstrument(d.curRegOffset, hhOff, &d.channels[7])
		d.opLevelHH = d.channels[7].opLevel1
		d.opLevelTT = d.channels[7].opLevel2
	}

	// Set up snare / cymbal on channel 8.
	sdOff := d.getInstrument(instSD)
	if sdOff >= 0 {
		d.curChannel = 8
		d.curRegOffset = regOffset[8]
		d.setupInstrument(d.curRegOffset, sdOff, &d.channels[8])
		d.opLevelSD = d.channels[8].opLevel1
		d.opLevelCY = d.channels[8].opLevel2
	}

	// Set frequency registers for rhythm channels 6, 7, 8.
	d.writeOPL(0xA6, d.soundData[p+4])
	d.writeOPL(0xB6, d.soundData[p+3]&0xDF)
	d.writeOPL(0xA7, d.soundData[p+6])
	d.writeOPL(0xB7, d.soundData[p+5]&0xDF)
	d.writeOPL(0xA8, d.soundData[p+8])
	d.writeOPL(0xB8, d.soundData[p+7]&0xDF)

	d.rhythmSectionBits = 0x20
	d.writeOPL(0xBD, (d.vibratoAndAMDepthBits&0xC0)|d.rhythmSectionBits)

	return 0
}

func (d *Driver) opcodePlayRhythmSection(ch *channel, p int) int {
	bits := d.soundData[p]
	d.rhythmSectionBits = 0x20 | (bits & 0x1F)

	// Apply per-instrument volume.
	if bits&0x01 != 0 { // HH
		d.writeOPL(0x51, d.rhythmCalcLevel(d.opLevelHH, d.opExtraLevel1HH, d.opExtraLevel2HH))
	}
	if bits&0x02 != 0 { // CY
		d.writeOPL(0x55, d.rhythmCalcLevel(d.opLevelCY, d.opExtraLevel1CY, d.opExtraLevel2CY))
	}
	if bits&0x04 != 0 { // TT
		d.writeOPL(0x52, d.rhythmCalcLevel(d.opLevelTT, d.opExtraLevel1TT, d.opExtraLevel2TT))
	}
	if bits&0x08 != 0 { // SD
		d.writeOPL(0x51, d.rhythmCalcLevel(d.opLevelSD, d.opExtraLevel1SD, d.opExtraLevel2SD))
	}
	if bits&0x10 != 0 { // BD
		d.writeOPL(0x50, d.rhythmCalcLevel(d.opLevelBD, d.opExtraLevel1BD, d.opExtraLevel2BD))
	}

	d.writeOPL(0xBD, (d.vibratoAndAMDepthBits&0xC0)|d.rhythmSectionBits)
	return 0
}

func (d *Driver) rhythmCalcLevel(base, extra1, extra2 uint8) uint8 {
	val := int16(base&0x3F) + int16(extra1) + int16(extra2)
	if val < 0 {
		val = 0
	} else if val > 0x3F {
		val = 0x3F
	}
	return uint8(val) | (base & 0xC0)
}

func (d *Driver) opcodeRemoveRhythmSection(ch *channel) int {
	d.rhythmSectionBits = 0
	d.writeOPL(0xBD, d.vibratoAndAMDepthBits&0xC0)
	return 0
}

func (d *Driver) opcodeSetRhythmLevel2(ch *channel, p int) int {
	bits := d.soundData[p]
	level := d.soundData[p+1]
	if bits&0x01 != 0 {
		d.opExtraLevel2HH = level
	}
	if bits&0x02 != 0 {
		d.opExtraLevel2CY = level
	}
	if bits&0x04 != 0 {
		d.opExtraLevel2TT = level
	}
	if bits&0x08 != 0 {
		d.opExtraLevel2SD = level
	}
	if bits&0x10 != 0 {
		d.opExtraLevel2BD = level
	}
	return 0
}

func (d *Driver) opcodeChangeRhythmLevel1(ch *channel, p int) int {
	bits := d.soundData[p]
	level := d.soundData[p+1]
	if bits&0x01 != 0 {
		d.opExtraLevel1HH += level
	}
	if bits&0x02 != 0 {
		d.opExtraLevel1CY += level
	}
	if bits&0x04 != 0 {
		d.opExtraLevel1TT += level
	}
	if bits&0x08 != 0 {
		d.opExtraLevel1SD += level
	}
	if bits&0x10 != 0 {
		d.opExtraLevel1BD += level
	}
	return 0
}

func (d *Driver) opcodeSetRhythmLevel1(ch *channel, p int) int {
	bits := d.soundData[p]
	level := d.soundData[p+1]
	if bits&0x01 != 0 {
		d.opExtraLevel1HH = level
	}
	if bits&0x02 != 0 {
		d.opExtraLevel1CY = level
	}
	if bits&0x04 != 0 {
		d.opExtraLevel1TT = level
	}
	if bits&0x08 != 0 {
		d.opExtraLevel1SD = level
	}
	if bits&0x10 != 0 {
		d.opExtraLevel1BD = level
	}
	return 0
}

func (d *Driver) opcodeSetSoundTrigger(ch *channel, p int) int {
	d.soundTrigger = d.soundData[p]
	return 0
}

func (d *Driver) opcodeSetTempoReset(ch *channel, p int) int {
	ch.tempoReset = d.soundData[p]
	return 0
}

func (d *Driver) opcodeCallback56(ch *channel, p int) int {
	ch.unk39 = d.soundData[p]
	ch.unk40 = d.soundData[p+1]
	return 0
}
