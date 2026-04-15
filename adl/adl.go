// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.
//
// Ported from AdPlug (https://github.com/adplug/adplug), original code by
// Torbjorn Andersson and Johannes Schickel of the ScummVM project.
// Original code is LGPL-2.1. See THIRD_PARTY_LICENSES for details.

// Package adl implements a parser and player for the Westwood Studios ADL
// music format used in games like Dune II, Eye of the Beholder, and Kyrandia.
//
// ADL files contain a bytecode virtual machine that drives OPL2 FM synthesis
// at a 72Hz tick rate. Each file embeds its own instrument definitions as
// raw OPL register values.
//
// This implementation targets the v2/v3 format variant used by Dune II.
package adl

import (
	"encoding/binary"
	"fmt"
	adplugadl "github.com/jebbisson/spice-adl-adplug"
	"io"

	"github.com/jebbisson/spice-synth/voice"
)

// File represents a parsed ADL file.
type File struct {
	Version      int       // Format version (1, 3, or 4). Dune II = 3.
	NumPrograms  int       // Number of program slots (150/250/500).
	NumSubsongs  int       // Number of valid subsong entries.
	TrackEntries [500]byte // Subsong → program ID lookup (v1/v2/v3: 120 uint8; v4: 250 uint16).
	SoundData    []byte    // Raw sound data (program/instrument offset tables + bytecode).
}

// Parse reads an ADL file from the given reader. The entire file is read
// into memory. Returns a parsed File or an error.
func Parse(r io.Reader) (*File, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("adl: read error: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes parses an ADL file from raw bytes.
func ParseBytes(data []byte) (*File, error) {
	if len(data) < 720 {
		return nil, fmt.Errorf("adl: file too small (%d bytes, minimum 720)", len(data))
	}

	f := &File{}

	// Read the first 500 bytes into trackEntries (maximum possible size).
	copy(f.TrackEntries[:], data[:min(500, len(data))])

	// Detect version: v4 has 250 uint16 track entries (500 bytes).
	// For v1/v2/v3, the first 120 bytes are uint8 track entries, and
	// bytes 120+ are part of soundData which will contain program offsets
	// (values >= 500 when read as uint16).
	ofs := 500
	f.Version = 4

	for i := 0; i < 500; i += 2 {
		w := binary.LittleEndian.Uint16(data[i : i+2])
		if w >= 500 && w < 0xFFFF {
			f.Version = 3 // Could be 1, 2, or 3; refined below.
			ofs = 120
			break
		}
	}

	// Extract sound data (everything after track entries).
	if len(data) <= ofs {
		return nil, fmt.Errorf("adl: file too small for version %d", f.Version)
	}
	f.SoundData = make([]byte, len(data)-ofs)
	copy(f.SoundData, data[ofs:])

	// For v1/v2/v3, clear the track entry bytes beyond 120.
	if f.Version < 4 {
		for i := 120; i < 500; i++ {
			f.TrackEntries[i] = 0xFF
		}
	}

	// Determine number of programs and refine version.
	if f.Version < 4 {
		numProgs := 150 // v1 default
		for i := 0; i < numProgs*2; i += 2 {
			if i+1 >= len(f.SoundData) {
				break
			}
			w := binary.LittleEndian.Uint16(f.SoundData[i : i+2])
			if w > 0 && w < 600 {
				return nil, fmt.Errorf("adl: invalid program offset %d at index %d", w, i/2)
			}
			if w > 0 && w < 1000 {
				f.Version = 1
			}
		}

		if f.Version > 1 {
			if len(data) < 1120 {
				return nil, fmt.Errorf("adl: file too small for v2/v3 (%d bytes, minimum 1120)", len(data))
			}
			numProgs = 250
			for i := 150 * 2; i < numProgs*2; i += 2 {
				if i+1 >= len(f.SoundData) {
					break
				}
				w := binary.LittleEndian.Uint16(f.SoundData[i : i+2])
				if w > 0 && w < 1000 {
					return nil, fmt.Errorf("adl: invalid v3 program offset %d at index %d", w, i/2)
				}
			}
		}
		f.NumPrograms = numProgs
	} else {
		if len(data) < 2500 {
			return nil, fmt.Errorf("adl: file too small for v4 (%d bytes, minimum 2500)", len(data))
		}
		f.NumPrograms = 500
		for i := 0; i < 500*2; i += 2 {
			if i+1 >= len(f.SoundData) {
				break
			}
			w := binary.LittleEndian.Uint16(f.SoundData[i : i+2])
			if w > 0 && w < 2000 {
				return nil, fmt.Errorf("adl: invalid v4 program offset %d at index %d", w, i/2)
			}
		}
	}

	// Count valid subsongs.
	if f.Version == 4 {
		for i := 250; i > 0; i-- {
			w := binary.LittleEndian.Uint16(f.TrackEntries[(i-1)*2 : (i-1)*2+2])
			if int(w) < f.NumPrograms {
				f.NumSubsongs = i
				break
			}
		}
	} else {
		for i := 120; i > 0; i-- {
			if int(f.TrackEntries[i-1]) < f.NumPrograms {
				f.NumSubsongs = i
				break
			}
		}
	}

	return f, nil
}

// GetProgram returns the bytecode data for a given program ID, or nil if invalid.
func (f *File) GetProgram(progID int) []byte {
	if progID < 0 || progID*2+1 >= len(f.SoundData) {
		return nil
	}
	offset := binary.LittleEndian.Uint16(f.SoundData[progID*2 : progID*2+2])
	if offset == 0 || int(offset) >= len(f.SoundData) {
		return nil
	}
	return f.SoundData[offset:]
}

// GetInstrument returns the raw 11-byte instrument data for the given
// instrument ID, or nil if invalid.
func (f *File) GetInstrument(instID int) []byte {
	return f.GetProgram(f.NumPrograms + instID)
}

// TrackForSubsong returns the program ID for the given subsong index.
// Returns -1 if the subsong is invalid.
func (f *File) TrackForSubsong(subsong int) int {
	if f.Version == 4 {
		if subsong < 0 || subsong*2+1 >= 500 {
			return -1
		}
		w := binary.LittleEndian.Uint16(f.TrackEntries[subsong*2 : subsong*2+2])
		if int(w) >= f.NumPrograms {
			return -1
		}
		return int(w)
	}
	if subsong < 0 || subsong >= 120 {
		return -1
	}
	id := f.TrackEntries[subsong]
	if int(id) >= f.NumPrograms {
		return -1
	}
	return int(id)
}

// InstrumentCount returns the number of instrument slots in this file.
func (f *File) InstrumentCount() int {
	return f.NumPrograms
}

// RawInstrument holds the 11 raw OPL register bytes for a single ADL instrument.
type RawInstrument struct {
	ModChar   uint8 // 0x20+offset: AM/Vib/EG/KSR/Mult for op1
	CarChar   uint8 // 0x23+offset: AM/Vib/EG/KSR/Mult for op2
	FeedConn  uint8 // 0xC0+channel: Feedback/Connection
	ModWave   uint8 // 0xE0+offset: Waveform for op1
	CarWave   uint8 // 0xE3+offset: Waveform for op2
	ModLevel  uint8 // op1 total level (KSL | TotalLevel)
	CarLevel  uint8 // op2 total level (KSL | TotalLevel)
	ModAttDec uint8 // 0x60+offset: Attack/Decay for op1
	CarAttDec uint8 // 0x63+offset: Attack/Decay for op2
	ModSusRel uint8 // 0x80+offset: Sustain/Release for op1
	CarSusRel uint8 // 0x83+offset: Sustain/Release for op2
}

// ParseRawInstrument parses an 11-byte instrument definition from the given data.
func ParseRawInstrument(data []byte) (RawInstrument, error) {
	if len(data) < 11 {
		return RawInstrument{}, fmt.Errorf("adl: instrument data too short (%d bytes, need 11)", len(data))
	}
	return RawInstrument{
		ModChar:   data[0],
		CarChar:   data[1],
		FeedConn:  data[2],
		ModWave:   data[3],
		CarWave:   data[4],
		ModLevel:  data[5],
		CarLevel:  data[6],
		ModAttDec: data[7],
		CarAttDec: data[8],
		ModSusRel: data[9],
		CarSusRel: data[10],
	}, nil
}

// ToVoiceInstrument converts a RawInstrument to a voice.Instrument for use
// with the SpiceSynth voice manager.
func (ri RawInstrument) ToVoiceInstrument(name string) *voice.Instrument {
	return &voice.Instrument{
		Name: name,
		Op1: voice.Operator{
			Tremolo:       ri.ModChar&0x80 != 0,
			Vibrato:       ri.ModChar&0x40 != 0,
			Sustaining:    ri.ModChar&0x20 != 0,
			KeyScaleRate:  ri.ModChar&0x10 != 0,
			Multiply:      ri.ModChar & 0x0F,
			KeyScaleLevel: (ri.ModLevel >> 6) & 0x03,
			Level:         ri.ModLevel & 0x3F,
			Attack:        (ri.ModAttDec >> 4) & 0x0F,
			Decay:         ri.ModAttDec & 0x0F,
			Sustain:       (ri.ModSusRel >> 4) & 0x0F,
			Release:       ri.ModSusRel & 0x0F,
			Waveform:      ri.ModWave & 0x03,
		},
		Op2: voice.Operator{
			Tremolo:       ri.CarChar&0x80 != 0,
			Vibrato:       ri.CarChar&0x40 != 0,
			Sustaining:    ri.CarChar&0x20 != 0,
			KeyScaleRate:  ri.CarChar&0x10 != 0,
			Multiply:      ri.CarChar & 0x0F,
			KeyScaleLevel: (ri.CarLevel >> 6) & 0x03,
			Level:         ri.CarLevel & 0x3F,
			Attack:        (ri.CarAttDec >> 4) & 0x0F,
			Decay:         ri.CarAttDec & 0x0F,
			Sustain:       (ri.CarSusRel >> 4) & 0x0F,
			Release:       ri.CarSusRel & 0x0F,
			Waveform:      ri.CarWave & 0x03,
		},
		Feedback:   (ri.FeedConn >> 1) & 0x07,
		Connection: ri.FeedConn & 0x01,
	}
}

// SubsongType classifies the kind of content in a subsong slot.
type SubsongType int

const (
	// SubsongEmpty means the slot has no valid program (trackEntry = 0xFF or
	// program offset is zero).
	SubsongEmpty SubsongType = iota
	// SubsongReset means the program targets channel 9 (control) but does not
	// spawn any sub-programs — typically a silence/reset track.
	SubsongReset
	// SubsongMusic means the program targets channel 9 and uses the
	// setupProgram opcode to orchestrate melodic channels.
	SubsongMusic
	// SubsongSFX means the program targets a melodic channel (0-8) directly,
	// which is the pattern used for sound effects.
	SubsongSFX
)

// String returns a short human-readable label for the subsong type.
func (t SubsongType) String() string {
	switch t {
	case SubsongEmpty:
		return "EMPTY"
	case SubsongReset:
		return "RESET"
	case SubsongMusic:
		return "MUSIC"
	case SubsongSFX:
		return "SFX"
	default:
		return "?"
	}
}

// SubsongInfo describes a single subsong slot.
type SubsongInfo struct {
	Index int         // Original subsong index in the track table.
	Type  SubsongType // Classification of the subsong content.
}

// ClassifySubsongs scans all subsong slots and returns their classifications.
// The returned slice has exactly NumSubsongs entries.
func (f *File) ClassifySubsongs() []SubsongInfo {
	infos := make([]SubsongInfo, f.NumSubsongs)
	for i := 0; i < f.NumSubsongs; i++ {
		infos[i] = SubsongInfo{Index: i, Type: f.classifySubsong(i)}
	}
	return infos
}

// NonEmptySubsongs returns only the subsong slots that are not empty,
// in their original order. This is useful for player UIs that want to
// skip over the many unused slots in Dune II ADL files.
func (f *File) NonEmptySubsongs() []SubsongInfo {
	all := f.ClassifySubsongs()
	var result []SubsongInfo
	for _, info := range all {
		if info.Type != SubsongEmpty {
			result = append(result, info)
		}
	}
	return result
}

// classifySubsong determines the type of a single subsong slot.
func (f *File) classifySubsong(subsong int) SubsongType {
	trackID := f.TrackForSubsong(subsong)
	if trackID < 0 {
		return SubsongEmpty
	}

	prog := f.GetProgram(trackID)
	if prog == nil || len(prog) < 2 {
		return SubsongEmpty
	}

	targetChan := prog[0]
	if targetChan > 9 {
		return SubsongEmpty
	}

	// Programs targeting melodic channels 0-8 directly are sound effects.
	if targetChan < 9 {
		return SubsongSFX
	}

	// Channel 9 = control channel. Scan bytecode for setupProgram (0x82).
	// Start after the 2-byte header (channel + priority).
	ptr := 2
	for ptr < len(prog) {
		b := prog[ptr]
		ptr++

		if b&0x80 != 0 {
			// Opcode.
			idx := int(b & 0x7F)
			if idx >= len(opcodeParamCount) {
				idx = len(opcodeParamCount) - 1
			}

			// Opcode 2 = setupProgram → this is a music track.
			if idx == 2 {
				return SubsongMusic
			}

			// Stop opcodes terminate the program scan.
			if idx == 8 || idx == 20 || idx == 22 || idx == 23 ||
				idx == 24 || idx == 25 || idx == 27 {
				break
			}

			nParams := opcodeParamCount[idx]
			ptr += nParams
		} else {
			// Note byte: followed by 1 duration byte.
			ptr++
		}
	}

	return SubsongReset
}

// ExtractInstruments reads all valid instruments from the file and converts
// them to voice.Instrument values. Each instrument is named with the given
// prefix and its index (e.g., "dune2_000").
func (f *File) ExtractInstruments(prefix string) []*voice.Instrument {
	var instruments []*voice.Instrument
	for i := 0; i < f.NumPrograms; i++ {
		data := f.GetInstrument(i)
		if data == nil || len(data) < 11 {
			continue
		}
		ri, err := ParseRawInstrument(data)
		if err != nil {
			continue
		}
		name := fmt.Sprintf("%s_%03d", prefix, i)
		instruments = append(instruments, ri.ToVoiceInstrument(name))
	}
	return instruments
}

func toExternalFile(file *File) *adplugadl.File {
	if file == nil {
		return nil
	}
	return &adplugadl.File{
		Version:      file.Version,
		NumPrograms:  file.NumPrograms,
		NumSubsongs:  file.NumSubsongs,
		TrackEntries: file.TrackEntries,
		SoundData:    append([]byte(nil), file.SoundData...),
	}
}
