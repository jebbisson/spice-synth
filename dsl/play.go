// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package dsl

import (
	"fmt"
	"math"

	"github.com/jebbisson/spice-synth/sequencer"
	"github.com/jebbisson/spice-synth/stream"
	"github.com/jebbisson/spice-synth/voice"
)

// rawWaveforms maps Strudel waveform names to OPL2 waveform register values.
var rawWaveforms = map[string]uint8{
	"sine":        0,
	"halfsine":    1,
	"abssine":     2,
	"quartersine": 3,
}

// Play compiles the Pattern into voice.Manager and sequencer calls, then
// starts playback on the specified channel of the given stream.
//
// This is the primary entry point: it translates the fluent DSL description
// into concrete OPL2 register writes via the existing voice and sequencer
// infrastructure.
func (p *Pattern) Play(s *stream.Stream, channel int) error {
	vm := s.Voices()
	seq := s.Sequencer()

	// 1. Resolve the instrument.
	inst, err := p.resolveInstrument(vm)
	if err != nil {
		return err
	}

	// 2. Apply Tier 2 overrides: carrier ADSR.
	p.applyCarrierADSR(inst)

	// 3. Apply Tier 3 overrides: FM parameters.
	p.applyFMParams(inst)

	// 4. Apply hardware flags.
	p.applyHWFlags(inst, vm)

	// 5. Apply velocity as a carrier level adjustment.
	p.applyVelocity(inst)

	// 6. Register the (possibly modified) instrument in the voice manager so
	//    the sequencer can resolve it by name during event triggering.
	//    Use a unique name for overridden instruments to avoid clashing with
	//    the original bank entry.
	instName := p.registerInstrument(vm, inst, channel)

	// 7. Build a sequencer pattern with the note and modulators.
	seqPat, err := p.buildSeqPattern(inst, instName, seq)
	if err != nil {
		return err
	}

	// 8. Assign pattern to channel.
	seq.SetPattern(channel, seqPat)

	return nil
}

// resolveInstrument looks up or creates the instrument for this pattern.
func (p *Pattern) resolveInstrument(vm *voice.Manager) (*voice.Instrument, error) {
	if p.sound == "" {
		// No instrument specified — create a default sine carrier patch.
		return p.defaultInstrument(), nil
	}

	// Check if it's a raw waveform name.
	if wf, ok := rawWaveforms[p.sound]; ok {
		inst := p.defaultInstrument()
		inst.Op2.Waveform = wf
		inst.Name = p.sound
		return inst, nil
	}

	// Look up named instrument from the bank.
	bankInst, err := vm.GetInstrument(p.sound)
	if err != nil {
		return nil, err
	}

	// Clone the instrument so DSL overrides don't mutate the bank.
	clone := *bankInst
	clone.Op1 = bankInst.Op1
	clone.Op2 = bankInst.Op2
	return &clone, nil
}

// defaultInstrument creates a minimal carrier-only instrument with sine
// waveform and neutral settings.
func (p *Pattern) defaultInstrument() *voice.Instrument {
	return &voice.Instrument{
		Name: "dsl_default",
		Op1: voice.Operator{
			Attack: 15, Decay: 0, Sustain: 0, Release: 8,
			Level: 63, Multiply: 1, Waveform: 0, // silent modulator
			Sustaining: true,
		},
		Op2: voice.Operator{
			Attack: 15, Decay: 4, Sustain: 2, Release: 8,
			Level: 0, Multiply: 1, Waveform: 0,
			Sustaining: true,
		},
		Feedback:   0,
		Connection: 0, // FM mode (though modulator is silent)
	}
}

// registerInstrument registers the instrument in the voice manager and returns
// the name under which it was registered. For instruments with DSL overrides
// the name is made unique per-channel to avoid clobbering other patterns.
func (p *Pattern) registerInstrument(vm *voice.Manager, inst *voice.Instrument, channel int) string {
	name := inst.Name
	// If the instrument was modified by DSL overrides (anything other than
	// a plain bank lookup), give it a unique name so we don't mutate the
	// shared bank entry.
	if p.hasOverrides() {
		name = fmt.Sprintf("_dsl_%s_ch%d", inst.Name, channel)
		inst.Name = name
	}
	vm.LoadBank("dsl", []*voice.Instrument{inst})
	return name
}

// hasOverrides returns true if the pattern has any DSL parameter overrides
// that would have modified the resolved instrument.
func (p *Pattern) hasOverrides() bool {
	return p.attack.isPresent || p.decay.isPresent || p.sustain.isPresent ||
		p.release.isPresent || p.sustained != nil ||
		p.fm.isPresent || p.fmh.isPresent || p.fmAttack.isPresent ||
		p.fmDecay.isPresent || p.fmSustain.isPresent ||
		p.feedback.isPresent || p.conn != nil ||
		p.carrierWF != nil || p.modWF != nil ||
		p.hwTremolo != nil || p.hwVibrato != nil ||
		p.gain.isPresent || p.velocity.isPresent
}

// ---------------------------------------------------------------------------
// Velocity
// ---------------------------------------------------------------------------

// applyVelocity scales the carrier total level by the velocity value.
// Velocity 1.0 = no change, 0.0 = silent. The velocity acts as a multiplier
// on the carrier's output level (attenuation).
func (p *Pattern) applyVelocity(inst *voice.Instrument) {
	if !p.velocity.isPresent || p.velocity.isSignal {
		return
	}
	vel := p.velocity.static
	if vel <= 0 {
		inst.Op2.Level = 63 // silent
		return
	}
	if vel >= 1 {
		return // no change
	}
	// Velocity scales the attenuation: more attenuation = quieter.
	// Current level is the base attenuation. We add attenuation for lower velocity.
	// Additional attenuation = (1 - vel) * 63
	additional := uint8((1.0 - vel) * 63.0)
	total := int(inst.Op2.Level) + int(additional)
	if total > 63 {
		total = 63
	}
	inst.Op2.Level = uint8(total)
}

// ---------------------------------------------------------------------------
// Tier 2: Carrier ADSR overrides
// ---------------------------------------------------------------------------

// adsrRates maps continuous time values (in seconds) to OPL2's 4-bit rate
// values (0-15). The OPL2 envelope is exponential, and rate 15 is the fastest
// (~0ms) while rate 1 is the slowest (~10s). Rate 0 means no envelope action.
//
// These approximate mappings are derived from the OPL2 datasheet:
//
//	Rate 0:  infinite (no attack / no decay)
//	Rate 1:  ~10.0s
//	Rate 2:  ~8.0s
//	Rate 3:  ~6.0s
//	Rate 4:  ~4.8s
//	Rate 5:  ~3.4s
//	Rate 6:  ~2.4s
//	Rate 7:  ~1.7s
//	Rate 8:  ~1.2s
//	Rate 9:  ~0.84s
//	Rate 10: ~0.60s
//	Rate 11: ~0.42s
//	Rate 12: ~0.30s
//	Rate 13: ~0.21s
//	Rate 14: ~0.15s
//	Rate 15: ~0 (instant)
var adsrTimesSeconds = [16]float64{
	math.Inf(1), // 0: infinite
	10.0,        // 1
	8.0,         // 2
	6.0,         // 3
	4.8,         // 4
	3.4,         // 5
	2.4,         // 6
	1.7,         // 7
	1.2,         // 8
	0.84,        // 9
	0.60,        // 10
	0.42,        // 11
	0.30,        // 12
	0.21,        // 13
	0.15,        // 14
	0.0,         // 15: instant
}

// secondsToRate converts a time in seconds to the nearest OPL2 4-bit rate.
func secondsToRate(sec float64) uint8 {
	if sec <= 0 {
		return 15 // instant
	}
	if math.IsInf(sec, 1) {
		return 0
	}

	best := uint8(0)
	bestDist := math.Inf(1)
	for i := uint8(0); i < 16; i++ {
		dist := math.Abs(adsrTimesSeconds[i] - sec)
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}
	return best
}

// sustainToOPL converts a normalised sustain level (0.0=silent, 1.0=loudest)
// to OPL2's inverted 4-bit sustain (0=loudest, 15=silent).
func sustainToOPL(level float64) uint8 {
	if level <= 0 {
		return 15
	}
	if level >= 1 {
		return 0
	}
	return uint8(math.Round((1.0 - level) * 15.0))
}

func (p *Pattern) applyCarrierADSR(inst *voice.Instrument) {
	if p.attack.isPresent && !p.attack.isSignal {
		inst.Op2.Attack = secondsToRate(p.attack.static)
	}
	if p.decay.isPresent && !p.decay.isSignal {
		inst.Op2.Decay = secondsToRate(p.decay.static)
	}
	if p.sustain.isPresent && !p.sustain.isSignal {
		inst.Op2.Sustain = sustainToOPL(p.sustain.static)
	}
	if p.release.isPresent && !p.release.isSignal {
		inst.Op2.Release = secondsToRate(p.release.static)
	}
	if p.sustained != nil {
		inst.Op2.Sustaining = *p.sustained
	}
}

// ---------------------------------------------------------------------------
// Tier 3: FM parameter overrides
// ---------------------------------------------------------------------------

// fmDepthToLevel converts an FM depth (0-inf) to OPL2 modulator total level.
// Higher depth = lower attenuation = more modulation.
// Mapping: level = max(0, 63 - depth * 6.3), so FM=10 ~ level 0 (max).
func fmDepthToLevel(depth float64) uint8 {
	level := 63.0 - depth*6.3
	if level < 0 {
		level = 0
	}
	if level > 63 {
		level = 63
	}
	return uint8(math.Round(level))
}

// fmhToMultiplier converts a harmonicity ratio to OPL2's frequency multiplier
// register value (0-15). The OPL2 multiplier table:
//
//	Reg 0 -> ratio 0.5
//	Reg 1 -> ratio 1
//	Reg 2 -> ratio 2
//	...
//	Reg 10 -> ratio 10
//	Reg 11 -> ratio 10
//	Reg 12 -> ratio 12
//	Reg 13 -> ratio 12
//	Reg 14 -> ratio 15
//	Reg 15 -> ratio 15
var fmhRatios = [16]float64{
	0.5, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 12, 12, 15, 15,
}

func fmhToMultiplier(ratio float64) uint8 {
	best := uint8(0)
	bestDist := math.Inf(1)
	for i := uint8(0); i < 16; i++ {
		dist := math.Abs(fmhRatios[i] - ratio)
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}
	return best
}

func (p *Pattern) applyFMParams(inst *voice.Instrument) {
	// FM depth (static only — signal handled as modulator)
	if p.fm.isPresent && !p.fm.isSignal {
		inst.Op1.Level = fmDepthToLevel(p.fm.static)
	}

	// FM harmonicity
	if p.fmh.isPresent && !p.fmh.isSignal {
		inst.Op1.Multiply = fmhToMultiplier(p.fmh.static)
	}

	// Modulator ADSR
	if p.fmAttack.isPresent && !p.fmAttack.isSignal {
		inst.Op1.Attack = secondsToRate(p.fmAttack.static)
	}
	if p.fmDecay.isPresent && !p.fmDecay.isSignal {
		inst.Op1.Decay = secondsToRate(p.fmDecay.static)
	}
	if p.fmSustain.isPresent && !p.fmSustain.isSignal {
		inst.Op1.Sustain = sustainToOPL(p.fmSustain.static)
	}

	// Feedback (static only)
	if p.feedback.isPresent && !p.feedback.isSignal {
		fb := uint8(p.feedback.static)
		if fb > 7 {
			fb = 7
		}
		inst.Feedback = fb
	}

	// Connection
	if p.conn != nil {
		if *p.conn != 0 {
			inst.Connection = 1
		} else {
			inst.Connection = 0
		}
	}

	// Waveforms
	if p.carrierWF != nil {
		wf := uint8(*p.carrierWF)
		if wf > 3 {
			wf = 3
		}
		inst.Op2.Waveform = wf
	}
	if p.modWF != nil {
		wf := uint8(*p.modWF)
		if wf > 3 {
			wf = 3
		}
		inst.Op1.Waveform = wf
	}
}

// ---------------------------------------------------------------------------
// Hardware feature flags
// ---------------------------------------------------------------------------

func (p *Pattern) applyHWFlags(inst *voice.Instrument, _ *voice.Manager) {
	if p.hwTremolo != nil {
		inst.Op2.Tremolo = *p.hwTremolo
	}
	if p.hwVibrato != nil {
		inst.Op2.Vibrato = *p.hwVibrato
	}
}

// ---------------------------------------------------------------------------
// Build sequencer pattern
// ---------------------------------------------------------------------------

func (p *Pattern) buildSeqPattern(inst *voice.Instrument, instName string, _ *sequencer.Sequencer) (*sequencer.Pattern, error) {
	// Create a single-step pattern that plays the note immediately and holds.
	// For Phase 1, patterns are single-note; multi-note patterns come in Phase 4.
	seqPat := sequencer.NewPattern(64) // 64 steps, long enough for held notes
	seqPat.Instrument(instName)

	// Parse and add the note.
	noteStr := p.noteStr
	if !p.noteSet {
		noteStr = "C4" // default note
	}

	freq, err := voice.ParseNote(noteStr)
	if err != nil {
		return nil, err
	}

	seqPat.Events = append(seqPat.Events, sequencer.Event{
		Step:       0,
		Type:       sequencer.NoteOn,
		Note:       voice.Note(freq),
		Instrument: instName,
	})

	// Attach modulators from signal-valued parameters.
	p.attachSignalModulators(seqPat)

	// Attach ramp modulator if specified.
	if p.rampFrom != nil && p.rampTo != nil && p.rampSec != nil {
		seqPat.ModRamp(voice.ModCarrierLevel, *p.rampFrom, *p.rampTo, *p.rampSec)
	}

	// Attach gain modulator (signal-driven volume).
	if p.gain.isPresent && p.gain.isSignal {
		mod := p.gain.signal.compile(voice.ModCarrierLevel)
		seqPat.ModDefs = append(seqPat.ModDefs, &compiledModDef{mod: mod})
	}

	// Attach feedback modulator (signal-driven feedback).
	if p.feedback.isPresent && p.feedback.isSignal {
		mod := p.feedback.signal.compile(voice.ModFeedback)
		seqPat.ModDefs = append(seqPat.ModDefs, &compiledModDef{mod: mod})
	}

	// Attach FM depth modulator (signal-driven modulator level).
	if p.fm.isPresent && p.fm.isSignal {
		mod := p.fm.signal.compile(voice.ModModulatorLevel)
		seqPat.ModDefs = append(seqPat.ModDefs, &compiledModDef{mod: mod})
	}

	return seqPat, nil
}

// attachSignalModulators adds signal-based modulators to the sequencer pattern.
// This is separate from the static parameter application done in applyXxx methods.
func (p *Pattern) attachSignalModulators(_ *sequencer.Pattern) {
	// Currently signal modulators are handled in buildSeqPattern directly.
	// This method is a placeholder for future expansion when more signal
	// targets are added (frequency modulation, etc.).
}

// compiledModDef wraps an already-compiled voice.Modulator as a ModDef.
// This is used when Signal.compile() has already produced the modulator and
// we just need to pass it through the sequencer's ModDef interface.
type compiledModDef struct {
	mod voice.Modulator
}

func (d *compiledModDef) Build(_ int) voice.Modulator {
	return d.mod
}

// ---------------------------------------------------------------------------
// PlayDirect provides a simpler path for single-note playback that bypasses
// the sequencer entirely, writing directly to the voice manager.
// ---------------------------------------------------------------------------

// PlayDirect plays the pattern immediately on the given channel without going
// through the sequencer. This is useful for sustained drone notes and testing.
func (p *Pattern) PlayDirect(s *stream.Stream, channel int) error {
	vm := s.Voices()

	// 1. Resolve the instrument.
	inst, err := p.resolveInstrument(vm)
	if err != nil {
		return err
	}

	// 2. Apply all parameter overrides.
	p.applyCarrierADSR(inst)
	p.applyFMParams(inst)
	p.applyHWFlags(inst, vm)
	p.applyVelocity(inst)

	// 3. Apply static gain as carrier level.
	if p.gain.isPresent && !p.gain.isSignal {
		level := uint8((1.0 - p.gain.static) * 63.0)
		inst.Op2.Level = level
	}

	// 4. Parse note and trigger.
	noteStr := p.noteStr
	if !p.noteSet {
		noteStr = "C4"
	}
	freq, err := voice.ParseNote(noteStr)
	if err != nil {
		return err
	}

	// 5. Register the instrument temporarily.
	vm.LoadBank("dsl", []*voice.Instrument{inst})

	// 6. NoteOn.
	if err := vm.NoteOn(channel, voice.Note(freq), inst); err != nil {
		return err
	}

	// 7. Clear existing mods and attach new ones.
	vm.ClearMods(channel)

	// Ramp modulator.
	if p.rampFrom != nil && p.rampTo != nil && p.rampSec != nil {
		ramp := voice.NewRamp(voice.ModCarrierLevel, *p.rampFrom, *p.rampTo, *p.rampSec, 44100)
		vm.AttachMod(channel, ramp)
	}

	// Gain signal modulator.
	if p.gain.isPresent && p.gain.isSignal {
		mod := p.gain.signal.compile(voice.ModCarrierLevel)
		vm.AttachMod(channel, mod)
	}

	// Feedback signal modulator.
	if p.feedback.isPresent && p.feedback.isSignal {
		mod := p.feedback.signal.compile(voice.ModFeedback)
		vm.AttachMod(channel, mod)
	}

	// FM depth signal modulator.
	if p.fm.isPresent && p.fm.isSignal {
		mod := p.fm.signal.compile(voice.ModModulatorLevel)
		vm.AttachMod(channel, mod)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Convenience constructors that bypass method chaining
// ---------------------------------------------------------------------------

// S creates a new Pattern with just an instrument selected.
func S(name string) *Pattern {
	return &Pattern{sound: name}
}

// N creates a new Pattern from scale step indices.
func N(step string) *Pattern {
	return &Pattern{nStep: step}
}
