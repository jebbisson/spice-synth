// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package instrument

import (
	"fmt"
	"strings"

	"github.com/jebbisson/spice-synth/adl"
	"github.com/jebbisson/spice-synth/op2"
	"github.com/jebbisson/spice-synth/voice"
)

const extractedGroup = "extracted"

// ExtractFromADL extracts all instruments from an ADL file and groups identical
// configurations under the first-seen instrument name.
func ExtractFromADL(file *adl.File) *File {
	return ExtractFromADLFiles([]*adl.File{file})
}

// ExtractFromADLFiles extracts all instruments from multiple ADL files and
// groups identical configurations under the first-seen instrument name.
func ExtractFromADLFiles(files []*adl.File) *File {
	insts := make([]*voice.Instrument, 0)
	for _, file := range files {
		if file == nil {
			continue
		}
		insts = append(insts, file.ExtractInstruments("adl")...)
	}
	if len(insts) == 0 {
		return &File{}
	}
	return extractFromVoiceInstruments(insts)
}

// ExtractFromOP2Bank extracts all melodic and percussion instruments from an
// OP2 bank and groups identical configurations under the first-seen name.
func ExtractFromOP2Bank(bank *op2.Bank) *File {
	if bank == nil {
		return &File{}
	}
	insts := make([]*voice.Instrument, 0, len(bank.Melodic)+len(bank.Percussion))
	for i := range bank.Melodic {
		insts = append(insts, bank.Melodic[i].ToInstrument())
	}
	for i := range bank.Percussion {
		insts = append(insts, bank.Percussion[i].ToInstrument())
	}
	return extractFromVoiceInstruments(insts)
}

func extractFromVoiceInstruments(insts []*voice.Instrument) *File {
	out := &File{}
	bySignature := make(map[string]*FileInstrument)
	nameCounts := make(map[string]int)

	for _, inst := range insts {
		if inst == nil {
			continue
		}
		def := ToInstrumentDef(inst)
		if def == nil {
			continue
		}
		sig := instrumentSignature(def)
		entry, ok := bySignature[sig]
		if !ok {
			entryName := uniqueInstrumentName(normalizeInstrumentName(inst.Name), nameCounts)
			entry = &FileInstrument{
				Name:        entryName,
				Group:       extractedGroup,
				DefaultNote: defaultPreviewNote(inst),
				Variants: []InstrumentDef{{
					Name:       "default",
					Op1:        def.Op1,
					Op2:        def.Op2,
					Feedback:   def.Feedback,
					Connection: def.Connection,
				}},
			}
			out.Instruments = append(out.Instruments, *entry)
			bySignature[sig] = &out.Instruments[len(out.Instruments)-1]
			continue
		}
		_ = entry
		continue
	}

	return out
}

func instrumentSignature(def *InstrumentDef) string {
	return fmt.Sprintf("%d:%d:%d:%d:%d:%t:%d:%t:%t:%t:%d|%d:%d:%d:%d:%d:%t:%d:%t:%t:%t:%d|%d|%d",
		def.Op1.Attack, def.Op1.Decay, def.Op1.Sustain, def.Op1.Release, def.Op1.Multiply,
		def.Op1.KeyScaleRate, def.Op1.KeyScaleLevel, def.Op1.Tremolo, def.Op1.Vibrato, def.Op1.Sustaining, def.Op1.Waveform,
		def.Op2.Attack, def.Op2.Decay, def.Op2.Sustain, def.Op2.Release, def.Op2.Multiply,
		def.Op2.KeyScaleRate, def.Op2.KeyScaleLevel, def.Op2.Tremolo, def.Op2.Vibrato, def.Op2.Sustaining, def.Op2.Waveform,
		def.Feedback, def.Connection)
}

func defaultPreviewNote(inst *voice.Instrument) string {
	if inst == nil {
		return "C4"
	}
	// Default to a mid-range note for auditioning. Notes are not part of
	// instrument identity and extraction currently has no per-program pitch data.
	return "C4"
}

func normalizeInstrumentName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func normalizeVariantName(name string) string {
	name = normalizeInstrumentName(name)
	if name == "" {
		return ""
	}
	return name
}

func uniqueInstrumentName(base string, counts map[string]int) string {
	if base == "" {
		base = "unnamed"
	}
	count := counts[base]
	counts[base] = count + 1
	if count == 0 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, count)
}
