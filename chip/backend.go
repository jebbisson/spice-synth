// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package chip

// BackendFactory constructs an OPL backend for a given sample rate.
type BackendFactory func(sampleRate int) Backend

// Backend defines the minimal OPL engine surface the rest of the project uses.
// Concrete backends can remain in-tree for now and later move into separate
// repos without forcing higher-level packages to change shape.
type Backend interface {
	Reset()
	Close()
	WriteRegister(port uint16, reg uint8, val uint8)
	WriteRegisterBuffered(port uint16, reg uint8, val uint8)
	GenerateSamples(n int) ([]int16, error)
	GenerateSamplesWithMeters(n int) ([]int16, []uint16, error)
	SetSoloChannel(ch int)
	SoloChannel() int
	ChannelMeter(ch int) float64
}

var backendFactory BackendFactory = func(sampleRate int) Backend {
	return New(uint32(sampleRate))
}

// NewBackend constructs a backend using the currently configured factory.
func NewBackend(sampleRate int) Backend {
	return backendFactory(sampleRate)
}

// SetBackendFactory overrides the backend constructor used by higher-level
// packages. Passing nil restores the in-tree default implementation.
func SetBackendFactory(factory BackendFactory) {
	if factory == nil {
		backendFactory = func(sampleRate int) Backend {
			return New(uint32(sampleRate))
		}
		return
	}
	backendFactory = factory
}
