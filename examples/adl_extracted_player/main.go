// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// adl_extracted_player plays a song that was extracted from an ADL file using
// the adl2dsl converter tool. The extracted song uses the DSL Song/Track API
// to drive the OPL2 chip through spice-synth's voice manager and sequencer.
//
// This example serves as a template: to play a different extracted song,
// replace song.go with the output of adl2dsl and update the call in main().
//
// Usage:
//
//	go run ./examples/adl_extracted_player
//
// Controls:
//
//	Q / Escape: quit
package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/jebbisson/spice-synth/stream"
)

const sampleRate = 44100

// DebugStream wraps a stream.Stream to monitor audio output.
type DebugStream struct {
	mu     sync.Mutex
	stream *stream.Stream

	TotalBytesRead int64
	CurrentVolume  float64
}

func (d *DebugStream) Read(b []byte) (int, error) {
	n, err := d.stream.Read(b)
	if n > 0 {
		d.mu.Lock()
		d.TotalBytesRead += int64(n)

		var maxAmp float64
		for i := 0; i < n-1; i += 2 {
			sample := int16(b[i]) | int16(b[i+1])<<8
			absVal := math.Abs(float64(sample))
			if absVal > maxAmp {
				maxAmp = absVal
			}
		}
		d.CurrentVolume = maxAmp / 32768.0
		d.mu.Unlock()
	}
	return n, err
}

func (d *DebugStream) getVolume() float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.CurrentVolume
}

func (d *DebugStream) getBytesRead() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.TotalBytesRead
}

// Game implements the ebiten.Game interface.
type Game struct {
	ds      *DebugStream
	status  string
	tickCnt int
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	g.tickCnt++
	if g.tickCnt%15 == 0 {
		g.status = fmt.Sprintf(
			"SpiceSynth ADL Extracted Player\n\n"+
				"Song: DUNE1.ADL subsong 2\n"+
				"Source: adl2dsl converter\n\n"+
				"Volume: %.0f%% | Read: %d KB\n\n"+
				"Controls:\n"+
				"  Q / Escape: quit",
			g.ds.getVolume()*100,
			g.ds.getBytesRead()/1024,
		)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, g.status)

	// Volume bar.
	barWidth := int(g.ds.getVolume() * 280)
	if barWidth > 280 {
		barWidth = 280
	}
	for y := 72; y < 77; y++ {
		for x := 20; x < 20+barWidth; x++ {
			screen.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

func main() {
	fmt.Println("--- SpiceSynth ADL Extracted Player ---")
	fmt.Printf("Host OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// 1. Create the audio stream.
	s := stream.New(sampleRate)

	// 2. Load and play the extracted song.
	//    ---------------------------------------------------------------
	//    To play a different extracted song, replace song.go with the
	//    output of: adl2dsl -package main -output song.go <your.ADL>
	//    Then update the function call below to match.
	//    ---------------------------------------------------------------
	if err := PlayDUNE1(s); err != nil {
		log.Fatalf("failed to play song: %v", err)
	}

	ds := &DebugStream{stream: s}

	// 3. Setup Ebiten audio.
	audioCtx := audio.NewContext(sampleRate)
	player, err := audioCtx.NewPlayer(ds)
	if err != nil {
		log.Fatalf("failed to create audio player: %v", err)
	}
	player.SetBufferSize(time.Millisecond * 100)
	player.Play()

	fmt.Printf("Audio initialized at %d Hz\n", sampleRate)
	fmt.Println("Playing extracted song... close window or press Q to stop.")

	// 4. Console volume logging.
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			fmt.Printf("[%s] Vol: %.0f%% | Read: %d KB\n",
				time.Now().Format("15:04:05"),
				ds.getVolume()*100,
				ds.getBytesRead()/1024,
			)
		}
	}()

	// 5. Run the Ebiten window.
	g := &Game{
		ds:     ds,
		status: "Initializing...",
	}

	ebiten.SetWindowTitle("SpiceSynth - ADL Extracted Song")
	ebiten.SetWindowSize(320, 240)
	if err := ebiten.RunGame(g); err != nil {
		if err != ebiten.Termination {
			log.Fatal(err)
		}
	}

	s.Close()
	fmt.Println("Goodbye.")
}
