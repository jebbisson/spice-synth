// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package adl

import (
	adplugadl "github.com/jebbisson/spice-adl-adplug"
	"github.com/jebbisson/spice-synth/chip"
)

// RuntimeFactory constructs an ADL runtime bound to an OPL backend.
type RuntimeFactory func(opl chip.Backend) Runtime

// Runtime is the minimal ADL playback engine surface used by Player. The
// current Driver remains the default implementation and can later move behind a
// separate LGPL wrapper repo without changing the player API.
type Runtime interface {
	InitDriver()
	SetVersion(v int)
	SetSoundData(data []byte)
	StartSound(track int, volume uint8)
	StopAllChannels()
	Callback()
	IsChannelPlaying(ch int) bool
	IsChannelRepeating(ch int) bool
	SnapshotChannels() []ChannelState
	SetTraceFunc(fn func(format string, args ...interface{}))
}

type externalRuntime struct {
	inner *adplugadl.Driver
}

func (r externalRuntime) InitDriver() {
	r.inner.InitDriver()
}

func (r externalRuntime) SetVersion(v int) {
	r.inner.SetVersion(v)
}

func (r externalRuntime) SetSoundData(data []byte) {
	r.inner.SetSoundData(data)
}

func (r externalRuntime) StartSound(track int, volume uint8) {
	r.inner.StartSound(track, volume)
}

func (r externalRuntime) StopAllChannels() {
	r.inner.StopAllChannels()
}

func (r externalRuntime) Callback() {
	r.inner.Callback()
}

func (r externalRuntime) IsChannelPlaying(ch int) bool {
	return r.inner.IsChannelPlaying(ch)
}

func (r externalRuntime) IsChannelRepeating(ch int) bool {
	return r.inner.IsChannelRepeating(ch)
}

func (r externalRuntime) SnapshotChannels() []ChannelState {
	states := r.inner.SnapshotChannels()
	out := make([]ChannelState, len(states))
	for i, state := range states {
		out[i] = ChannelState{
			Channel:            state.Channel,
			BytecodeActive:     state.BytecodeActive,
			KeyOn:              state.KeyOn,
			Repeating:          state.Repeating,
			Releasing:          state.Releasing,
			ControlChannel:     state.ControlChannel,
			InstrumentID:       state.InstrumentID,
			RawNote:            state.RawNote,
			Note:               state.Note,
			FrequencyHz:        state.FrequencyHz,
			Duration:           state.Duration,
			InitialDuration:    state.InitialDuration,
			Spacing1:           state.Spacing1,
			Spacing2:           state.Spacing2,
			VolumeModifier:     state.VolumeModifier,
			OutputLevel:        state.OutputLevel,
			CarrierLevel:       state.CarrierLevel,
			ModulatorLevel:     state.ModulatorLevel,
			TwoOperatorCarrier: state.TwoOperatorCarrier,
			Dataptr:            state.Dataptr,
		}
	}
	return out
}

func (r externalRuntime) SetTraceFunc(fn func(format string, args ...interface{})) {
	r.inner.SetTraceFunc(fn)
}

var runtimeFactory RuntimeFactory = func(opl chip.Backend) Runtime {
	return externalRuntime{inner: adplugadl.NewDriver(opl)}
}

// NewRuntime constructs a runtime using the currently configured factory.
func NewRuntime(opl chip.Backend) Runtime {
	return runtimeFactory(opl)
}

// SetRuntimeFactory overrides the runtime constructor used by Player. Passing
// nil restores the in-tree driver implementation.
func SetRuntimeFactory(factory RuntimeFactory) {
	if factory == nil {
		runtimeFactory = func(opl chip.Backend) Runtime {
			return externalRuntime{inner: adplugadl.NewDriver(opl)}
		}
		return
	}
	runtimeFactory = factory
}
