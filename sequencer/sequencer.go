// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package sequencer

import (
	"math"

	"github.com/jebbisson/spice-synth/voice"
)

// Sequencer drives the pattern playback engine.
type Sequencer struct {
	voices      *voice.Manager
	bpm         float64
	sampleRate  int
	tracks      map[int]*Pattern
	currentTick float64
	modsApplied map[int]bool // tracks which channels have had mods attached
}

// New creates a sequencer attached to a voice manager.
func New(v *voice.Manager, bpm float64, sampleRate int) *Sequencer {
	return &Sequencer{
		voices:      v,
		bpm:         bpm,
		sampleRate:  sampleRate,
		tracks:      make(map[int]*Pattern),
		currentTick: 0,
		modsApplied: make(map[int]bool),
	}
}

// SetBPM changes the tempo.
func (s *Sequencer) SetBPM(bpm float64) {
	s.bpm = bpm
}

// SetPattern sets the active pattern for a channel. Any modulators defined
// on the pattern (via LFO, ModRamp, etc.) are immediately attached to the
// channel on the voice manager.
func (s *Sequencer) SetPattern(channel int, pattern *Pattern) {
	s.tracks[channel] = pattern

	// Attach pattern-level modulators to the voice manager channel.
	if pattern != nil && len(pattern.ModDefs) > 0 {
		s.voices.ClearMods(channel)
		for _, def := range pattern.ModDefs {
			mod := def.Build(s.sampleRate)
			s.voices.AttachMod(channel, mod)
		}
		s.modsApplied[channel] = true
	}
}

// Advance advances the sequencer by the given number of samples.
func (s *Sequencer) Advance(samples int) {
	if s.bpm <= 0 {
		return
	}

	// Calculate ticks per sample:
	// BPM is beats per minute. Let's assume 4 ticks per beat (16th notes).
	// Ticks per second = (BPM * 4) / 60
	// Ticks per sample = (BPM * 4) / (60 * sampleRate)
	ticksPerSample := (s.bpm * 4.0) / (60.0 * float64(s.sampleRate))

	startTick := s.currentTick
	s.currentTick += float64(samples) * ticksPerSample

	// Check for events that should have triggered in this window
	for channel, pattern := range s.tracks {
		if pattern == nil {
			continue
		}

		for _, event := range pattern.Events {
			// The event step is an integer (0..Steps-1)
			// We convert it to a global tick value relative to the start of the pattern loop
			tickPos := float64(event.Step) // Simple 1:1 mapping for now,
			// in a real system we'd handle loop wraps

			// Check if this tick position fell within our window [startTick, currentTick)
			// This is simplified; needs to handle modulo pattern length for looping
			patternDuration := float64(pattern.Steps)

			// Current relative positions in the loop
			relStart := math.Mod(startTick, patternDuration)
			relEnd := math.Mod(s.currentTick, patternDuration)

			triggered := false
			if relStart < relEnd {
				if tickPos >= relStart && tickPos < relEnd {
					triggered = true
				}
			} else {
				// Wrapped around the end of the pattern
				if tickPos >= relStart || tickPos < relEnd {
					triggered = true
				}
			}

			if triggered {
				s.triggerEvent(channel, event)
			}
		}
	}
}

func (s *Sequencer) triggerEvent(channel int, e Event) {
	// This is where the Sequencer talks to the Voice Manager
	switch e.Type {
	case NoteOn:
		inst, err := s.voices.GetInstrument(e.Instrument)
		if err == nil {
			s.voices.NoteOn(channel, e.Note, inst)
		}
	case NoteOff:
		s.voices.NoteOff(channel)
	case InstrumentChange:
		// Instrument change only updates the default instrument for
		// subsequent NoteOn events on this channel.
		if pat, ok := s.tracks[channel]; ok && pat != nil {
			pat.DefaultInstrument = e.Instrument
		}
	case VolumeChange:
		// Set the carrier level (operator 2) directly.
		level := uint8((1.0 - e.Volume) * 63.0)
		s.voices.SetLevel(channel, 1, level)
	case FrequencyChange:
		s.voices.SetFrequency(channel, voice.Note(e.Frequency))
	case LevelChange:
		s.voices.SetLevel(channel, e.Operator, e.Level)
	case FeedbackChange:
		s.voices.SetFeedback(channel, e.Feedback)
	}
}

// Pattern defines a repeating sequence of musical events.
type Pattern struct {
	Steps             int
	Events            []Event
	DefaultInstrument string
	ModDefs           []ModDef // modulator definitions attached to this pattern
}

// NewPattern creates a new pattern with a set number of steps.
func NewPattern(steps int) *Pattern {
	return &Pattern{
		Steps:   steps,
		Events:  []Event{},
		ModDefs: []ModDef{},
	}
}

// Instrument sets the default instrument for this pattern.
func (p *Pattern) Instrument(name string) *Pattern {
	p.DefaultInstrument = name
	return p
}

// Note adds a note event at the specified step.
func (p *Pattern) Note(step int, noteStr string) *Pattern {
	freq, err := voice.ParseNote(noteStr)
	if err != nil {
		return p
	}

	p.Events = append(p.Events, Event{
		Step:       step,
		Type:       NoteOn,
		Note:       voice.Note(freq),
		Instrument: p.DefaultInstrument,
	})
	return p
}

// Off adds a note-off event at the specified step, releasing the channel.
func (p *Pattern) Off(step int) *Pattern {
	p.Events = append(p.Events, Event{
		Step: step,
		Type: NoteOff,
	})
	return p
}

// Hit adds a simple note event (useful for drums).
func (p *Pattern) Hit(step int) *Pattern {
	return p.Note(step, "C2")
}

// ---------------------------------------------------------------------------
// Modulator definitions — these are templates that produce live Modulator
// instances when the pattern is assigned to a channel.
// ---------------------------------------------------------------------------

// ModDef is a blueprint for a Modulator. It is stored on the Pattern and
// instantiated (via Build) when the sequencer assigns the pattern to a
// channel. This indirection lets the same pattern be reused on multiple
// channels, each getting its own modulator state.
type ModDef interface {
	Build(sampleRate int) voice.Modulator
}

// lfoDef stores the parameters for an LFO modulator definition.
type lfoDef struct {
	target voice.ModTarget
	rateHz float64
	depth  float64
	center float64
	shape  voice.WaveShape
}

func (d *lfoDef) Build(_ int) voice.Modulator {
	lfo := voice.NewLFO(d.target, d.rateHz, d.depth, d.shape)
	lfo.Center = d.center
	return lfo
}

// LFO attaches an LFO modulator definition to the pattern.
//
//   - target: which parameter to modulate (e.g. voice.ModCarrierLevel)
//   - rateHz: oscillation speed in Hz (independent of BPM)
//   - depth: peak-to-peak swing (0.0–1.0)
//   - shape: waveform shape (voice.ShapeSine, etc.)
func (p *Pattern) LFO(target voice.ModTarget, rateHz, depth float64, shape voice.WaveShape) *Pattern {
	p.ModDefs = append(p.ModDefs, &lfoDef{
		target: target,
		rateHz: rateHz,
		depth:  depth,
		center: 0.5,
		shape:  shape,
	})
	return p
}

// LFOCentered is like LFO but allows specifying the center value.
func (p *Pattern) LFOCentered(target voice.ModTarget, rateHz, depth, center float64, shape voice.WaveShape) *Pattern {
	p.ModDefs = append(p.ModDefs, &lfoDef{
		target: target,
		rateHz: rateHz,
		depth:  depth,
		center: center,
		shape:  shape,
	})
	return p
}

// rampDef stores the parameters for a Ramp modulator definition.
type rampDef struct {
	target      voice.ModTarget
	from, to    float64
	durationSec float64
}

func (d *rampDef) Build(sampleRate int) voice.Modulator {
	return voice.NewRamp(d.target, d.from, d.to, d.durationSec, sampleRate)
}

// ModRamp attaches a one-shot linear ramp to the pattern.
//
//   - target: which parameter to modulate
//   - from, to: start and end values (0.0–1.0)
//   - durationSec: ramp time in seconds
func (p *Pattern) ModRamp(target voice.ModTarget, from, to, durationSec float64) *Pattern {
	p.ModDefs = append(p.ModDefs, &rampDef{
		target:      target,
		from:        from,
		to:          to,
		durationSec: durationSec,
	})
	return p
}

// envDef stores the parameters for a software Envelope modulator definition.
type envDef struct {
	target                          voice.ModTarget
	attack, decay, sustain, release float64
}

func (d *envDef) Build(_ int) voice.Modulator {
	return voice.NewEnvelope(d.target, d.attack, d.decay, d.sustain, d.release)
}

// ModEnvelope attaches a software ADSR envelope to the pattern.
//
//   - target: which parameter to modulate
//   - attack, decay: times in seconds
//   - sustain: hold level (0.0–1.0)
//   - release: time in seconds
func (p *Pattern) ModEnvelope(target voice.ModTarget, attack, decay, sustain, release float64) *Pattern {
	p.ModDefs = append(p.ModDefs, &envDef{
		target:  target,
		attack:  attack,
		decay:   decay,
		sustain: sustain,
		release: release,
	})
	return p
}

// Event represents something that happens at a step.
type Event struct {
	Step       int
	Type       EventType
	Note       voice.Note
	Instrument string
	Volume     float64

	// FrequencyChange fields
	Frequency float64 // raw Hz for SetFrequency events

	// LevelChange fields
	Operator int   // 0=modulator, 1=carrier
	Level    uint8 // 0-63 attenuation

	// FeedbackChange fields
	Feedback uint8 // 0-7
}

// EventType identifies the kind of sequencer event.
type EventType int

const (
	// NoteOn triggers a note on a channel.
	NoteOn EventType = iota
	// NoteOff releases a note on a channel.
	NoteOff
	// InstrumentChange switches the active instrument on a channel.
	InstrumentChange
	// VolumeChange sets the output level on a channel.
	VolumeChange
	// FrequencyChange changes the pitch without retriggering.
	FrequencyChange
	// LevelChange sets a specific operator's level.
	LevelChange
	// FeedbackChange sets the channel feedback amount.
	FeedbackChange
)
