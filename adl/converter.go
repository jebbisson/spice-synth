// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package adl

import (
	"fmt"
	"math"
	"regexp"
	"strconv"

	adplugadl "github.com/jebbisson/spice-adl-adplug"
	"github.com/jebbisson/spice-synth/chip"
	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/voice"
)

// ---------------------------------------------------------------------------
// Recorded event types
// ---------------------------------------------------------------------------

// recEventType identifies the kind of recorded event captured during ADL simulation.
type recEventType int

const (
	recNoteOn recEventType = iota
	recNoteOff
	recInstrumentChange
	recFreqChange  // pitch change without retrigger (slide, vibrato, pitch bend)
	recChannelStop // channel stopped playing
)

// recEvent is a single event captured during ADL simulation.
type recEvent struct {
	Tick      int // 72Hz tick number when event occurred
	Channel   int // OPL channel 0-8
	Type      recEventType
	Frequency float64 // Hz, for NoteOn and FreqChange
	InstID    int     // instrument index for InstrumentChange
	InstName  string  // instrument name for InstrumentChange
}

// ---------------------------------------------------------------------------
// Recorder — wraps Driver to capture events
// ---------------------------------------------------------------------------

// recorder wraps an ADL Driver and captures musical events during simulation.
type recorder struct {
	driver   *adplugadl.Driver
	opl      chip.Backend
	file     *File
	events   []recEvent
	tick     int
	maxTicks int

	// Per-channel state tracking for deduplication
	curInst    [9]int     // current instrument ID per channel, -1 = none
	lastFreq   [9]float64 // last frequency per channel
	chanActive [9]bool    // whether each channel has an active note

	// Instrument and note-on tracking parsed from trace output.
	instRegex   *regexp.Regexp
	noteOnRegex *regexp.Regexp
}

// newRecorder creates a new recorder for the given ADL file.
func newRecorder(file *File, maxTicks int) *recorder {
	opl := chip.NewBackend(44100) // sample rate doesn't matter for simulation
	driver := adplugadl.NewDriver(opl)
	driver.SetVersion(file.Version)
	driver.SetSoundData(file.SoundData)
	driver.InitDriver()

	r := &recorder{
		driver:      driver,
		opl:         opl,
		file:        file,
		maxTicks:    maxTicks,
		instRegex:   regexp.MustCompile(`setupInstrument: ch(\d+) inst=(\d+)`),
		noteOnRegex: regexp.MustCompile(`noteOn: ch(\d+) rawNote=0x([0-9A-Fa-f]{2}) regAx=0x([0-9A-Fa-f]{2}) regBx=0x([0-9A-Fa-f]{2})`),
	}
	for i := range r.curInst {
		r.curInst[i] = -1
	}

	// Hook the trace function to capture instrument assignments
	driver.SetTraceFunc(r.handleTrace)

	return r
}

// handleTrace processes driver trace messages to capture instrument assignments.
func (r *recorder) handleTrace(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if matches := r.instRegex.FindStringSubmatch(msg); matches != nil {
		ch, _ := strconv.Atoi(matches[1])
		instID, _ := strconv.Atoi(matches[2])
		if ch >= 0 && ch < 9 {
			if r.curInst[ch] != instID {
				r.curInst[ch] = instID
				r.events = append(r.events, recEvent{
					Tick:     r.tick,
					Channel:  ch,
					Type:     recInstrumentChange,
					InstID:   instID,
					InstName: fmt.Sprintf("adl_%03d", instID),
				})
			}
		}
	}
	if matches := r.noteOnRegex.FindStringSubmatch(msg); matches != nil {
		ch, _ := strconv.Atoi(matches[1])
		regAxVal, _ := strconv.ParseUint(matches[3], 16, 8)
		regBxVal, _ := strconv.ParseUint(matches[4], 16, 8)
		if ch >= 0 && ch < 9 {
			freq := regToFreq(uint8(regAxVal), uint8(regBxVal))
			r.events = append(r.events, recEvent{
				Tick:      r.tick,
				Channel:   ch,
				Type:      recNoteOn,
				Frequency: freq,
				InstID:    r.curInst[ch],
				InstName:  r.instName(ch),
			})
			r.lastFreq[ch] = freq
			r.chanActive[ch] = true
		}
	}
}

// close releases resources.
func (r *recorder) close() {
	if r.opl != nil {
		r.opl.Close()
		r.opl = nil
	}
}

// run simulates the ADL driver for up to maxTicks 72Hz ticks, capturing events.
// After each tick, it inspects the driver's channel state to detect note on/off
// and frequency changes.
func (r *recorder) run(subsong int) {
	trackID := r.file.TrackForSubsong(subsong)
	if trackID < 0 {
		return
	}

	r.driver.StartSound(trackID, 0xFF)

	// Track previous state per channel for change detection
	type chanState struct {
		freq    float64
		active  bool
		dataptr int
	}

	prev := [9]chanState{}
	for i := range prev {
		prev[i].dataptr = -1
	}

	for r.tick = 0; r.tick < r.maxTicks; r.tick++ {
		// Snapshot pre-tick state.
		states := r.driver.SnapshotChannels()
		for ch := 0; ch < 9; ch++ {
			c := states[ch]
			prev[ch] = chanState{
				freq:    c.FrequencyHz,
				active:  c.KeyOn,
				dataptr: c.Dataptr,
			}
		}

		// Run one 72Hz tick (this will trigger trace callbacks for instrument setup)
		r.driver.Callback()

		states = r.driver.SnapshotChannels()

		// Detect state changes for each melodic channel
		for ch := 0; ch < 9; ch++ {
			c := states[ch]
			nowActive := c.KeyOn
			wasActive := prev[ch].active

			// Detect note-off (key-on transition from 1 to 0)
			if wasActive && !nowActive {
				r.events = append(r.events, recEvent{
					Tick:    r.tick,
					Channel: ch,
					Type:    recNoteOff,
				})
				r.chanActive[ch] = false
			}

			if nowActive && r.chanActive[ch] {
				// Check for frequency change without retrigger (slide/vibrato)
				freq := c.FrequencyHz
				if math.Abs(freq-prev[ch].freq) > 0.1 && math.Abs(freq-r.lastFreq[ch]) > 0.1 {
					r.events = append(r.events, recEvent{
						Tick:      r.tick,
						Channel:   ch,
						Type:      recFreqChange,
						Frequency: freq,
					})
					r.lastFreq[ch] = freq
				}
			}
		}

		// Check if all channels have stopped or are repeating
		allDone := true
		for ch := 0; ch <= 9; ch++ {
			if r.driver.IsChannelPlaying(ch) && !r.driver.IsChannelRepeating(ch) {
				allDone = false
				break
			}
		}
		if allDone && r.tick > 10 {
			break
		}
	}
}

// instName returns the instrument name for the given channel.
func (r *recorder) instName(ch int) string {
	if r.curInst[ch] >= 0 {
		return fmt.Sprintf("adl_%03d", r.curInst[ch])
	}
	return ""
}

// regToFreq converts OPL2 register values (regAx + regBx) to frequency in Hz.
func regToFreq(regAx, regBx uint8) float64 {
	fnum := uint16(regAx) | (uint16(regBx&0x03) << 8)
	block := (regBx >> 2) & 0x07
	if fnum == 0 {
		return 0
	}
	// f = fnum * 49716 / 2^(20-block)
	return float64(fnum) * 49716.0 / math.Pow(2, float64(20-block))
}

// ---------------------------------------------------------------------------
// ConvertResult holds the output of an ADL-to-DSL conversion
// ---------------------------------------------------------------------------

// ConvertResult holds all the data needed to construct a DSL Song from an
// ADL subsong conversion.
type ConvertResult struct {
	Song        *dsl.Song
	Instruments []*voice.Instrument
	BPM         float64
	TicksUsed   int
	Channels    []int // which OPL channels were active
}

// ---------------------------------------------------------------------------
// Convert — the main entry point
// ---------------------------------------------------------------------------

// Convert runs an ADL subsong through simulation and returns a DSL Song
// that reproduces the same musical output.
//
// Parameters:
//   - file: parsed ADL file
//   - subsong: subsong index to convert
//   - maxSeconds: maximum simulation duration in seconds (0 = auto-detect)
func Convert(file *File, subsong int, maxSeconds float64) (*ConvertResult, error) {
	if file == nil {
		return nil, fmt.Errorf("adl: nil file")
	}
	if subsong < 0 || subsong >= file.NumSubsongs {
		return nil, fmt.Errorf("adl: subsong %d out of range (0-%d)", subsong, file.NumSubsongs-1)
	}

	// Default max simulation: 5 minutes
	if maxSeconds <= 0 {
		maxSeconds = 300
	}
	maxTicks := int(maxSeconds * adplugadl.CallbacksPerSecond)

	rec := newRecorder(file, maxTicks)
	defer rec.close()

	rec.run(subsong)

	if len(rec.events) == 0 {
		return nil, fmt.Errorf("adl: subsong %d produced no events", subsong)
	}

	// Determine which channels were used
	chanUsed := map[int]bool{}
	for _, e := range rec.events {
		if e.Type == recNoteOn || e.Type == recNoteOff || e.Type == recFreqChange {
			chanUsed[e.Channel] = true
		}
	}

	var activeChannels []int
	for ch := 0; ch < 9; ch++ {
		if chanUsed[ch] {
			activeChannels = append(activeChannels, ch)
		}
	}

	// Extract instruments from the file
	instruments := file.ExtractInstruments("adl")

	// BPM mapping: ADL driver runs at 72Hz.
	// DSL sequencer uses 4 ticks per beat.
	// To map 1 ADL tick = 1 DSL tick: BPM = 72 * 60 / 4 = 1080
	bpm := float64(adplugadl.CallbacksPerSecond) * 60.0 / 4.0

	// Determine total song length in ticks
	lastTick := 0
	for _, e := range rec.events {
		if e.Tick > lastTick {
			lastTick = e.Tick
		}
	}

	// Build the Song
	song := dsl.NewSong(bpm)
	for _, inst := range instruments {
		song.AddInstrument(inst)
	}

	// Create one Track per active channel
	for _, ch := range activeChannels {
		track := dsl.NewTrack(ch)
		track.SetLength(lastTick + 1)

		// Find the first instrument for this channel
		firstInst := ""
		for _, e := range rec.events {
			if e.Channel == ch && e.Type == recInstrumentChange {
				firstInst = e.InstName
				break
			}
		}
		if firstInst != "" {
			track.SetInstrument(firstInst)
		}

		// Current instrument tracking for this channel
		curInstName := firstInst

		// Collect events for this channel, in order
		for _, e := range rec.events {
			if e.Channel != ch {
				continue
			}

			switch e.Type {
			case recInstrumentChange:
				if e.InstName != curInstName {
					curInstName = e.InstName
					track.AddEvent(dsl.TrackEvent{
						Tick:       e.Tick,
						Type:       dsl.TrackInstrumentChange,
						Instrument: e.InstName,
					})
				}

			case recNoteOn:
				noteStr := freqToNoteName(e.Frequency)
				instName := curInstName
				if e.InstName != "" {
					instName = e.InstName
				}
				track.AddEvent(dsl.TrackEvent{
					Tick:       e.Tick,
					Type:       dsl.TrackNoteOn,
					Note:       voice.Note(e.Frequency),
					NoteStr:    noteStr,
					Instrument: instName,
				})

			case recNoteOff:
				track.AddEvent(dsl.TrackEvent{
					Tick: e.Tick,
					Type: dsl.TrackNoteOff,
				})

			case recFreqChange:
				track.AddEvent(dsl.TrackEvent{
					Tick:      e.Tick,
					Type:      dsl.TrackFrequencyChange,
					Frequency: e.Frequency,
				})
			}
		}

		song.AddTrack(track)
	}

	return &ConvertResult{
		Song:        song,
		Instruments: instruments,
		BPM:         bpm,
		TicksUsed:   lastTick + 1,
		Channels:    activeChannels,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// freqToNoteName converts a frequency in Hz to the closest note name string.
func freqToNoteName(freq float64) string {
	if freq <= 0 {
		return "C0"
	}

	// Convert frequency to MIDI note number
	midiNote := 69.0 + 12.0*math.Log2(freq/440.0)
	rounded := int(math.Round(midiNote))

	if rounded < 0 {
		rounded = 0
	}
	if rounded > 127 {
		rounded = 127
	}

	noteNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	noteName := noteNames[rounded%12]
	octave := (rounded / 12) - 1

	return fmt.Sprintf("%s%d", noteName, octave)
}
