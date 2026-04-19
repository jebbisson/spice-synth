// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package dsl

import (
	"fmt"

	"github.com/jebbisson/spice-synth/sequencer"
	"github.com/jebbisson/spice-synth/stream"
	"github.com/jebbisson/spice-synth/voice"
)

// ---------------------------------------------------------------------------
// Song — top-level composition
// ---------------------------------------------------------------------------

// Song represents a complete multi-track musical composition. It holds a set
// of instruments and tracks that are compiled to the voice manager and
// sequencer when Play() is called.
//
// Usage:
//
//	song := dsl.NewSong(120) // 120 BPM
//	song.AddInstrument(bass)
//	song.AddTrack(bassTrack)
//	song.Play(stream)
type Song struct {
	bpm         float64
	instruments []*voice.Instrument
	tracks      []*Track
}

// NewSong creates a new song at the given BPM.
func NewSong(bpm float64) *Song {
	return &Song{bpm: bpm}
}

// BPM returns the song's tempo.
func (s *Song) BPM() float64 {
	return s.bpm
}

// SetBPM changes the song's tempo.
func (s *Song) SetBPM(bpm float64) *Song {
	s.bpm = bpm
	return s
}

// Slow slows down the song by a factor. Slow(2) halves the BPM.
func (s *Song) Slow(factor float64) *Song {
	if factor > 0 {
		s.bpm /= factor
	}
	return s
}

// Fast speeds up the song by a factor. Fast(2) doubles the BPM.
func (s *Song) Fast(factor float64) *Song {
	if factor > 0 {
		s.bpm *= factor
	}
	return s
}

// AddInstrument registers an instrument for use by the song's tracks.
func (s *Song) AddInstrument(inst *voice.Instrument) *Song {
	s.instruments = append(s.instruments, inst)
	return s
}

// AddTrack adds a track to the song. The track's Channel field determines
// which OPL2 channel it is assigned to (0-8).
func (s *Song) AddTrack(t *Track) *Song {
	s.tracks = append(s.tracks, t)
	return s
}

// Tracks returns the song's tracks.
func (s *Song) Tracks() []*Track {
	return s.tracks
}

// Play compiles the song to the stream's voice manager and sequencer,
// registering all instruments and assigning track patterns to channels.
func (s *Song) Play(st *stream.Stream) error {
	vm := st.Voices()
	seq := st.Sequencer()

	// Set tempo.
	seq.SetBPM(s.bpm)

	// Register all instruments.
	if len(s.instruments) > 0 {
		vm.LoadBank("dsl_song", s.instruments)
	}

	// Compile and assign each track.
	for _, t := range s.tracks {
		if t.channel < 0 || t.channel >= 9 {
			return fmt.Errorf("track channel %d out of range (0-8); use multiple streams for more channels", t.channel)
		}

		seqPat, err := t.compile()
		if err != nil {
			return fmt.Errorf("track ch%d: %w", t.channel, err)
		}

		seq.SetPattern(t.channel, seqPat)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Track — a sequence of events on a single OPL2 channel
// ---------------------------------------------------------------------------

// Track represents a sequence of musical events on a single OPL2 channel.
// Events are placed on a tick-based timeline where each tick corresponds to
// one sequencer step (at 4 ticks per beat / 16th notes by default).
type Track struct {
	channel    int
	instrument string // default instrument name for this track
	events     []TrackEvent
	length     int // total length in ticks (0 = auto from last event)
}

// Phrase is a reusable block of track events with ticks relative to the phrase
// start.
type Phrase struct {
	instrument string
	override   *voice.InstrumentOverride
	events     []TrackEvent
	length     int
}

// NewPhrase creates an empty phrase.
func NewPhrase() *Phrase {
	return &Phrase{}
}

// AppendEvent adds a pre-built event to the phrase.
func (p *Phrase) AppendEvent(e TrackEvent) *Phrase {
	p.events = append(p.events, e)
	if e.Tick+1 > p.length {
		p.length = e.Tick + 1
	}
	return p
}

// SetLength sets the phrase length in ticks.
func (p *Phrase) SetLength(ticks int) *Phrase {
	p.length = ticks
	return p
}

// Length returns the phrase length in ticks.
func (p *Phrase) Length() int {
	return p.length
}

// Events returns the phrase events.
func (p *Phrase) Events() []TrackEvent {
	return p.events
}

// NewTrack creates a new track assigned to the given OPL2 channel (0-8).
func NewTrack(channel int) *Track {
	return &Track{channel: channel}
}

// SetInstrument sets the default instrument name for this track.
func (t *Track) SetInstrument(name string) *Track {
	t.instrument = name
	return t
}

// Instrument returns the default instrument name for this track.
func (t *Track) Instrument() string {
	return t.instrument
}

// SetLength sets the track length in ticks. If not set, the length is
// determined from the last event position plus its duration.
func (t *Track) SetLength(ticks int) *Track {
	t.length = ticks
	return t
}

// Channel returns the OPL2 channel this track is assigned to.
func (t *Track) Channel() int {
	return t.channel
}

// Length returns the configured track length in ticks.
func (t *Track) Length() int {
	return t.length
}

// Events returns the track's events (read-only access for converters).
func (t *Track) Events() []TrackEvent {
	return t.events
}

// AddEvent adds a pre-built event to the track. Used by converters that
// construct events programmatically.
func (t *Track) AddEvent(e TrackEvent) *Track {
	t.events = append(t.events, e)
	return t
}

// compile converts the track into a sequencer.Pattern.
func (t *Track) compile() (*sequencer.Pattern, error) {
	// Determine pattern length.
	length := t.length
	if length <= 0 {
		// Auto-detect from events.
		for _, e := range t.events {
			if e.Tick+1 > length {
				length = e.Tick + 1
			}
		}
	}
	if length <= 0 {
		length = 1 // minimum
	}

	pat := sequencer.NewPattern(length)
	if t.instrument != "" {
		pat.Instrument(t.instrument)
	}

	for _, e := range t.events {
		if e.err != nil {
			return nil, e.err
		}

		switch e.Type {
		case TrackNoteOn:
			inst := e.Instrument
			if inst == "" {
				inst = t.instrument
			}
			pat.Events = append(pat.Events, sequencer.Event{
				Step:       e.Tick,
				Type:       sequencer.NoteOn,
				Note:       e.Note,
				Instrument: inst,
				Override:   e.Override,
			})

		case TrackNoteOff:
			pat.Events = append(pat.Events, sequencer.Event{
				Step: e.Tick,
				Type: sequencer.NoteOff,
			})

		case TrackInstrumentChange:
			pat.Events = append(pat.Events, sequencer.Event{
				Step:       e.Tick,
				Type:       sequencer.InstrumentChange,
				Instrument: e.Instrument,
			})

		case TrackVolumeChange:
			pat.Events = append(pat.Events, sequencer.Event{
				Step:   e.Tick,
				Type:   sequencer.VolumeChange,
				Volume: e.Volume,
			})

		case TrackFrequencyChange:
			pat.Events = append(pat.Events, sequencer.Event{
				Step:      e.Tick,
				Type:      sequencer.FrequencyChange,
				Frequency: e.Frequency,
			})

		case TrackLevelChange:
			pat.Events = append(pat.Events, sequencer.Event{
				Step:     e.Tick,
				Type:     sequencer.LevelChange,
				Operator: e.Operator,
				Level:    e.Level,
			})

		case TrackFeedbackChange:
			pat.Events = append(pat.Events, sequencer.Event{
				Step:     e.Tick,
				Type:     sequencer.FeedbackChange,
				Feedback: e.Feedback,
			})
		}
	}

	return pat, nil
}

// ---------------------------------------------------------------------------
// TrackEvent — individual event on a track timeline
// ---------------------------------------------------------------------------

// TrackEventType identifies the kind of track event.
type TrackEventType int

const (
	// TrackNoteOn triggers a note.
	TrackNoteOn TrackEventType = iota
	// TrackNoteOff releases the current note.
	TrackNoteOff
	// TrackInstrumentChange switches the instrument on this channel.
	TrackInstrumentChange
	// TrackVolumeChange changes the channel volume.
	TrackVolumeChange
	// TrackFrequencyChange changes pitch without retriggering.
	TrackFrequencyChange
	// TrackLevelChange sets a specific operator's level.
	TrackLevelChange
	// TrackFeedbackChange sets the channel feedback amount.
	TrackFeedbackChange
)

// TrackEvent represents a single event on a track's timeline.
type TrackEvent struct {
	Tick       int            // Position in ticks from the start of the track.
	Type       TrackEventType // What kind of event.
	Note       voice.Note     // For NoteOn: the frequency.
	NoteStr    string         // For NoteOn: the original note string (for code gen).
	Instrument string         // For NoteOn/InstrumentChange: instrument name.
	Override   *voice.InstrumentOverride
	Volume     float64 // For VolumeChange: 0.0-1.0.
	Frequency  float64 // For FrequencyChange: Hz.
	Operator   int     // For LevelChange: 0=modulator, 1=carrier.
	Level      uint8   // For LevelChange: 0-63.
	Feedback   uint8   // For FeedbackChange: 0-7.
	err        error   // Deferred parse error.
}
