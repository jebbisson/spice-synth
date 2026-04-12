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
}

// New creates a sequencer attached to a voice manager.
func New(v *voice.Manager, bpm float64, sampleRate int) *Sequencer {
	return &Sequencer{
		voices:      v,
		bpm:         bpm,
		sampleRate:  sampleRate,
		tracks:      make(map[int]*Pattern),
		currentTick: 0,
	}
}

// SetBPM changes the tempo.
func (s *Sequencer) SetBPM(bpm float64) {
	s.bpm = bpm
}

// SetPattern sets the active pattern for a channel.
func (s *Sequencer) SetPattern(channel int, pattern *Pattern) {
	s.tracks[channel] = pattern
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
	}
}

// Pattern defines a repeating sequence of musical events.
type Pattern struct {
	Steps             int
	Events            []Event
	DefaultInstrument string
}

// NewPattern creates a new pattern with a set number of steps.
func NewPattern(steps int) *Pattern {
	return &Pattern{
		Steps:  steps,
		Events: []Event{},
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

// Hit adds a simple note event (useful for drums).
func (p *Pattern) Hit(step int) *Pattern {
	return p.Note(step, "C2")
}

// Event represents something that happens at a step.
type Event struct {
	Step       int
	Type       EventType
	Note       voice.Note
	Instrument string
	Volume     float64
}

type EventType int

const (
	NoteOn EventType = iota
	NoteOff
	InstrumentChange
)
