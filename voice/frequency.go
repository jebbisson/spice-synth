// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package voice

import (
	"fmt"
	"math"
)

const (
	// OPL clock frequency is typically based on the NTSC colorburst 3.579545 MHz.
	oplsClockRate = 3579545.0
)

// FNumberAndBlock calculates the OPL register values for a given frequency.
func FNumberAndBlock(freq float64) (fnumber uint16, block uint8) {
	if freq <= 0 {
		return 0, 0
	}

	// The formula from the manual: f-num = freq * 2^(20 - block) / 49716
	// We want to find the smallest 'block' such that the frequency is within range.
	// The maximum possible F-number is 1023.
	// Rearranging: 2^(20 - block) = (fnum * 49716) / freq

	for b := uint8(0); b <= 7; b++ {
		// Calculate the frequency range for this block.
		// Min F-num = 1, Max F-num = 1023
		minFreq := (1.0 * 49716.0) / math.Pow(2, float64(20-b))
		maxFreq := (1023.0 * 49716.0) / math.Pow(2, float64(20-b))

		if freq >= minFreq && freq <= maxFreq {
			// We found the right block! Now calculate fnumber.
			fnum := (freq * math.Pow(2, float64(20-b))) / 49716.0
			return uint16(math.Round(fnum)), b
		}
	}

	// Fallback if frequency is out of range for all blocks
	return 0, 0
}

// NoteToFrequency converts a MIDI note number to frequency in Hz.
func NoteToFrequency(midiNote int) float64 {
	return 440.0 * math.Pow(2, float64(midiNote-69)/12.0)
}

// ParseNote converts a note string (e.g., "C4", "Eb2", "F#3") to frequency in Hz.
func ParseNote(noteStr string) (float64, error) {
	if len(noteStr) < 2 {
		return 0, fmt.Errorf("invalid note format: %s", noteStr)
	}

	// Basic mapping for notes in an octave
	notes := map[string]int{
		"C": 0, "C#": 1, "Db": 1, "D": 2, "D#": 3, "Eb": 3, "E": 4,
		"F": 5, "F#": 6, "Gb": 6, "G": 7, "G#": 8, "Ab": 8, "A": 9,
		"A#": 10, "Bb": 10, "B": 11,
	}

	var noteName string
	var octaveStr string

	if len(noteStr) >= 3 && (noteStr[1] == '#' || noteStr[1] == 'b') {
		noteName = noteStr[:2]
		octaveStr = noteStr[2:]
	} else {
		noteName = noteStr[:1]
		octaveStr = noteStr[1:]
	}

	val, ok := notes[noteName]
	if !ok {
		return 0, fmt.Errorf("unknown note: %s", noteName)
	}

	var octave int
	_, err := fmt.Sscanf(octaveStr, "%d", &octave)
	if err != nil {
		return 0, fmt.Errorf("invalid octave: %s", octaveStr)
	}

	// MIDI note = (octave + 1) * 12 + note_value
	midiNote := (octave+1)*12 + val
	return NoteToFrequency(midiNote), nil
}
