// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

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
	"github.com/jebbisson/spice-synth/patches"
	"github.com/jebbisson/spice-synth/sequencer"
	"github.com/jebbisson/spice-synth/stream"
)

const (
	sampleRate = 44100
	bpm        = 125
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
	seq     *sequencer.Sequencer
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
	// Background status
	ebitenutil.DebugPrint(screen, g.status)

	// Simple Visualizer Bar
	barWidth := int(g.ds.CurrentVolume * 200)
	if barWidth > 200 {
		barWidth = 200
	}

	// Draw a simple line as a volume meter
	for i := 0; i < barWidth; i++ {
		screen.Set(i, 50, color.RGBA{0, 255, 0, 255}) // Green bar
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

var startTime time.Time

func main() {
	startTime = time.Now()

	// 1. Initialize the synth stream and wrap it for debugging
	s := stream.New(sampleRate)
	ds := &DebugStream{Stream: s}
	seq := s.Sequencer()
	seq.SetBPM(bpm)

	s.Voices().LoadBank("spice", patches.Spice())

	// Arrangement
	bass := sequencer.NewPattern(16).
		Instrument("desert_bass").
		Note(0, "C2").Note(3, "C2").Note(6, "G1").Note(10, "C2").Note(14, "Eb2")

	lead := sequencer.NewPattern(16).
		Instrument("mystic_lead").
		Note(0, "G3").Note(4, "Ab3").Note(8, "G3").Note(12, "F3")

	perc := sequencer.NewPattern(16).
		Instrument("fm_perc").
		Hit(0).Hit(4).Hit(8).Hit(12)

	seq.SetPattern(0, bass)
	seq.SetPattern(1, lead)
	seq.SetPattern(2, perc)

	// 4. Setup Ebiten Audio
	fmt.Println("--- SpiceSynth Debug Mode ---")
	fmt.Printf("Host OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("CPU Cores: %d\n", runtime.NumCPU())
	audioCtx := audio.NewContext(sampleRate)

	// IMPORTANT: We pass the DebugStream wrapper here
	player, err := audioCtx.NewPlayer(ds)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to create audio player: %v", err)
	}

	player.SetBufferSize(time.Millisecond * 100)
	player.Play()

	fmt.Printf("Audio Context initialized at %dHz\n", sampleRate)
	fmt.Println("Playing arrangement... check window for visualizer.")
	fmt.Println("If 'Audio Flow' stays SILENT, the stream is not being read by Ebiten.")

	g := &Game{
		ds:     ds,
		seq:    seq,
		status: "Initializing...",
	}

	// Start a goroutine to print stats to console every second
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			fmt.Printf("[%s] Vol: %.2f%% | Bytes Read: %d KB | Status: %s\n",
				time.Now().Format("15:04:05"),
				ds.CurrentVolume*100,
				ds.TotalBytesRead/1024,
				func() string {
					if ds.CurrentVolume > 0 {
						return "🔊 SOUNDING"
					}
					return "🔇 SILENT"
				}(),
			)
		}
	}()

	ebiten.SetWindowTitle("SpiceSynth Debugger")
	ebiten.SetWindowSize(320, 240)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
