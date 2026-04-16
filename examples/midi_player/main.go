// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

// midi_player plays a MIDI file through OPL2 FM synthesis using the embedded
// DMXOPL General MIDI bank and the Nuked-OPL3 emulator, with real-time audio
// output via Ebiten.
//
// Usage: go run ./examples/midi_player
package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/jebbisson/spice-synth/midi"
	"github.com/jebbisson/spice-synth/op2"
	"github.com/jebbisson/spice-synth/player"
)

const sampleRate = 44100

// DebugStream wraps a player.Player to monitor audio data flow.
type DebugStream struct {
	mu             sync.Mutex
	player         *player.Player
	TotalBytesRead int64
	CurrentVolume  float64
}

func (d *DebugStream) Read(b []byte) (int, error) {
	n, err := d.player.Read(b)
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

type Game struct {
	ds      *DebugStream
	p       *player.Player
	status  string
	tickCnt int
}

func (g *Game) Update() error {
	g.tickCnt++
	if g.tickCnt%30 == 0 {
		pos := g.p.PositionSeconds()
		dur := g.p.DurationSeconds()
		chips := g.p.ActiveChips()
		voices := g.p.ActiveVoices()
		state := g.p.GetState()

		stateStr := "STOPPED"
		switch state {
		case player.StatePlaying:
			stateStr = "PLAYING"
		case player.StatePaused:
			stateStr = "PAUSED"
		case player.StateDone:
			stateStr = "DONE"
		}

		g.status = fmt.Sprintf(
			"[%s] %.1f / %.1f sec\nChips: %d | Voices: %d\nVolume: %.1f%% | Read: %d KB",
			stateStr, pos, dur, chips, voices,
			g.ds.getVolume()*100,
			g.ds.getBytesRead()/1024,
		)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, g.status)

	// Volume bar.
	barWidth := int(g.ds.getVolume() * 200)
	if barWidth > 200 {
		barWidth = 200
	}
	for y := 80; y < 85; y++ {
		for x := 0; x < barWidth; x++ {
			screen.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}

	// Progress bar.
	pos := g.p.PositionSeconds()
	dur := g.p.DurationSeconds()
	if dur > 0 {
		progWidth := int((pos / dur) * 200)
		if progWidth > 200 {
			progWidth = 200
		}
		for y := 90; y < 95; y++ {
			for x := 0; x < progWidth; x++ {
				screen.Set(x, y, color.RGBA{0, 128, 255, 255})
			}
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

func main() {
	// Default MIDI file path.
	midiPath := "../midi/Title.mid"
	if len(os.Args) > 1 {
		midiPath = os.Args[1]
	}

	// 1. Load the OP2 instrument bank.
	fmt.Println("--- SpiceSynth MIDI Player ---")
	fmt.Printf("Host OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	bank, err := op2.DefaultBank()
	if err != nil {
		log.Fatalf("failed to load bank: %v", err)
	}
	fmt.Println("Loaded DMXOPL GM bank (175 instruments)")

	// 2. Parse the MIDI file.
	f, err := os.Open(midiPath)
	if err != nil {
		log.Fatalf("failed to open MIDI file: %v", err)
	}
	defer f.Close()

	mf, err := midi.Parse(f)
	if err != nil {
		log.Fatalf("failed to parse MIDI file: %v", err)
	}

	totalEvents := 0
	for _, track := range mf.Tracks {
		totalEvents += len(track.Events)
	}
	fmt.Printf("Loaded %s: format %d, %d tracks, %d events, %.1f sec\n",
		midiPath, mf.Format, len(mf.Tracks), totalEvents, mf.Duration())

	// 3. Create the player.
	p := player.New(sampleRate, bank, mf)
	defer p.Close()

	ds := &DebugStream{player: p}

	// 4. Setup Ebiten audio.
	audioCtx := audio.NewContext(sampleRate)
	ap, err := audioCtx.NewPlayer(ds)
	if err != nil {
		log.Fatalf("failed to create audio player: %v", err)
	}
	ap.SetBufferSize(time.Millisecond * 100)
	ap.Play()

	// 5. Start playback.
	p.Play()
	fmt.Printf("Playing at %d Hz... close window to stop.\n", sampleRate)

	g := &Game{
		ds:     ds,
		p:      p,
		status: "Initializing...",
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			pos := p.PositionSeconds()
			dur := p.DurationSeconds()
			chips := p.ActiveChips()
			voices := p.ActiveVoices()
			fmt.Printf("[%s] %.1f/%.1f sec | chips: %d | voices: %d | vol: %.1f%%\n",
				time.Now().Format("15:04:05"),
				pos, dur, chips, voices,
				ds.getVolume()*100,
			)
		}
	}()

	ebiten.SetWindowTitle("SpiceSynth MIDI Player")
	ebiten.SetWindowSize(320, 240)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
