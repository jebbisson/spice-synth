// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package dsl

import (
	"fmt"

	"github.com/jebbisson/spice-synth/voice"
)

// SetInstrument sets the default instrument used by note builders in the phrase.
func (p *Phrase) SetInstrument(name string) *Phrase {
	p.instrument = name
	return p
}

// Instrument returns the phrase's default note-builder instrument.
func (p *Phrase) Instrument() string {
	return p.instrument
}

// SetOverride sets the default note-start override used by note builders.
func (p *Phrase) SetOverride(override *voice.InstrumentOverride) *Phrase {
	p.override = cloneInstrumentOverride(override)
	return p
}

// Override returns a clone of the phrase's default note-start override.
func (p *Phrase) Override() *voice.InstrumentOverride {
	return cloneInstrumentOverride(p.override)
}

// AddEvent appends a pre-built event to the phrase.
func (p *Phrase) AddEvent(e TrackEvent) *Phrase {
	return p.AppendEvent(e)
}

// Cursor creates a builder cursor positioned at tick 0.
func (p *Phrase) Cursor() *PhraseCursor {
	return p.At(0)
}

// At creates a builder cursor positioned at the given phrase-relative tick.
func (p *Phrase) At(tick int) *PhraseCursor {
	if p == nil {
		return nil
	}
	if tick < 0 {
		tick = 0
	}
	p.extendLength(tick)
	return &PhraseCursor{
		phrase:     p,
		tick:       tick,
		instrument: p.instrument,
		override:   cloneInstrumentOverride(p.override),
	}
}

func (p *Phrase) extendLength(end int) {
	if p == nil {
		return
	}
	if end > p.length {
		p.length = end
	}
}

func phraseEventExtent(p *Phrase) int {
	if p == nil {
		return 0
	}
	extent := 0
	for _, e := range p.events {
		if e.Tick+1 > extent {
			extent = e.Tick + 1
		}
	}
	return extent
}

func phraseSpan(p *Phrase) int {
	if p == nil {
		return 0
	}
	if p.length > 0 {
		return p.length
	}
	return phraseEventExtent(p)
}

func phraseExtent(p *Phrase) int {
	span := phraseSpan(p)
	extent := phraseEventExtent(p)
	if extent > span {
		return extent
	}
	return span
}

// PhraseCursor incrementally builds phrase-relative events using a moving tick.
type PhraseCursor struct {
	phrase     *Phrase
	tick       int
	instrument string
	override   *voice.InstrumentOverride
}

// Tick returns the cursor's current phrase-relative tick.
func (c *PhraseCursor) Tick() int {
	if c == nil {
		return 0
	}
	return c.tick
}

// Advance moves the cursor forward by the given number of ticks.
func (c *PhraseCursor) Advance(ticks int) *PhraseCursor {
	if c == nil || ticks <= 0 {
		return c
	}
	c.tick += ticks
	c.phrase.extendLength(c.tick)
	return c
}

// AdvanceTo moves the cursor to the given phrase-relative tick.
func (c *PhraseCursor) AdvanceTo(tick int) *PhraseCursor {
	if c == nil || tick <= c.tick {
		return c
	}
	return c.Advance(tick - c.tick)
}

// Rest is an alias for Advance.
func (c *PhraseCursor) Rest(ticks int) *PhraseCursor {
	return c.Advance(ticks)
}

// UseInstrument changes the builder's default instrument for subsequent notes.
func (c *PhraseCursor) UseInstrument(name string) *PhraseCursor {
	if c == nil {
		return c
	}
	c.instrument = name
	return c
}

// ClearInstrument clears the builder's default note instrument.
func (c *PhraseCursor) ClearInstrument() *PhraseCursor {
	return c.UseInstrument("")
}

// UseOverride changes the builder's default note-start override.
func (c *PhraseCursor) UseOverride(override *voice.InstrumentOverride) *PhraseCursor {
	if c == nil {
		return c
	}
	c.override = cloneInstrumentOverride(override)
	return c
}

// ClearOverride clears the builder's default note-start override.
func (c *PhraseCursor) ClearOverride() *PhraseCursor {
	return c.UseOverride(nil)
}

// ChangeInstrument emits an instrument-change event at the current cursor tick.
func (c *PhraseCursor) ChangeInstrument(name string) *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, TrackEvent{
		Tick:       c.tick,
		Type:       TrackInstrumentChange,
		Instrument: name,
	})
	c.instrument = name
	return c
}

// On emits a note-on event at the current cursor tick.
func (c *PhraseCursor) On(note string) *PhraseCursor {
	return c.OnWithOverride(note, nil)
}

// OnWithOverride emits a note-on with an optional per-note override.
func (c *PhraseCursor) OnWithOverride(note string, override *voice.InstrumentOverride) *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, buildNoteOnEvent(c.tick, note, c.instrument, mergeInstrumentOverrides(c.override, override)))
	c.phrase.extendLength(c.tick)
	return c
}

// Note emits a note-on followed by note-off after the given duration.
func (c *PhraseCursor) Note(note string, duration int) *PhraseCursor {
	return c.NoteWithOverride(note, duration, nil)
}

// NoteWithOverride emits a duration note with an optional per-note override.
func (c *PhraseCursor) NoteWithOverride(note string, duration int, override *voice.InstrumentOverride) *PhraseCursor {
	if c == nil {
		return c
	}
	start := c.tick
	c.phrase.events = append(c.phrase.events, buildNoteOnEvent(start, note, c.instrument, mergeInstrumentOverrides(c.override, override)))
	c.phrase.events = append(c.phrase.events, TrackEvent{Tick: start + duration, Type: TrackNoteOff})
	c.tick += duration
	c.phrase.extendLength(c.tick)
	return c
}

// Pulse emits a note-on and advances the cursor without adding a note-off.
func (c *PhraseCursor) Pulse(note string, duration int) *PhraseCursor {
	return c.PulseWithOverride(note, duration, nil)
}

// PulseWithOverride emits a retrigger-style note-on and advances the cursor.
func (c *PhraseCursor) PulseWithOverride(note string, duration int, override *voice.InstrumentOverride) *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, buildNoteOnEvent(c.tick, note, c.instrument, mergeInstrumentOverrides(c.override, override)))
	c.tick += duration
	c.phrase.extendLength(c.tick)
	return c
}

// Off emits a note-off at the current cursor tick.
func (c *PhraseCursor) Off() *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, TrackEvent{Tick: c.tick, Type: TrackNoteOff})
	return c
}

// Volume emits a volume change at the current cursor tick.
func (c *PhraseCursor) Volume(vol float64) *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, TrackEvent{Tick: c.tick, Type: TrackVolumeChange, Volume: vol})
	return c
}

// Frequency emits a frequency change at the current cursor tick.
func (c *PhraseCursor) Frequency(freqHz float64) *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, TrackEvent{Tick: c.tick, Type: TrackFrequencyChange, Frequency: freqHz})
	return c
}

// Level emits an operator level change at the current cursor tick.
func (c *PhraseCursor) Level(op int, level uint8) *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, TrackEvent{Tick: c.tick, Type: TrackLevelChange, Operator: op, Level: level})
	return c
}

// Feedback emits a feedback change at the current cursor tick.
func (c *PhraseCursor) Feedback(fb uint8) *PhraseCursor {
	if c == nil {
		return c
	}
	c.phrase.events = append(c.phrase.events, TrackEvent{Tick: c.tick, Type: TrackFeedbackChange, Feedback: fb})
	return c
}

// Event appends a raw event shifted by the cursor's current tick.
func (c *PhraseCursor) Event(e TrackEvent) *PhraseCursor {
	if c == nil {
		return c
	}
	shifted := e
	shifted.Tick += c.tick
	c.phrase.AppendEvent(shifted)
	return c
}

// Cursor creates a track builder cursor positioned at tick 0.
func (t *Track) Cursor() *TrackCursor {
	return t.At(0)
}

// At creates a track builder cursor positioned at the given track tick.
func (t *Track) At(tick int) *TrackCursor {
	if t == nil {
		return nil
	}
	if tick < 0 {
		tick = 0
	}
	if tick > t.length {
		t.length = tick
	}
	return &TrackCursor{
		track:      t,
		tick:       tick,
		instrument: t.instrument,
	}
}

type trackCursorRestore struct {
	instrument string
	override   *voice.InstrumentOverride
}

type trackRepeatState struct {
	body *Phrase
}

// TrackCursor incrementally builds a track timeline. It can also represent
// temporary block scopes such as repeat bodies and scoped instrument/override
// changes.
type TrackCursor struct {
	track      *Track
	phrase     *Phrase
	tick       int
	instrument string
	override   *voice.InstrumentOverride

	parent  *TrackCursor
	restore *trackCursorRestore
	repeat  *trackRepeatState
}

// Tick returns the cursor's current tick within its current scope.
func (c *TrackCursor) Tick() int {
	if c == nil {
		return 0
	}
	return c.tick
}

// Advance moves the cursor forward by the given number of ticks.
func (c *TrackCursor) Advance(ticks int) *TrackCursor {
	if c == nil || ticks <= 0 {
		return c
	}
	c.tick += ticks
	c.extendSpan(c.tick)
	return c
}

// AdvanceTo moves the cursor to the given tick within its current scope.
func (c *TrackCursor) AdvanceTo(tick int) *TrackCursor {
	if c == nil || tick <= c.tick {
		return c
	}
	return c.Advance(tick - c.tick)
}

// Rest is an alias for Advance.
func (c *TrackCursor) Rest(ticks int) *TrackCursor {
	return c.Advance(ticks)
}

// UseInstrument changes the default note instrument for subsequent notes.
func (c *TrackCursor) UseInstrument(name string) *TrackCursor {
	if c == nil {
		return c
	}
	c.instrument = name
	return c
}

// ClearInstrument clears the default note instrument.
func (c *TrackCursor) ClearInstrument() *TrackCursor {
	return c.UseInstrument("")
}

// UseOverride changes the default note-start override.
func (c *TrackCursor) UseOverride(override *voice.InstrumentOverride) *TrackCursor {
	if c == nil {
		return c
	}
	c.override = cloneInstrumentOverride(override)
	return c
}

// ClearOverride clears the default note-start override.
func (c *TrackCursor) ClearOverride() *TrackCursor {
	return c.UseOverride(nil)
}

// ChangeInstrument emits an instrument-change event at the current tick.
func (c *TrackCursor) ChangeInstrument(name string) *TrackCursor {
	if c == nil {
		return c
	}
	e := TrackEvent{Tick: c.tick, Type: TrackInstrumentChange, Instrument: name}
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
	} else {
		c.appendTrackEvent(e)
	}
	c.instrument = name
	return c
}

// On emits a note-on at the current tick.
func (c *TrackCursor) On(note string) *TrackCursor {
	return c.OnWithOverride(note, nil)
}

// OnWithOverride emits a note-on with an optional per-note override.
func (c *TrackCursor) OnWithOverride(note string, override *voice.InstrumentOverride) *TrackCursor {
	if c == nil {
		return c
	}
	e := buildNoteOnEvent(c.tick, note, c.instrument, mergeInstrumentOverrides(c.override, override))
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
		c.phrase.extendLength(c.tick)
	} else {
		c.appendTrackEvent(e)
	}
	return c
}

// Note emits a note-on followed by note-off after the given duration.
func (c *TrackCursor) Note(note string, duration int) *TrackCursor {
	return c.NoteWithOverride(note, duration, nil)
}

// NoteWithOverride emits a duration note with an optional per-note override.
func (c *TrackCursor) NoteWithOverride(note string, duration int, override *voice.InstrumentOverride) *TrackCursor {
	if c == nil {
		return c
	}
	start := c.tick
	noteOn := buildNoteOnEvent(start, note, c.instrument, mergeInstrumentOverrides(c.override, override))
	noteOff := TrackEvent{Tick: start + duration, Type: TrackNoteOff}
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, noteOn, noteOff)
		c.tick += duration
		c.phrase.extendLength(c.tick)
		return c
	}
	c.appendTrackEvent(noteOn)
	c.appendTrackEvent(noteOff)
	c.tick += duration
	c.extendSpan(c.tick)
	return c
}

// Pulse emits a note-on and advances the cursor without adding a note-off.
func (c *TrackCursor) Pulse(note string, duration int) *TrackCursor {
	return c.PulseWithOverride(note, duration, nil)
}

// PulseWithOverride emits a retrigger-style note-on and advances the cursor.
func (c *TrackCursor) PulseWithOverride(note string, duration int, override *voice.InstrumentOverride) *TrackCursor {
	if c == nil {
		return c
	}
	e := buildNoteOnEvent(c.tick, note, c.instrument, mergeInstrumentOverrides(c.override, override))
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
		c.tick += duration
		c.phrase.extendLength(c.tick)
		return c
	}
	c.appendTrackEvent(e)
	c.tick += duration
	c.extendSpan(c.tick)
	return c
}

// Off emits a note-off at the current tick.
func (c *TrackCursor) Off() *TrackCursor {
	if c == nil {
		return c
	}
	e := TrackEvent{Tick: c.tick, Type: TrackNoteOff}
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
	} else {
		c.appendTrackEvent(e)
	}
	return c
}

// Volume emits a volume change at the current tick.
func (c *TrackCursor) Volume(vol float64) *TrackCursor {
	if c == nil {
		return c
	}
	e := TrackEvent{Tick: c.tick, Type: TrackVolumeChange, Volume: vol}
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
	} else {
		c.appendTrackEvent(e)
	}
	return c
}

// Frequency emits a frequency change at the current tick.
func (c *TrackCursor) Frequency(freqHz float64) *TrackCursor {
	if c == nil {
		return c
	}
	e := TrackEvent{Tick: c.tick, Type: TrackFrequencyChange, Frequency: freqHz}
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
	} else {
		c.appendTrackEvent(e)
	}
	return c
}

// Level emits an operator level change at the current tick.
func (c *TrackCursor) Level(op int, level uint8) *TrackCursor {
	if c == nil {
		return c
	}
	e := TrackEvent{Tick: c.tick, Type: TrackLevelChange, Operator: op, Level: level}
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
	} else {
		c.appendTrackEvent(e)
	}
	return c
}

// Feedback emits a feedback change at the current tick.
func (c *TrackCursor) Feedback(fb uint8) *TrackCursor {
	if c == nil {
		return c
	}
	e := TrackEvent{Tick: c.tick, Type: TrackFeedbackChange, Feedback: fb}
	if c.phrase != nil {
		c.phrase.events = append(c.phrase.events, e)
	} else {
		c.appendTrackEvent(e)
	}
	return c
}

// Event appends a raw event shifted by the cursor's current tick.
func (c *TrackCursor) Event(e TrackEvent) *TrackCursor {
	if c == nil {
		return c
	}
	shifted := e
	shifted.Tick += c.tick
	if c.phrase != nil {
		c.phrase.AppendEvent(shifted)
	} else {
		c.appendTrackEvent(shifted)
	}
	return c
}

// Append appends a phrase at the current cursor tick and advances by its span.
func (c *TrackCursor) Append(p *Phrase) *TrackCursor {
	if c == nil || p == nil {
		return c
	}
	c.appendPhraseAt(p, c.tick)
	c.tick += phraseSpan(p)
	c.extendSpan(c.tick)
	return c
}

// Repeat appends the same phrase count times using the phrase span as spacing.
func (c *TrackCursor) Repeat(p *Phrase, count int) *TrackCursor {
	if c == nil || p == nil || count <= 0 {
		return c
	}
	span := phraseSpan(p)
	if span <= 0 {
		return c
	}
	start := c.tick
	for i := 0; i < count; i++ {
		c.appendPhraseAt(p, start+i*span)
	}
	c.tick = start + count*span
	c.extendSpan(c.tick)
	return c
}

// RepeatUntil appends whole-phrase repeats until the cursor reaches endTick.
// Partial phrase repeats are intentionally skipped so explicit tails can follow.
func (c *TrackCursor) RepeatUntil(p *Phrase, endTick int) *TrackCursor {
	if c == nil || p == nil || endTick <= c.tick {
		return c
	}
	span := phraseSpan(p)
	if span <= 0 {
		return c
	}
	count := (endTick - c.tick) / span
	if count <= 0 {
		return c
	}
	return c.Repeat(p, count)
}

// WithRepeat opens a repeat block whose body is collected and later expanded by
// Until or Times.
func (c *TrackCursor) WithRepeat() *TrackCursor {
	if c == nil {
		return nil
	}
	body := NewPhrase().SetInstrument(c.instrument).SetOverride(c.override)
	return &TrackCursor{
		phrase:     body,
		instrument: c.instrument,
		override:   cloneInstrumentOverride(c.override),
		parent:     c,
		repeat:     &trackRepeatState{body: body},
	}
}

// WithOverride opens a scoped override block that restores the previous state
// when End is called.
func (c *TrackCursor) WithOverride(override *voice.InstrumentOverride) *TrackCursor {
	if c == nil {
		return nil
	}
	return &TrackCursor{
		track:      c.track,
		phrase:     c.phrase,
		tick:       c.tick,
		instrument: c.instrument,
		override:   cloneInstrumentOverride(override),
		parent:     c,
		restore: &trackCursorRestore{
			instrument: c.instrument,
			override:   cloneInstrumentOverride(c.override),
		},
	}
}

// WithInstrument opens a scoped instrument block that restores the previous
// state when End is called.
func (c *TrackCursor) WithInstrument(name string) *TrackCursor {
	if c == nil {
		return nil
	}
	return &TrackCursor{
		track:      c.track,
		phrase:     c.phrase,
		tick:       c.tick,
		instrument: name,
		override:   cloneInstrumentOverride(c.override),
		parent:     c,
		restore: &trackCursorRestore{
			instrument: c.instrument,
			override:   cloneInstrumentOverride(c.override),
		},
	}
}

// End closes a scoped instrument/override block and restores the parent state.
func (c *TrackCursor) End() *TrackCursor {
	if c == nil || c.parent == nil {
		return c
	}
	if c.repeat != nil {
		return c.Times(1)
	}
	parent := c.parent
	parent.tick = c.tick
	if c.restore != nil {
		parent.instrument = c.restore.instrument
		parent.override = cloneInstrumentOverride(c.restore.override)
	} else {
		parent.instrument = c.instrument
		parent.override = cloneInstrumentOverride(c.override)
	}
	parent.extendSpan(parent.tick)
	return parent
}

// Times repeats the block body the specified number of times and returns the
// parent cursor.
func (c *TrackCursor) Times(count int) *TrackCursor {
	if c == nil || c.parent == nil || c.repeat == nil {
		return c
	}
	parent := c.parent
	body := c.repeat.body
	span := phraseSpan(body)
	if count > 0 && span > 0 {
		start := parent.tick
		for i := 0; i < count; i++ {
			parent.appendPhraseAt(body, start+i*span)
		}
		parent.tick = start + count*span
		parent.extendSpan(parent.tick)
	}
	parent.instrument = c.instrument
	parent.override = cloneInstrumentOverride(c.override)
	return parent
}

// Until repeats the block body until the parent cursor reaches endTick.
// Partial repeats are intentionally skipped.
func (c *TrackCursor) Until(endTick int) *TrackCursor {
	if c == nil || c.parent == nil || c.repeat == nil {
		return c
	}
	span := phraseSpan(c.repeat.body)
	if span <= 0 || endTick <= c.parent.tick {
		parent := c.parent
		parent.instrument = c.instrument
		parent.override = cloneInstrumentOverride(c.override)
		return parent
	}
	count := (endTick - c.parent.tick) / span
	return c.Times(count)
}

func (c *TrackCursor) appendPhraseAt(p *Phrase, at int) {
	if c == nil || p == nil {
		return
	}
	if c.phrase != nil {
		for _, e := range p.events {
			shifted := e
			shifted.Tick += at
			c.phrase.events = append(c.phrase.events, shifted)
		}
		c.phrase.extendLength(at + phraseExtent(p))
		return
	}
	c.track.Append(p, at)
}

func (c *TrackCursor) appendTrackEvent(e TrackEvent) {
	if c == nil || c.track == nil {
		return
	}
	c.track.events = append(c.track.events, e)
	if e.Tick+1 > c.track.length {
		c.track.length = e.Tick + 1
	}
}

func (c *TrackCursor) extendSpan(tick int) {
	if c == nil {
		return
	}
	if c.phrase != nil {
		c.phrase.extendLength(tick)
		return
	}
	if c.track != nil && tick > c.track.length {
		c.track.length = tick
	}
}

func buildNoteOnEvent(tick int, note string, instrument string, override *voice.InstrumentOverride) TrackEvent {
	freq, err := voice.ParseNote(note)
	if err != nil {
		return TrackEvent{
			Tick: tick,
			Type: TrackNoteOn,
			err:  fmt.Errorf("invalid note %q: %w", note, err),
		}
	}
	return TrackEvent{
		Tick:       tick,
		Type:       TrackNoteOn,
		Note:       voice.Note(freq),
		NoteStr:    note,
		Instrument: instrument,
		Override:   cloneInstrumentOverride(override),
	}
}

func mergeInstrumentOverrides(base, extra *voice.InstrumentOverride) *voice.InstrumentOverride {
	if base == nil || base.Empty() {
		return cloneInstrumentOverride(extra)
	}
	if extra == nil || extra.Empty() {
		return cloneInstrumentOverride(base)
	}
	merged := cloneInstrumentOverride(base)
	mergeOperatorOverride(&merged.Op1, extra.Op1)
	mergeOperatorOverride(&merged.Op2, extra.Op2)
	if extra.Feedback != nil {
		value := *extra.Feedback
		merged.Feedback = &value
	}
	if extra.Connection != nil {
		value := *extra.Connection
		merged.Connection = &value
	}
	if merged.Empty() {
		return nil
	}
	return merged
}

func mergeOperatorOverride(dst *voice.OperatorOverride, src voice.OperatorOverride) {
	if dst == nil {
		return
	}
	if src.Attack != nil {
		value := *src.Attack
		dst.Attack = &value
	}
	if src.Decay != nil {
		value := *src.Decay
		dst.Decay = &value
	}
	if src.Sustain != nil {
		value := *src.Sustain
		dst.Sustain = &value
	}
	if src.Release != nil {
		value := *src.Release
		dst.Release = &value
	}
	if src.Level != nil {
		value := *src.Level
		dst.Level = &value
	}
	if src.Multiply != nil {
		value := *src.Multiply
		dst.Multiply = &value
	}
	if src.KeyScaleRate != nil {
		value := *src.KeyScaleRate
		dst.KeyScaleRate = &value
	}
	if src.KeyScaleLevel != nil {
		value := *src.KeyScaleLevel
		dst.KeyScaleLevel = &value
	}
	if src.Tremolo != nil {
		value := *src.Tremolo
		dst.Tremolo = &value
	}
	if src.Vibrato != nil {
		value := *src.Vibrato
		dst.Vibrato = &value
	}
	if src.Sustaining != nil {
		value := *src.Sustaining
		dst.Sustaining = &value
	}
	if src.Waveform != nil {
		value := *src.Waveform
		dst.Waveform = &value
	}
}

func cloneInstrumentOverride(override *voice.InstrumentOverride) *voice.InstrumentOverride {
	if override == nil || override.Empty() {
		return nil
	}
	cloned := &voice.InstrumentOverride{
		Op1: cloneOperatorOverride(override.Op1),
		Op2: cloneOperatorOverride(override.Op2),
	}
	if override.Feedback != nil {
		value := *override.Feedback
		cloned.Feedback = &value
	}
	if override.Connection != nil {
		value := *override.Connection
		cloned.Connection = &value
	}
	if cloned.Empty() {
		return nil
	}
	return cloned
}

func cloneOperatorOverride(override voice.OperatorOverride) voice.OperatorOverride {
	cloned := voice.OperatorOverride{}
	if override.Attack != nil {
		value := *override.Attack
		cloned.Attack = &value
	}
	if override.Decay != nil {
		value := *override.Decay
		cloned.Decay = &value
	}
	if override.Sustain != nil {
		value := *override.Sustain
		cloned.Sustain = &value
	}
	if override.Release != nil {
		value := *override.Release
		cloned.Release = &value
	}
	if override.Level != nil {
		value := *override.Level
		cloned.Level = &value
	}
	if override.Multiply != nil {
		value := *override.Multiply
		cloned.Multiply = &value
	}
	if override.KeyScaleRate != nil {
		value := *override.KeyScaleRate
		cloned.KeyScaleRate = &value
	}
	if override.KeyScaleLevel != nil {
		value := *override.KeyScaleLevel
		cloned.KeyScaleLevel = &value
	}
	if override.Tremolo != nil {
		value := *override.Tremolo
		cloned.Tremolo = &value
	}
	if override.Vibrato != nil {
		value := *override.Vibrato
		cloned.Vibrato = &value
	}
	if override.Sustaining != nil {
		value := *override.Sustaining
		cloned.Sustaining = &value
	}
	if override.Waveform != nil {
		value := *override.Waveform
		cloned.Waveform = &value
	}
	return cloned
}
