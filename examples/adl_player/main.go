// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

// adl_player plays Dune II ADL music files through OPL2 FM synthesis using the
// Nuked-OPL3 emulator, with real-time audio output via Ebiten.
//
// Usage:
//
//	go run ./examples/adl_player                          # plays DUNE1.ADL, auto-selects first music track
//	go run ./examples/adl_player path/to/DUNE9.ADL        # plays specified file
//	go run ./examples/adl_player path/to/DUNE9.ADL 3      # plays specified file, subsong 3
//
// Controls:
//
//	Left/Right arrows: previous/next subsong
//	Up/Down arrows:    previous/next ADL file (if using default directory)
//	Space:             pause/resume
//	R:                 restart current subsong
//	Q / Escape:        quit
package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/jebbisson/spice-synth/adl"
)

const sampleRate = 44100

// DebugStream wraps an adl.Player to monitor audio data flow.
type DebugStream struct {
	mu             sync.Mutex
	player         *adl.Player
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

// Game implements the ebiten.Game interface.
type Game struct {
	ds      *DebugStream
	player  *adl.Player
	status  string
	tickCnt int

	// File management.
	adlFiles []string // sorted list of ADL file paths
	fileIdx  int      // current index into adlFiles
	fileName string   // display name of current file

	// Subsong navigation (filtered to non-empty slots only).
	subsongs   []adl.SubsongInfo // non-empty subsongs for current file
	subsongIdx int               // current index into subsongs slice
}

func (g *Game) Update() error {
	// Handle input.
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		state := g.player.GetState()
		if state == adl.StatePlaying {
			g.player.Pause()
		} else {
			g.player.Play()
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.player.Stop()
		g.player.SetSubsong(g.currentSubsongIndex())
		g.player.Play()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		g.subsongIdx++
		if g.subsongIdx >= len(g.subsongs) {
			g.subsongIdx = 0
		}
		g.player.SetSubsong(g.currentSubsongIndex())
		if g.player.GetState() != adl.StatePlaying {
			g.player.Play()
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.subsongIdx--
		if g.subsongIdx < 0 {
			g.subsongIdx = len(g.subsongs) - 1
		}
		g.player.SetSubsong(g.currentSubsongIndex())
		if g.player.GetState() != adl.StatePlaying {
			g.player.Play()
		}
	}

	// File switching with Up/Down.
	if len(g.adlFiles) > 1 {
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			g.fileIdx--
			if g.fileIdx < 0 {
				g.fileIdx = len(g.adlFiles) - 1
			}
			g.loadFile(g.adlFiles[g.fileIdx])
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			g.fileIdx++
			if g.fileIdx >= len(g.adlFiles) {
				g.fileIdx = 0
			}
			g.loadFile(g.adlFiles[g.fileIdx])
		}
	}

	// Update status display.
	g.tickCnt++
	if g.tickCnt%15 == 0 {
		state := g.player.GetState()
		stateStr := "STOPPED"
		switch state {
		case adl.StatePlaying:
			stateStr = "PLAYING"
		case adl.StatePaused:
			stateStr = "PAUSED"
		case adl.StateDone:
			stateStr = "DONE"
		}

		info := g.currentSubsongInfo()
		typeLabel := info.Type.String()

		g.status = fmt.Sprintf(
			"SpiceSynth ADL Player\n\n"+
				"File: %s\n"+
				"[%s] Subsong: %d (%s) [%d/%d]\n"+
				"Volume: %.0f%% | Read: %d KB\n\n"+
				"Controls:\n"+
				"  Left/Right: prev/next subsong\n"+
				"  Up/Down:    prev/next file\n"+
				"  Space:      pause/resume\n"+
				"  R:          restart | Q: quit",
			g.fileName, stateStr,
			info.Index, typeLabel, g.subsongIdx+1, len(g.subsongs),
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
	for y := 55; y < 60; y++ {
		for x := 20; x < 20+barWidth; x++ {
			screen.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 320, 240
}

func (g *Game) loadFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("failed to open %s: %v", path, err)
		return
	}
	defer f.Close()

	af, err := adl.Parse(f)
	if err != nil {
		log.Printf("failed to parse %s: %v", path, err)
		return
	}

	g.player.Stop()
	g.player.Close()

	g.player = adl.NewPlayer(sampleRate, af)
	g.ds.player = g.player
	g.fileName = filepath.Base(path)

	// Build filtered subsong list.
	g.subsongs = af.NonEmptySubsongs()
	if len(g.subsongs) == 0 {
		// Fallback: include all subsongs if none are non-empty.
		g.subsongs = af.ClassifySubsongs()
	}

	// Default to first music subsong, or first non-empty.
	g.subsongIdx = 0
	for i, info := range g.subsongs {
		if info.Type == adl.SubsongMusic {
			g.subsongIdx = i
			break
		}
	}

	g.player.SetSubsong(g.currentSubsongIndex())
	g.player.Play()

	fmt.Printf("Loaded %s: v%d, %d subsongs (%d non-empty), playing subsong %d (%s)\n",
		g.fileName, af.Version, af.NumSubsongs, len(g.subsongs),
		g.currentSubsongIndex(), g.currentSubsongInfo().Type)
}

// currentSubsongIndex returns the real subsong index for the current filtered position.
func (g *Game) currentSubsongIndex() int {
	if g.subsongIdx < 0 || g.subsongIdx >= len(g.subsongs) {
		return 0
	}
	return g.subsongs[g.subsongIdx].Index
}

// currentSubsongInfo returns the SubsongInfo for the current filtered position.
func (g *Game) currentSubsongInfo() adl.SubsongInfo {
	if g.subsongIdx < 0 || g.subsongIdx >= len(g.subsongs) {
		return adl.SubsongInfo{}
	}
	return g.subsongs[g.subsongIdx]
}

// findADLFiles scans a directory for .ADL files and returns them sorted.
func findADLFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && (filepath.Ext(e.Name()) == ".ADL" || filepath.Ext(e.Name()) == ".adl") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files
}

func main() {
	// Parse args.
	adlPath := "../adl/DUNE1.ADL"
	startSubsong := -1 // -1 = auto-select first music track
	if len(os.Args) > 1 {
		adlPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil {
			startSubsong = n
		}
	}

	fmt.Println("--- SpiceSynth ADL Player ---")
	fmt.Printf("Host OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Parse the ADL file.
	f, err := os.Open(adlPath)
	if err != nil {
		log.Fatalf("failed to open ADL file: %v", err)
	}
	defer f.Close()

	af, err := adl.Parse(f)
	if err != nil {
		log.Fatalf("failed to parse ADL file: %v", err)
	}

	// Build filtered subsong list.
	subsongs := af.NonEmptySubsongs()
	if len(subsongs) == 0 {
		subsongs = af.ClassifySubsongs()
	}

	fmt.Printf("Loaded %s: v%d, %d programs, %d subsongs (%d non-empty)\n",
		filepath.Base(adlPath), af.Version, af.NumPrograms, af.NumSubsongs, len(subsongs))

	// Determine starting subsong index into the filtered list.
	subsongIdx := 0
	if startSubsong >= 0 {
		// User specified a raw subsong number — find it in the filtered list.
		for i, info := range subsongs {
			if info.Index == startSubsong {
				subsongIdx = i
				break
			}
		}
	} else {
		// Auto-select first music track.
		for i, info := range subsongs {
			if info.Type == adl.SubsongMusic {
				subsongIdx = i
				break
			}
		}
	}

	// Create player.
	p := adl.NewPlayer(sampleRate, af)
	p.SetSubsong(subsongs[subsongIdx].Index)

	ds := &DebugStream{player: p}

	// Setup Ebiten audio.
	audioCtx := audio.NewContext(sampleRate)
	ap, err := audioCtx.NewPlayer(ds)
	if err != nil {
		log.Fatalf("failed to create audio player: %v", err)
	}
	ap.SetBufferSize(time.Millisecond * 100)
	ap.Play()

	// Start playback.
	p.Play()
	fmt.Printf("Playing subsong %d (%s) at %d Hz... close window to stop.\n",
		subsongs[subsongIdx].Index, subsongs[subsongIdx].Type, sampleRate)

	// Find all ADL files in the same directory for file switching.
	adlDir := filepath.Dir(adlPath)
	adlFiles := findADLFiles(adlDir)
	fileIdx := 0
	for i, fp := range adlFiles {
		abs1, _ := filepath.Abs(fp)
		abs2, _ := filepath.Abs(adlPath)
		if abs1 == abs2 {
			fileIdx = i
			break
		}
	}

	g := &Game{
		ds:         ds,
		player:     p,
		status:     "Initializing...",
		adlFiles:   adlFiles,
		fileIdx:    fileIdx,
		fileName:   filepath.Base(adlPath),
		subsongs:   subsongs,
		subsongIdx: subsongIdx,
	}

	// Console logging.
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			state := p.GetState()
			stateStr := "STOP"
			switch state {
			case adl.StatePlaying:
				stateStr = "PLAY"
			case adl.StatePaused:
				stateStr = "PAUS"
			case adl.StateDone:
				stateStr = "DONE"
			}
			info := g.currentSubsongInfo()
			fmt.Printf("[%s] %s | sub:%d (%s) | vol:%.0f%%\n",
				time.Now().Format("15:04:05"),
				stateStr,
				info.Index,
				info.Type,
				ds.getVolume()*100,
			)
		}
	}()

	ebiten.SetWindowTitle("SpiceSynth ADL Player - Dune II")
	ebiten.SetWindowSize(320, 240)
	if err := ebiten.RunGame(g); err != nil {
		if err != ebiten.Termination {
			log.Fatal(err)
		}
	}

	p.Close()
	fmt.Println("Goodbye.")
}
