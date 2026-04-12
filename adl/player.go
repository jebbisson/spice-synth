// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.
//
// Ported from AdPlug (https://github.com/adplug/adplug), original code by
// Torbjorn Andersson and Johannes Schickel of the ScummVM project.
// Original code is LGPL-2.1. See THIRD_PARTY_LICENSES for details.

package adl

import (
	"io"
	"sync"

	"github.com/jebbisson/spice-synth/chip"
)

// Player state constants.
const (
	StateStopped = iota
	StatePlaying
	StatePaused
	StateDone
)

// Player renders an ADL file through OPL2 FM synthesis. It drives the
// bytecode interpreter at 72Hz and exposes a standard io.Reader for
// signed 16-bit stereo little-endian PCM audio data.
type Player struct {
	mu sync.Mutex

	sampleRate int
	file       *File
	opl        *chip.OPL3
	driver     *Driver

	state      int
	curSubsong int

	// Timing: samples per 72Hz callback tick.
	samplesPerTick  float64
	tickAccumulator float64

	// Audio output mixing.
	masterVol float64
	gain      float64
}

// NewPlayer creates a new ADL player from a parsed ADL file. The player
// owns the OPL3 chip instance and will close it when Close() is called.
func NewPlayer(sampleRate int, file *File) *Player {
	opl := chip.New(uint32(sampleRate))
	driver := NewDriver(opl)
	driver.SetVersion(file.Version)
	driver.SetSoundData(file.SoundData)
	driver.InitDriver()

	return &Player{
		sampleRate:     sampleRate,
		file:           file,
		opl:            opl,
		driver:         driver,
		state:          StateStopped,
		samplesPerTick: float64(sampleRate) / float64(callbacksPerSecond),
		masterVol:      1.0,
		gain:           4.0,
	}
}

// Play starts or resumes playback. If stopped, starts from the current
// subsong (defaults to subsong 2, matching the original player behavior).
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state == StateStopped {
		p.rewindLocked(p.curSubsong)
	}
	if p.state == StateStopped || p.state == StatePaused || p.state == StateDone {
		p.state = StatePlaying
	}
}

// Pause pauses playback. Audio output continues as silence.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StatePlaying {
		p.state = StatePaused
	}
}

// Stop stops playback and resets to the beginning.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = StateStopped
	p.driver.StopAllChannels()
	p.opl.Reset()
	p.driver.InitDriver()
	p.tickAccumulator = 0
}

// GetState returns the current player state.
func (p *Player) GetState() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// SetSubsong selects a subsong by index and restarts playback.
func (p *Player) SetSubsong(subsong int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.curSubsong = subsong
	if p.state == StatePlaying || p.state == StateDone {
		p.rewindLocked(subsong)
		p.state = StatePlaying
	}
}

// NumSubsongs returns the number of available subsongs.
func (p *Player) NumSubsongs() int {
	if p.file == nil {
		return 0
	}
	return p.file.NumSubsongs
}

// CurrentSubsong returns the currently selected subsong index.
func (p *Player) CurrentSubsong() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.curSubsong
}

// SetMasterVolume sets the master output volume (0.0 - 1.0).
func (p *Player) SetMasterVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.masterVol = v
}

// SetGain sets the output gain multiplier.
func (p *Player) SetGain(g float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gain = g
}

// Close releases all resources. The Player must not be used after calling Close.
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = StateStopped
	if p.opl != nil {
		p.opl.Close()
		p.opl = nil
	}
}

// Read fills b with signed 16-bit stereo little-endian PCM data.
// This implements io.Reader for use with audio output libraries.
func (p *Player) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	totalFrames := len(b) / 4
	if totalFrames == 0 {
		return 0, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	byteOffset := 0
	framesLeft := totalFrames

	for framesLeft > 0 {
		// Determine how many frames until the next 72Hz tick.
		framesToTick := int(p.samplesPerTick - p.tickAccumulator)
		if framesToTick < 1 {
			framesToTick = 1
		}
		n := framesToTick
		if n > framesLeft {
			n = framesLeft
		}

		// Generate OPL audio.
		samples, err := p.opl.GenerateSamples(n)
		if err != nil {
			return byteOffset, err
		}

		// Apply gain and volume, write to output buffer.
		for i := 0; i < n*2; i++ {
			scaled := float64(samples[i]) * p.gain * p.masterVol
			if scaled > 32767 {
				scaled = 32767
			} else if scaled < -32768 {
				scaled = -32768
			}
			out := int16(scaled)
			idx := byteOffset + i*2
			if idx+1 < len(b) {
				b[idx] = byte(out & 0xFF)
				b[idx+1] = byte(out >> 8)
			}
		}

		byteOffset += n * 4
		framesLeft -= n

		// Advance the tick accumulator.
		p.tickAccumulator += float64(n)
		if p.tickAccumulator >= p.samplesPerTick {
			p.tickAccumulator -= p.samplesPerTick

			if p.state == StatePlaying {
				// Run one 72Hz callback tick.
				p.driver.Callback()

				// Check if song ended (all channels stopped or repeating).
				if !p.isPlaying() {
					p.state = StateDone
				}
			}
		}
	}

	return totalFrames * 4, nil
}

// Ensure Player implements io.Reader.
var _ io.Reader = (*Player)(nil)

// --- Internal ---

// rewindLocked resets the driver and starts the given subsong.
func (p *Player) rewindLocked(subsong int) {
	p.driver.StopAllChannels()
	p.opl.Reset()
	p.driver.InitDriver()
	p.tickAccumulator = 0

	if subsong >= p.file.NumSubsongs {
		subsong = 0
	}
	if subsong < 0 {
		subsong = 0
	}
	p.curSubsong = subsong

	// Look up track and start it.
	trackID := p.file.TrackForSubsong(subsong)
	if trackID >= 0 {
		p.driver.StartSound(trackID, 0xFF)
	}
}

// isPlaying returns true if any channel is actively playing (not stopped
// and not in a repeating loop).
func (p *Player) isPlaying() bool {
	for i := 0; i < 10; i++ {
		if p.driver.IsChannelPlaying(i) && !p.driver.IsChannelRepeating(i) {
			return true
		}
	}
	return false
}
