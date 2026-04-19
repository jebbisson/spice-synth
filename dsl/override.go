// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package dsl

import "github.com/jebbisson/spice-synth/voice"

// InstrumentOverrideBuilder fluently constructs note-start overrides.
type InstrumentOverrideBuilder struct {
	value voice.InstrumentOverride
}

// OperatorOverrideBuilder fluently constructs operator overrides.
type OperatorOverrideBuilder struct {
	value voice.OperatorOverride
}

// Override starts a fluent note-start override builder.
func Override() *InstrumentOverrideBuilder {
	return &InstrumentOverrideBuilder{}
}

// OpOverride starts a fluent operator override builder.
func OpOverride() *OperatorOverrideBuilder {
	return &OperatorOverrideBuilder{}
}

// Op1 applies a modulator override.
func (b *InstrumentOverrideBuilder) Op1(op *OperatorOverrideBuilder) *InstrumentOverrideBuilder {
	if b == nil || op == nil {
		return b
	}
	b.value.Op1 = cloneOperatorOverride(op.value)
	return b
}

// Op2 applies a carrier override.
func (b *InstrumentOverrideBuilder) Op2(op *OperatorOverrideBuilder) *InstrumentOverrideBuilder {
	if b == nil || op == nil {
		return b
	}
	b.value.Op2 = cloneOperatorOverride(op.value)
	return b
}

// Feedback overrides channel feedback.
func (b *InstrumentOverrideBuilder) Feedback(v uint8) *InstrumentOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Feedback = &value
	return b
}

// Connection overrides channel connection mode.
func (b *InstrumentOverrideBuilder) Connection(v uint8) *InstrumentOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Connection = &value
	return b
}

// ModulatorLevel overrides the modulator level.
func (b *InstrumentOverrideBuilder) ModulatorLevel(v uint8) *InstrumentOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Op1.Level = &value
	return b
}

// CarrierLevel overrides the carrier level.
func (b *InstrumentOverrideBuilder) CarrierLevel(v uint8) *InstrumentOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Op2.Level = &value
	return b
}

// Build returns the constructed override.
func (b *InstrumentOverrideBuilder) Build() *voice.InstrumentOverride {
	if b == nil {
		return nil
	}
	return cloneInstrumentOverride(&b.value)
}

// Attack overrides operator attack.
func (b *OperatorOverrideBuilder) Attack(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Attack = &value
	return b
}

// Decay overrides operator decay.
func (b *OperatorOverrideBuilder) Decay(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Decay = &value
	return b
}

// Sustain overrides operator sustain.
func (b *OperatorOverrideBuilder) Sustain(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Sustain = &value
	return b
}

// Release overrides operator release.
func (b *OperatorOverrideBuilder) Release(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Release = &value
	return b
}

// Level overrides operator level.
func (b *OperatorOverrideBuilder) Level(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Level = &value
	return b
}

// Multiply overrides operator multiplier.
func (b *OperatorOverrideBuilder) Multiply(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Multiply = &value
	return b
}

// KeyScaleRate overrides operator key-scale-rate.
func (b *OperatorOverrideBuilder) KeyScaleRate(v bool) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.KeyScaleRate = &value
	return b
}

// KeyScaleLevel overrides operator key-scale-level.
func (b *OperatorOverrideBuilder) KeyScaleLevel(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.KeyScaleLevel = &value
	return b
}

// Tremolo overrides operator tremolo.
func (b *OperatorOverrideBuilder) Tremolo(v bool) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Tremolo = &value
	return b
}

// Vibrato overrides operator vibrato.
func (b *OperatorOverrideBuilder) Vibrato(v bool) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Vibrato = &value
	return b
}

// Sustaining overrides operator sustaining mode.
func (b *OperatorOverrideBuilder) Sustaining(v bool) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Sustaining = &value
	return b
}

// Waveform overrides operator waveform.
func (b *OperatorOverrideBuilder) Waveform(v uint8) *OperatorOverrideBuilder {
	if b == nil {
		return b
	}
	value := v
	b.value.Waveform = &value
	return b
}

// Build returns the constructed operator override.
func (b *OperatorOverrideBuilder) Build() voice.OperatorOverride {
	if b == nil {
		return voice.OperatorOverride{}
	}
	return cloneOperatorOverride(b.value)
}
