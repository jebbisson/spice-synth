// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// ebiten_player recreates the Hope Fades (Dune II) intro arrangement using
// the Strudel-inspired DSL API with real-time audio playback via Ebiten.
//
// Three layers:
//   - BASS:   Deep A0 with 5s fade-in ramp + slow sine wobble
//   - WIND:   Sustained desert_wind texture, 8s fade-in, very quiet
//   - CHIMES: Bright spice_chime hits (single held note for now)
//
// Usage: go run ./examples/ebiten_player
package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/stream"
	"github.com/jebbisson/spice-synth/voice"
)

const (
	sampleRate = 44100
	bpm        = 55 // Slow, brooding tempo for Hope Fades intro
)

// DebugStream wraps a stream.Stream to monitor audio data flow and volume.
type DebugStream struct {
	*stream.Stream
	TotalBytesRead int64
	CurrentVolume  float64 // Peak amplitude in the last Read call
}

func (d *DebugStream) Read(b []byte) (int, error) {
	n, err := d.Stream.Read(b)
	if n > 0 {
		d.TotalBytesRead += int64(n)

		// Calculate peak amplitude for visualization (S16LE)
		var maxAmp float64
		for i := 0; i < n; i += 2 {
			sample := int16(b[i]) | int16(b[i+1])<<8
			absVal := math.Abs(float64(sample))
			if absVal > maxAmp {
				maxAmp = absVal
			}
		}
		d.CurrentVolume = maxAmp / 32768.0 // Normalize to 0.0 - 1.0
	}
	return n, err
}

type Game struct {
	ds      *DebugStream
	status  string
	tickCnt int
}

func (g *Game) Update() error {
	g.tickCnt++
	if g.tickCnt%60 == 0 {
		g.status = fmt.Sprintf(
			"Audio Flow: %s | Volume: %.2f%% | Total Read: %d KB",
			func() string {
				if g.ds.CurrentVolume > 0 {
					return "ACTIVE"
				}
				return "SILENT"
			}(),
			g.ds.CurrentVolume*100,
			g.ds.TotalBytesRead/1024,
		)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, g.status)

	barWidth := int(g.ds.CurrentVolume * 200)
	if barWidth > 200 {
		barWidth = 200
	}
	for i := 0; i < barWidth; i++ {
		screen.Set(i, 50, color.RGBA{0, 255, 0, 255})
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

func main() {
	// 1. Initialize the synth stream.
	s := stream.New(sampleRate)
	defer s.Close()
	ds := &DebugStream{Stream: s}
	s.Voices().LoadBank("spice", spiceBank())
	s.Sequencer().SetBPM(bpm)

	// --- Hope Fades (Dune II) intro arrangement (DSL) ---
	//
	// Layer 1: Bass — deep A0 that fades in from silence over 5 seconds,
	// then holds with a slow sine wobble. The ramp and gain signal are
	// multiplied together by the modulator system.
	bass := dsl.Note("A0").S("desert_bass").
		Ramp(0.0, 1.0, 5.0).
		GainSignal(dsl.Sine().Range(0.3, 1.0).Slow(4))

	// Layer 2: Wind — sustained noise texture, 8s fade-in, quiet drift.
	wind := dsl.Note("C3").S("desert_wind").
		Ramp(0.0, 1.0, 8.0).
		GainSignal(dsl.Tri().Range(0.1, 0.55).Slow(6.67))

	// Layer 3: Chimes — bright metallic hit.
	// (Multi-note sequenced patterns coming in Phase 4 of the DSL.)
	chimes := dsl.Note("E5").S("spice_chime")

	// 2. Play each layer on a separate OPL2 channel.
	if err := bass.Play(s, 0); err != nil {
		log.Fatalf("bass: %v", err)
	}
	if err := wind.Play(s, 1); err != nil {
		log.Fatalf("wind: %v", err)
	}
	if err := chimes.Play(s, 2); err != nil {
		log.Fatalf("chimes: %v", err)
	}

	// 3. Setup Ebiten audio.
	fmt.Println("--- SpiceSynth DSL Mode ---")
	fmt.Printf("Host OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("CPU Cores: %d\n", runtime.NumCPU())
	audioCtx := audio.NewContext(sampleRate)

	player, err := audioCtx.NewPlayer(ds)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to create audio player: %v", err)
	}

	player.SetBufferSize(time.Millisecond * 100)
	player.Play()

	fmt.Printf("Audio Context initialized at %dHz\n", sampleRate)
	fmt.Println("Playing arrangement... close window to stop.")

	g := &Game{
		ds:     ds,
		status: "Initializing...",
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			fmt.Printf("[%s] Vol: %.2f%% | Read: %d KB\n",
				time.Now().Format("15:04:05"),
				ds.CurrentVolume*100,
				ds.TotalBytesRead/1024,
			)
		}
	}()

	ebiten.SetWindowTitle("SpiceSynth DSL — Hope Fades")
	ebiten.SetWindowSize(320, 240)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func spiceBank() []*voice.Instrument {
	return []*voice.Instrument{
		{
			Name:       "desert_bass",
			Op1:        voice.Operator{Attack: 15, Decay: 4, Sustain: 2, Release: 6, Level: 18, Multiply: 1, Waveform: 0, Sustaining: true},
			Op2:        voice.Operator{Attack: 15, Decay: 3, Sustain: 1, Release: 8, Level: 0, Multiply: 1, Waveform: 0, Sustaining: true},
			Feedback:   6,
			Connection: 0,
		},
		{
			Name:       "desert_wind",
			Op1:        voice.Operator{Attack: 12, Decay: 4, Sustain: 2, Release: 10, Level: 20, Multiply: 7, Waveform: 0, Sustaining: true},
			Op2:        voice.Operator{Attack: 10, Decay: 3, Sustain: 1, Release: 12, Level: 0, Multiply: 1, Waveform: 0, Sustaining: true},
			Feedback:   7,
			Connection: 0,
		},
		{
			Name:       "spice_chime",
			Op1:        voice.Operator{Attack: 15, Decay: 6, Sustain: 6, Release: 6, Level: 22, Multiply: 3, Waveform: 1, Sustaining: false},
			Op2:        voice.Operator{Attack: 15, Decay: 7, Sustain: 10, Release: 7, Level: 0, Multiply: 4, Waveform: 0, Sustaining: false},
			Feedback:   2,
			Connection: 0,
		},
	}
}
