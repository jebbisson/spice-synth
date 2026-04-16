// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package midi

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func TestParseRealFile(t *testing.T) {
	f, err := os.Open("../examples/midi/Title.mid")
	if err != nil {
		t.Skipf("test MIDI file not found: %v", err)
	}
	defer f.Close()

	mf, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	t.Logf("format: %d, division: %d, tracks: %d", mf.Format, mf.Division, len(mf.Tracks))

	if len(mf.Tracks) == 0 {
		t.Fatal("no tracks parsed")
	}

	totalEvents := 0
	for i, track := range mf.Tracks {
		noteOns := 0
		noteOffs := 0
		progChanges := 0
		tempoChanges := 0
		for _, ev := range track.Events {
			switch ev.Type {
			case NoteOn:
				noteOns++
			case NoteOff:
				noteOffs++
			case ProgramChange:
				progChanges++
			case TempoChange:
				tempoChanges++
			}
		}
		t.Logf("  track %d (%s): %d events, %d noteOn, %d noteOff, %d progChange, %d tempo",
			i, track.Name, len(track.Events), noteOns, noteOffs, progChanges, tempoChanges)
		totalEvents += len(track.Events)
	}

	if totalEvents == 0 {
		t.Fatal("no events parsed in any track")
	}
	t.Logf("total events: %d", totalEvents)
	t.Logf("total ticks: %d", mf.TotalTicks())
	t.Logf("approx duration: %.1f seconds", mf.Duration())
}

func TestVarLen(t *testing.T) {
	tests := []struct {
		data  []byte
		value uint32
		bytes int
	}{
		{[]byte{0x00}, 0, 1},
		{[]byte{0x40}, 64, 1},
		{[]byte{0x7F}, 127, 1},
		{[]byte{0x81, 0x00}, 128, 2},
		{[]byte{0xC0, 0x00}, 8192, 2},
		{[]byte{0xFF, 0x7F}, 16383, 2},
		{[]byte{0x81, 0x80, 0x00}, 16384, 3},
	}

	for _, tt := range tests {
		val, n, err := readVarLen(tt.data)
		if err != nil {
			t.Errorf("readVarLen(%v) error: %v", tt.data, err)
			continue
		}
		if val != tt.value || n != tt.bytes {
			t.Errorf("readVarLen(%v) = (%d, %d), want (%d, %d)", tt.data, val, n, tt.value, tt.bytes)
		}
	}
}

func TestInvalidHeader(t *testing.T) {
	_, err := Parse(bytes.NewReader([]byte("INVALID!")))
	if err == nil {
		t.Fatal("expected error for invalid header")
	}
}

func TestFormat2Rejected(t *testing.T) {
	// Build a minimal header with format 2.
	var buf bytes.Buffer
	buf.WriteString("MThd")
	binary.Write(&buf, binary.BigEndian, uint32(6))
	binary.Write(&buf, binary.BigEndian, uint16(2)) // format 2
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, uint16(96))

	_, err := Parse(&buf)
	if err == nil {
		t.Fatal("expected error for format 2")
	}
}

func TestMinimalFormat0(t *testing.T) {
	// Build a minimal format 0 file with one track containing a NoteOn and EndOfTrack.
	var buf bytes.Buffer

	// Header chunk.
	buf.WriteString("MThd")
	binary.Write(&buf, binary.BigEndian, uint32(6))
	binary.Write(&buf, binary.BigEndian, uint16(0))  // format 0
	binary.Write(&buf, binary.BigEndian, uint16(1))  // 1 track
	binary.Write(&buf, binary.BigEndian, uint16(96)) // 96 ticks/quarter

	// Track chunk.
	var track bytes.Buffer
	// Delta=0, NoteOn ch0, note 60, vel 100
	track.Write([]byte{0x00, 0x90, 0x3C, 0x64})
	// Delta=96, NoteOff ch0, note 60, vel 0
	track.Write([]byte{0x60, 0x80, 0x3C, 0x00})
	// Delta=0, Meta EndOfTrack
	track.Write([]byte{0x00, 0xFF, 0x2F, 0x00})

	buf.WriteString("MTrk")
	binary.Write(&buf, binary.BigEndian, uint32(track.Len()))
	buf.Write(track.Bytes())

	mf, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if mf.Format != 0 {
		t.Errorf("format = %d, want 0", mf.Format)
	}
	if len(mf.Tracks) != 1 {
		t.Fatalf("tracks = %d, want 1", len(mf.Tracks))
	}

	events := mf.Tracks[0].Events
	if len(events) < 3 {
		t.Fatalf("events = %d, want >= 3", len(events))
	}

	// First event: NoteOn at tick 0.
	if events[0].Type != NoteOn || events[0].Tick != 0 || events[0].Data1 != 60 || events[0].Data2 != 100 {
		t.Errorf("event 0: got %+v", events[0])
	}

	// Second event: NoteOff at tick 96.
	if events[1].Type != NoteOff || events[1].Tick != 96 || events[1].Data1 != 60 {
		t.Errorf("event 1: got %+v", events[1])
	}

	// Third event: EndOfTrack.
	if events[2].Type != EndOfTrack {
		t.Errorf("event 2: got %+v", events[2])
	}
}

func TestRunningStatus(t *testing.T) {
	// Build a track that uses running status.
	var buf bytes.Buffer

	// Header.
	buf.WriteString("MThd")
	binary.Write(&buf, binary.BigEndian, uint32(6))
	binary.Write(&buf, binary.BigEndian, uint16(0))
	binary.Write(&buf, binary.BigEndian, uint16(1))
	binary.Write(&buf, binary.BigEndian, uint16(96))

	// Track.
	var track bytes.Buffer
	// Delta=0, NoteOn ch0, note 60, vel 100
	track.Write([]byte{0x00, 0x90, 0x3C, 0x64})
	// Delta=48, Running status NoteOn ch0, note 64, vel 80
	track.Write([]byte{0x30, 0x40, 0x50})
	// Delta=48, Running status NoteOn ch0, note 60, vel 0 (= NoteOff)
	track.Write([]byte{0x30, 0x3C, 0x00})
	// End of track.
	track.Write([]byte{0x00, 0xFF, 0x2F, 0x00})

	buf.WriteString("MTrk")
	binary.Write(&buf, binary.BigEndian, uint32(track.Len()))
	buf.Write(track.Bytes())

	mf, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	events := mf.Tracks[0].Events
	if len(events) < 3 {
		t.Fatalf("events = %d, want >= 3", len(events))
	}

	// Event 0: NoteOn note 60, vel 100, tick 0.
	if events[0].Type != NoteOn || events[0].Data1 != 60 || events[0].Tick != 0 {
		t.Errorf("event 0: got %+v", events[0])
	}

	// Event 1: NoteOn note 64, vel 80, tick 48 (running status).
	if events[1].Type != NoteOn || events[1].Data1 != 64 || events[1].Tick != 48 {
		t.Errorf("event 1: got %+v", events[1])
	}

	// Event 2: NoteOff note 60 (vel 0), tick 96 (running status).
	if events[2].Type != NoteOff || events[2].Data1 != 60 || events[2].Tick != 96 {
		t.Errorf("event 2: got %+v", events[2])
	}
}
