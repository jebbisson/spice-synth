// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

// Package player provides a MIDI file player that renders General MIDI files
// through OPL2 FM synthesis using the Nuked-OPL3 emulator.
//
// The player manages multiple OPL3 chip instances to provide unlimited
// polyphony — new chips are allocated on demand as more simultaneous voices
// are needed. Audio from all chips is mixed together and exposed as a
// standard io.Reader producing signed 16-bit stereo little-endian PCM data.
//
// Basic usage:
//
//	bank, _ := op2.DefaultBank()
//	midiFile, _ := midi.Parse(f)
//	p := player.New(44100, bank, midiFile)
//	p.Play()
//	// Use p as an io.Reader for audio output.
package player

import (
	"sync"

	"github.com/jebbisson/spice-synth/chip"
	"github.com/jebbisson/spice-synth/midi"
	"github.com/jebbisson/spice-synth/op2"
	"github.com/jebbisson/spice-synth/voice"
)

// subBlockSize is the number of sample frames rendered between event
// processing steps. This matches the stream package's value for
// consistent modulation granularity.
const subBlockSize = 64

// State represents the player's current state.
type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
	StateDone // Playback finished (reached end of MIDI file)
)

// Player renders a MIDI file through OPL2 FM synthesis.
type Player struct {
	mu sync.Mutex

	sampleRate int
	bank       *op2.Bank
	file       *midi.File

	state State

	// Merged, time-sorted event list with sample positions.
	events    []timedEvent
	eventIdx  int    // Next event to process
	samplePos uint64 // Current sample position

	// OPL chip pool: each chipSlot has a chip + voice manager + 9 channel slots.
	chips []*chipSlot

	// MIDI channel state (16 channels).
	channels [16]midiChannel

	// Active voice allocations.
	voices []*voiceAlloc

	// Audio mixing.
	masterVol float64
	gain      float64

	// Fade-in state.
	fadeInSamples int
}

// chipSlot wraps a single OPL3 chip instance with its voice manager.
type chipSlot struct {
	chip   *chip.OPL3
	voices *voice.Manager
	inUse  [9]bool // Which of the 9 OPL2 channels are in use
}

// midiChannel tracks the state of a single MIDI channel (0-15).
type midiChannel struct {
	program uint8 // Current GM program number
	volume  uint8 // CC7 volume (0-127), default 100
	pan     uint8 // CC10 pan (0-127), default 64
}

// voiceAlloc tracks a single active OPL voice.
type voiceAlloc struct {
	midiChannel uint8  // Which MIDI channel this voice belongs to
	midiNote    uint8  // MIDI note number
	chipIdx     int    // Index into Player.chips
	oplChannel  int    // OPL channel on that chip (0-8)
	startSample uint64 // When this voice was triggered
}

// timedEvent is a MIDI event with its absolute sample position.
type timedEvent struct {
	sample uint64
	event  midi.Event
}

// New creates a new MIDI player.
func New(sampleRate int, bank *op2.Bank, file *midi.File) *Player {
	p := &Player{
		sampleRate:    sampleRate,
		bank:          bank,
		file:          file,
		state:         StateStopped,
		masterVol:     1.0,
		gain:          3.0,
		fadeInSamples: sampleRate / 20, // 50ms fade-in
	}

	// Initialize MIDI channels with defaults.
	for i := range p.channels {
		p.channels[i].volume = 100
		p.channels[i].pan = 64
	}

	// Build the merged event list.
	p.events = p.buildEventList()

	return p
}

// Play starts or resumes playback.
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StateStopped || p.state == StatePaused {
		p.state = StatePlaying
	}
}

// Pause pauses playback. Audio output continues (silence) but events stop.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StatePlaying {
		p.state = StatePaused
	}
}

// Stop stops playback and resets to the beginning.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = StateStopped
	p.eventIdx = 0
	p.samplePos = 0
	p.releaseAllVoices()
	p.freeAllChips()
}

// State returns the current player state.
func (p *Player) GetState() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// SetMasterVolume sets the master output volume (0.0 - 1.0).
func (p *Player) SetMasterVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.masterVol = v
}

// SetGain sets the output gain multiplier.
func (p *Player) SetGain(g float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gain = g
}

// Progress returns (current, total) in samples.
func (p *Player) Progress() (uint64, uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := uint64(0)
	if len(p.events) > 0 {
		total = p.events[len(p.events)-1].sample
	}
	return p.samplePos, total
}

// Read fills b with signed 16-bit stereo little-endian PCM data.
// This implements io.Reader for use with audio output libraries.
func (p *Player) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	totalFrames := len(b) / 4
	if totalFrames == 0 {
		return 0, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	byteOffset := 0
	framesLeft := totalFrames

	for framesLeft > 0 {
		n := subBlockSize
		if n > framesLeft {
			n = framesLeft
		}

		// Process MIDI events that fall within this sub-block.
		if p.state == StatePlaying {
			p.processEvents(n)
		}

		// Tick modulators on all active chips.
		for _, cs := range p.chips {
			cs.voices.Tick(n)
		}

		// Generate and mix audio from all chips.
		p.mixAudio(b[byteOffset:], n)

		if p.state == StatePlaying {
			p.samplePos += uint64(n)
		}

		byteOffset += n * 4
		framesLeft -= n
	}

	return totalFrames * 4, nil
}

// buildEventList merges all tracks into a single time-ordered event list
// with absolute sample positions calculated from tempo changes.
func (p *Player) buildEventList() []timedEvent {
	if p.file == nil || len(p.file.Tracks) == 0 {
		return nil
	}

	division := float64(p.file.Division)
	if division == 0 {
		division = 96
	}

	// Collect all events from all tracks.
	type rawEvent struct {
		tick  uint32
		event midi.Event
	}

	var all []rawEvent
	for _, track := range p.file.Tracks {
		for _, ev := range track.Events {
			all = append(all, rawEvent{tick: ev.Tick, event: ev})
		}
	}

	// Sort by tick (stable sort to preserve order within same tick).
	// Simple insertion sort is fine for the sizes we're dealing with.
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].tick < all[j-1].tick; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}

	// Convert ticks to sample positions, handling tempo changes.
	usPerBeat := float64(500000) // Default 120 BPM
	var currentTick uint32
	var currentSample float64

	result := make([]timedEvent, 0, len(all))

	for _, raw := range all {
		// Advance time from currentTick to raw.tick at the current tempo.
		if raw.tick > currentTick {
			deltaTicks := float64(raw.tick - currentTick)
			secondsPerTick := usPerBeat / (division * 1_000_000.0)
			currentSample += deltaTicks * secondsPerTick * float64(p.sampleRate)
			currentTick = raw.tick
		}

		result = append(result, timedEvent{
			sample: uint64(currentSample),
			event:  raw.event,
		})

		// Update tempo if this is a tempo change event.
		if raw.event.Type == midi.TempoChange {
			usPerBeat = float64(raw.event.Tempo)
		}
	}

	return result
}

// processEvents processes all MIDI events that fall within the next n samples.
func (p *Player) processEvents(n int) {
	endSample := p.samplePos + uint64(n)

	for p.eventIdx < len(p.events) {
		ev := &p.events[p.eventIdx]
		if ev.sample > endSample {
			break
		}
		p.handleEvent(&ev.event)
		p.eventIdx++
	}

	// Check if we've reached the end of the file.
	if p.eventIdx >= len(p.events) {
		p.state = StateDone
	}
}

// handleEvent processes a single MIDI event.
func (p *Player) handleEvent(ev *midi.Event) {
	switch ev.Type {
	case midi.NoteOn:
		p.noteOn(ev.Channel, ev.Data1, ev.Data2)
	case midi.NoteOff:
		p.noteOff(ev.Channel, ev.Data1)
	case midi.ProgramChange:
		p.channels[ev.Channel].program = ev.Data1
	case midi.ControlChange:
		p.controlChange(ev.Channel, ev.Data1, ev.Data2)
	case midi.TempoChange:
		// Tempo is already handled in buildEventList for sample position calculation.
	case midi.EndOfTrack:
		// Handled by the event list exhaustion check.
	}
}

// noteOn triggers a note, allocating a new OPL voice.
func (p *Player) noteOn(channel, note, velocity uint8) {
	if velocity == 0 {
		p.noteOff(channel, note)
		return
	}

	// Get the instrument from the bank.
	var inst *voice.Instrument
	if channel == 9 {
		// Percussion channel: use percussion bank.
		inst = p.bank.PercussionInstrument(note)
	} else {
		inst = p.bank.MelodicInstrument(p.channels[channel].program)
	}

	if inst == nil {
		return
	}

	// Apply velocity scaling to the carrier level.
	inst = p.applyVelocity(inst, velocity, channel)

	// Allocate a voice.
	chipIdx, oplCh := p.allocateVoice()

	cs := p.chips[chipIdx]

	// Determine the note frequency.
	var freq float64
	if channel == 9 {
		// Percussion: use the bank's fixed note or the MIDI note.
		percNote := p.bank.PercussionNote(note)
		freq = voice.NoteToFrequency(int(percNote))
	} else {
		freq = voice.NoteToFrequency(int(note))
	}

	// Trigger the note.
	cs.voices.NoteOn(oplCh, voice.Note(freq), inst)
	cs.inUse[oplCh] = true

	// Track the voice allocation.
	p.voices = append(p.voices, &voiceAlloc{
		midiChannel: channel,
		midiNote:    note,
		chipIdx:     chipIdx,
		oplChannel:  oplCh,
		startSample: p.samplePos,
	})
}

// noteOff releases a note, finding and freeing the corresponding OPL voice.
func (p *Player) noteOff(channel, note uint8) {
	// Find the matching voice allocation (most recent if duplicates).
	for i := len(p.voices) - 1; i >= 0; i-- {
		v := p.voices[i]
		if v.midiChannel == channel && v.midiNote == note {
			// Release the OPL voice.
			cs := p.chips[v.chipIdx]
			cs.voices.NoteOff(v.oplChannel)
			cs.inUse[v.oplChannel] = false

			// Remove from the active list.
			p.voices = append(p.voices[:i], p.voices[i+1:]...)
			return
		}
	}
}

// controlChange handles MIDI CC messages.
func (p *Player) controlChange(channel, cc, value uint8) {
	switch cc {
	case 7: // Channel Volume
		p.channels[channel].volume = value
	case 10: // Pan
		p.channels[channel].pan = value
	case 123: // All Notes Off
		p.allNotesOff(channel)
	}
}

// allNotesOff releases all active voices on a MIDI channel.
func (p *Player) allNotesOff(channel uint8) {
	for i := len(p.voices) - 1; i >= 0; i-- {
		v := p.voices[i]
		if v.midiChannel == channel {
			cs := p.chips[v.chipIdx]
			cs.voices.NoteOff(v.oplChannel)
			cs.inUse[v.oplChannel] = false
			p.voices = append(p.voices[:i], p.voices[i+1:]...)
		}
	}
}

// allocateVoice finds a free OPL channel, creating a new chip if needed.
func (p *Player) allocateVoice() (chipIdx, oplChannel int) {
	// Search existing chips for a free channel.
	for ci, cs := range p.chips {
		for ch := 0; ch < 9; ch++ {
			if !cs.inUse[ch] {
				return ci, ch
			}
		}
	}

	// No free channels — allocate a new chip.
	cs := &chipSlot{
		chip: chip.New(uint32(p.sampleRate)),
	}
	cs.voices = voice.NewManager(cs.chip, p.sampleRate)
	p.chips = append(p.chips, cs)

	return len(p.chips) - 1, 0
}

// applyVelocity creates a copy of the instrument with velocity and channel
// volume applied to the carrier level.
func (p *Player) applyVelocity(inst *voice.Instrument, velocity, channel uint8) *voice.Instrument {
	// Clone the instrument.
	out := *inst
	out.Op1 = inst.Op1
	out.Op2 = inst.Op2

	// Scale carrier level by velocity and channel volume.
	// OPL2 level: 0 = loudest, 63 = silent.
	// velocity: 0-127, channel volume: 0-127.
	velScale := float64(velocity) / 127.0
	volScale := float64(p.channels[channel].volume) / 127.0
	combinedScale := velScale * volScale

	// Convert to attenuation: more attenuation = quieter.
	// At full velocity+volume, use the instrument's native level.
	// At lower values, add attenuation.
	extraAtten := int((1.0 - combinedScale) * 32.0) // Up to 32 levels of extra attenuation
	newLevel := int(out.Op2.Level) + extraAtten
	if newLevel > 63 {
		newLevel = 63
	}
	out.Op2.Level = uint8(newLevel)

	return &out
}

// mixAudio generates n frames of audio from all active chips and writes
// the mixed result into b.
func (p *Player) mixAudio(b []byte, n int) {
	if len(p.chips) == 0 {
		// Silence.
		for i := 0; i < n*4 && i < len(b); i++ {
			b[i] = 0
		}
		return
	}

	// Generate samples from each chip and accumulate.
	mixed := make([]float64, n*2) // Stereo: L, R, L, R, ...

	for _, cs := range p.chips {
		samples, err := cs.chip.GenerateSamples(n)
		if err != nil {
			continue
		}
		for i, s := range samples {
			mixed[i] += float64(s)
		}
	}

	// Apply gain, master volume, fade-in, and convert to int16 PCM.
	for i := 0; i < n*2; i++ {
		frameIdx := int(p.samplePos) + i/2
		fadeScale := 1.0
		if frameIdx < p.fadeInSamples {
			fadeScale = float64(frameIdx) / float64(p.fadeInSamples)
		}

		scaled := mixed[i] * p.gain * p.masterVol * fadeScale

		// Soft clip to prevent harsh digital clipping.
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}

		out := int16(scaled)
		idx := i * 2
		if idx+1 < len(b) {
			b[idx] = byte(out & 0xFF)
			b[idx+1] = byte(out >> 8)
		}
	}
}

// releaseAllVoices sends NoteOff to all active voices.
func (p *Player) releaseAllVoices() {
	for _, v := range p.voices {
		if v.chipIdx < len(p.chips) {
			cs := p.chips[v.chipIdx]
			cs.voices.NoteOff(v.oplChannel)
			cs.inUse[v.oplChannel] = false
		}
	}
	p.voices = nil
}

// freeAllChips closes and removes all OPL chip instances.
func (p *Player) freeAllChips() {
	for _, cs := range p.chips {
		cs.chip.Close()
	}
	p.chips = nil
}

// Close releases all resources. The Player must not be used after calling Close.
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = StateStopped
	p.releaseAllVoices()
	p.freeAllChips()
}

// ActiveVoices returns the number of currently sounding OPL voices.
func (p *Player) ActiveVoices() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.voices)
}

// ActiveChips returns the number of OPL chip instances currently allocated.
func (p *Player) ActiveChips() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.chips)
}

// PositionSeconds returns the current playback position in seconds.
func (p *Player) PositionSeconds() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return float64(p.samplePos) / float64(p.sampleRate)
}

// DurationSeconds returns the total duration of the MIDI file in seconds.
func (p *Player) DurationSeconds() float64 {
	if p.file == nil {
		return 0
	}
	return p.file.Duration()
}
