// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

// Package midi provides a minimal Standard MIDI File (SMF) parser.
//
// It supports Format 0 (single track) and Format 1 (multiple tracks,
// simultaneous) files. The parser extracts all channel voice messages
// (NoteOn, NoteOff, ProgramChange, ControlChange, PitchBend) and meta
// events (Tempo, EndOfTrack, TrackName) needed for FM synthesis playback.
package midi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// File represents a parsed Standard MIDI File.
type File struct {
	Format   uint16  // 0 = single track, 1 = multi-track simultaneous
	Division uint16  // Ticks per quarter note (if bit 15 is 0)
	Tracks   []Track // One or more tracks
}

// Track is a sequence of MIDI events with delta times.
type Track struct {
	Name   string  // From meta event 0x03 (track name)
	Events []Event // Time-ordered events
}

// Event is a single MIDI event with its absolute tick position.
type Event struct {
	Tick    uint32    // Absolute tick position from start of track
	Type    EventType // What kind of event
	Channel uint8     // MIDI channel (0-15)
	Data1   uint8     // First data byte (note number, program, CC number)
	Data2   uint8     // Second data byte (velocity, CC value)
	Tempo   uint32    // Microseconds per quarter note (for TempoChange events)
}

// EventType identifies the kind of MIDI event.
type EventType uint8

const (
	NoteOff       EventType = iota // Channel voice: note off
	NoteOn                         // Channel voice: note on
	ProgramChange                  // Channel voice: program change
	ControlChange                  // Channel voice: control change
	PitchBend                      // Channel voice: pitch bend
	TempoChange                    // Meta: tempo change
	EndOfTrack                     // Meta: end of track
)

// Parse reads a Standard MIDI File from the given reader.
func Parse(r io.Reader) (*File, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("midi: read error: %w", err)
	}
	return parseBytes(data)
}

func parseBytes(data []byte) (*File, error) {
	if len(data) < 14 {
		return nil, errors.New("midi: file too small")
	}

	// Parse header chunk: "MThd" + 4-byte length + 2-byte format + 2-byte ntracks + 2-byte division
	if string(data[0:4]) != "MThd" {
		return nil, errors.New("midi: invalid header magic")
	}

	headerLen := binary.BigEndian.Uint32(data[4:8])
	if headerLen < 6 {
		return nil, errors.New("midi: header too short")
	}

	format := binary.BigEndian.Uint16(data[8:10])
	nTracks := binary.BigEndian.Uint16(data[10:12])
	division := binary.BigEndian.Uint16(data[12:14])

	if format > 1 {
		return nil, fmt.Errorf("midi: unsupported format %d (only 0 and 1 are supported)", format)
	}

	// SMPTE time division is not supported.
	if division&0x8000 != 0 {
		return nil, errors.New("midi: SMPTE time division not supported")
	}

	f := &File{
		Format:   format,
		Division: division,
		Tracks:   make([]Track, 0, nTracks),
	}

	// Parse track chunks.
	offset := 8 + int(headerLen)
	for i := 0; i < int(nTracks); i++ {
		if offset+8 > len(data) {
			return nil, fmt.Errorf("midi: unexpected end of file at track %d", i)
		}

		if string(data[offset:offset+4]) != "MTrk" {
			return nil, fmt.Errorf("midi: invalid track %d magic", i)
		}

		trackLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		trackData := data[offset+8:]
		if len(trackData) < trackLen {
			return nil, fmt.Errorf("midi: track %d data truncated", i)
		}

		track, err := parseTrack(trackData[:trackLen])
		if err != nil {
			return nil, fmt.Errorf("midi: track %d: %w", i, err)
		}
		f.Tracks = append(f.Tracks, *track)

		offset += 8 + trackLen
	}

	return f, nil
}

// parseTrack parses a single track's event data.
func parseTrack(data []byte) (*Track, error) {
	t := &Track{}
	pos := 0
	var absTick uint32
	var runningStatus uint8

	for pos < len(data) {
		// Read delta time (variable-length quantity).
		delta, bytesRead, err := readVarLen(data[pos:])
		if err != nil {
			return nil, fmt.Errorf("delta time at offset %d: %w", pos, err)
		}
		pos += bytesRead
		absTick += delta

		if pos >= len(data) {
			return nil, errors.New("unexpected end of track data")
		}

		statusByte := data[pos]

		// Handle running status: if the high bit is not set, reuse the
		// previous status byte.
		if statusByte < 0x80 {
			if runningStatus == 0 {
				return nil, fmt.Errorf("running status without prior status at offset %d", pos)
			}
			statusByte = runningStatus
			// Don't advance pos — the current byte is data, not status.
		} else {
			pos++
			// Update running status for channel voice messages (0x80-0xEF).
			if statusByte >= 0x80 && statusByte < 0xF0 {
				runningStatus = statusByte
			} else {
				// System messages (0xF0-0xFF) clear running status.
				runningStatus = 0
			}
		}

		msgType := statusByte & 0xF0
		channel := statusByte & 0x0F

		switch {
		case msgType == 0x80: // Note Off
			if pos+2 > len(data) {
				return nil, errors.New("truncated note off")
			}
			t.Events = append(t.Events, Event{
				Tick:    absTick,
				Type:    NoteOff,
				Channel: channel,
				Data1:   data[pos],
				Data2:   data[pos+1],
			})
			pos += 2

		case msgType == 0x90: // Note On
			if pos+2 > len(data) {
				return nil, errors.New("truncated note on")
			}
			evType := NoteOn
			// Velocity 0 is equivalent to Note Off.
			if data[pos+1] == 0 {
				evType = NoteOff
			}
			t.Events = append(t.Events, Event{
				Tick:    absTick,
				Type:    evType,
				Channel: channel,
				Data1:   data[pos],
				Data2:   data[pos+1],
			})
			pos += 2

		case msgType == 0xA0: // Polyphonic Aftertouch — skip
			pos += 2

		case msgType == 0xB0: // Control Change
			if pos+2 > len(data) {
				return nil, errors.New("truncated control change")
			}
			t.Events = append(t.Events, Event{
				Tick:    absTick,
				Type:    ControlChange,
				Channel: channel,
				Data1:   data[pos],
				Data2:   data[pos+1],
			})
			pos += 2

		case msgType == 0xC0: // Program Change
			if pos+1 > len(data) {
				return nil, errors.New("truncated program change")
			}
			t.Events = append(t.Events, Event{
				Tick:    absTick,
				Type:    ProgramChange,
				Channel: channel,
				Data1:   data[pos],
			})
			pos += 1

		case msgType == 0xD0: // Channel Aftertouch — skip
			pos += 1

		case msgType == 0xE0: // Pitch Bend
			if pos+2 > len(data) {
				return nil, errors.New("truncated pitch bend")
			}
			t.Events = append(t.Events, Event{
				Tick:    absTick,
				Type:    PitchBend,
				Channel: channel,
				Data1:   data[pos],
				Data2:   data[pos+1],
			})
			pos += 2

		case statusByte == 0xFF: // Meta event
			if pos+1 > len(data) {
				return nil, errors.New("truncated meta event")
			}
			metaType := data[pos]
			pos++
			metaLen, bytesRead, err := readVarLen(data[pos:])
			if err != nil {
				return nil, fmt.Errorf("meta event length: %w", err)
			}
			pos += bytesRead

			if pos+int(metaLen) > len(data) {
				return nil, errors.New("truncated meta event data")
			}
			metaData := data[pos : pos+int(metaLen)]
			pos += int(metaLen)

			switch metaType {
			case 0x03: // Track Name
				t.Name = string(metaData)
			case 0x51: // Set Tempo
				if len(metaData) == 3 {
					tempo := uint32(metaData[0])<<16 | uint32(metaData[1])<<8 | uint32(metaData[2])
					t.Events = append(t.Events, Event{
						Tick:  absTick,
						Type:  TempoChange,
						Tempo: tempo,
					})
				}
			case 0x2F: // End of Track
				t.Events = append(t.Events, Event{
					Tick: absTick,
					Type: EndOfTrack,
				})
				return t, nil
			}
			// All other meta events are skipped.

		case statusByte == 0xF0 || statusByte == 0xF7: // SysEx
			sysexLen, bytesRead, err := readVarLen(data[pos:])
			if err != nil {
				return nil, fmt.Errorf("sysex length: %w", err)
			}
			pos += bytesRead + int(sysexLen)

		default:
			// Unknown status byte — skip.
			return nil, fmt.Errorf("unknown status byte 0x%02X at offset %d", statusByte, pos)
		}
	}

	return t, nil
}

// readVarLen reads a MIDI variable-length quantity.
func readVarLen(data []byte) (value uint32, bytesRead int, err error) {
	if len(data) == 0 {
		return 0, 0, errors.New("empty data for variable-length quantity")
	}

	for i := 0; i < 4 && i < len(data); i++ {
		value = (value << 7) | uint32(data[i]&0x7F)
		bytesRead = i + 1
		if data[i]&0x80 == 0 {
			return value, bytesRead, nil
		}
	}

	return 0, 0, errors.New("variable-length quantity exceeds 4 bytes")
}

// TotalTicks returns the maximum tick count across all tracks.
func (f *File) TotalTicks() uint32 {
	var maxTick uint32
	for _, track := range f.Tracks {
		for _, ev := range track.Events {
			if ev.Tick > maxTick {
				maxTick = ev.Tick
			}
		}
	}
	return maxTick
}

// Duration returns the approximate duration in seconds based on the initial
// tempo. If no tempo event is found, 120 BPM is assumed.
func (f *File) Duration() float64 {
	// Find the initial tempo.
	usPerBeat := uint32(500000) // default 120 BPM
	for _, track := range f.Tracks {
		for _, ev := range track.Events {
			if ev.Type == TempoChange {
				usPerBeat = ev.Tempo
				break
			}
		}
		if usPerBeat != 500000 {
			break
		}
	}

	totalTicks := f.TotalTicks()
	if f.Division == 0 {
		return 0
	}

	// Simple approximation using the first tempo only.
	secondsPerTick := float64(usPerBeat) / (float64(f.Division) * 1_000_000.0)
	return float64(totalTicks) * secondsPerTick
}
