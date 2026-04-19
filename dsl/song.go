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
			return fmt.Errorf("track channel %d out of range (0-8)", t.channel)
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

// NoteOn adds a note-on event at the given tick position.
func (t *Track) NoteOn(tick int, note string) *Track {
	return t.NoteOnWithOverride(tick, note, nil)
}

// NoteOnWithOverride adds a note-on event with optional note-start instrument
// overrides applied against the current instrument.
func (t *Track) NoteOnWithOverride(tick int, note string, override *voice.InstrumentOverride) *Track {
	t.events = append(t.events, buildNoteOnEvent(tick, note, t.instrument, override))
	return t
}

// NoteOff adds a note-off event at the given tick position.
func (t *Track) NoteOff(tick int) *Track {
	t.events = append(t.events, TrackEvent{
		Tick: tick,
		Type: TrackNoteOff,
	})
	return t
}

// NoteOnOff adds a note-on at `tick` and a note-off at `tick + duration`.
// This is a convenience for notes with explicit durations.
func (t *Track) NoteOnOff(tick int, note string, duration int) *Track {
	t.NoteOn(tick, note)
	t.NoteOff(tick + duration)
	return t
}

// NoteOnOffWithOverride adds a note-on plus note-off pair with optional
// note-start instrument overrides.
func (t *Track) NoteOnOffWithOverride(tick int, note string, duration int, override *voice.InstrumentOverride) *Track {
	t.NoteOnWithOverride(tick, note, override)
	t.NoteOff(tick + duration)
	return t
}

// Rest is a no-op marker for readability. It doesn't produce any events
// since rests are implicitly represented by gaps between notes.
func (t *Track) Rest(_ int, _ int) *Track {
	return t
}

// SetInstrumentAt adds an instrument-change event at the given tick.
func (t *Track) SetInstrumentAt(tick int, name string) *Track {
	t.events = append(t.events, TrackEvent{
		Tick:       tick,
		Type:       TrackInstrumentChange,
		Instrument: name,
	})
	return t
}

// SetVolumeAt adds a volume-change event at the given tick.
// Volume is 0.0 (silent) to 1.0 (loudest).
func (t *Track) SetVolumeAt(tick int, vol float64) *Track {
	t.events = append(t.events, TrackEvent{
		Tick:   tick,
		Type:   TrackVolumeChange,
		Volume: vol,
	})
	return t
}

// SetFrequencyAt adds a frequency-change event at the given tick.
// This changes the pitch without retriggering the note.
func (t *Track) SetFrequencyAt(tick int, freqHz float64) *Track {
	t.events = append(t.events, TrackEvent{
		Tick:      tick,
		Type:      TrackFrequencyChange,
		Frequency: freqHz,
	})
	return t
}

// SetLevelAt adds an operator level-change event at the given tick.
// op: 0=modulator, 1=carrier. level: 0(loudest)-63(silent).
func (t *Track) SetLevelAt(tick int, op int, level uint8) *Track {
	t.events = append(t.events, TrackEvent{
		Tick:     tick,
		Type:     TrackLevelChange,
		Operator: op,
		Level:    level,
	})
	return t
}

// SetFeedbackAt adds a feedback-change event at the given tick.
func (t *Track) SetFeedbackAt(tick int, fb uint8) *Track {
	t.events = append(t.events, TrackEvent{
		Tick:     tick,
		Type:     TrackFeedbackChange,
		Feedback: fb,
	})
	return t
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

// Append adds a phrase at the given track tick offset.
func (t *Track) Append(p *Phrase, at int) *Track {
	if p == nil {
		return t
	}
	for _, e := range p.events {
		shifted := e
		shifted.Tick += at
		t.events = append(t.events, shifted)
	}
	if extent := phraseExtent(p); extent > 0 && at+extent > t.length {
		t.length = at + extent
	}
	return t
}

// Repeat appends the same phrase multiple times with a fixed spacing.
func (t *Track) Repeat(p *Phrase, count int, spacing int) *Track {
	if p == nil || count <= 0 {
		return t
	}
	for i := 0; i < count; i++ {
		t.Append(p, i*spacing)
	}
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

// ---------------------------------------------------------------------------
// Stack — simultaneous layers (Tier 8)
// ---------------------------------------------------------------------------

// Stack creates a Song that plays multiple patterns simultaneously, each on
// its own channel. This is the Strudel-style Stack() combinator.
//
// Usage:
//
//	song := dsl.Stack(bassPattern, leadPattern, padPattern)
//	song.Play(stream)
func Stack(patterns ...*Pattern) *Song {
	song := NewSong(120) // default BPM
	for i, p := range patterns {
		if i >= 9 {
			break // OPL2 max channels
		}
		track := patternToTrack(p, i)
		song.AddTrack(track)
	}
	return song
}

// Seq creates a Song that plays patterns sequentially on channel 0.
// Each pattern occupies one "cycle" (pattern length).
//
// Usage:
//
//	song := dsl.Seq(intro, verse, chorus)
//	song.Play(stream)
func Seq(patterns ...*Pattern) *Song {
	song := NewSong(120) // default BPM
	track := NewTrack(0)

	offset := 0
	defaultLen := 16 // default pattern length in ticks

	for _, p := range patterns {
		noteStr := p.noteStr
		if !p.noteSet {
			noteStr = "C4"
		}

		instName := p.sound
		if instName == "" {
			instName = "dsl_default"
		}

		track.SetInstrumentAt(offset, instName)
		track.NoteOnOff(offset, noteStr, defaultLen)
		offset += defaultLen
	}

	track.SetLength(offset)
	song.AddTrack(track)
	return song
}

// patternToTrack converts a single-event Pattern into a Track for use in
// Stack/Seq composition. The pattern's instrument is registered and a
// NoteOn event is created at tick 0.
func patternToTrack(p *Pattern, channel int) *Track {
	track := NewTrack(channel)

	// Resolve instrument name.
	instName := p.sound
	if instName == "" {
		instName = "dsl_default"
	}
	track.SetInstrument(instName)

	// Add the note event.
	noteStr := p.noteStr
	if !p.noteSet {
		noteStr = "C4"
	}
	track.NoteOn(0, noteStr)

	return track
}

// Silence creates a Pattern that produces no sound. Useful as a placeholder
// in Seq() or Stack() calls.
func Silence() *Pattern {
	return &Pattern{}
}
