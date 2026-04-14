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
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/jebbisson/spice-synth/adl"
)

const (
	sampleRate   = 44100
	windowWidth  = 920
	windowHeight = 760
	maxPartRows  = 14
)

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

type partView struct {
	InstrumentID    int
	Note            string
	RawVolume       float64
	DisplayVolume   float64
	Remaining       float64
	RemainingMin    float64
	RemainingMax    float64
	Voices          int
	Channels        []int
	Releasing       int
	KeyOnCount      int
	StrongestVolume float64
	Summary         string
	VariantSummary  string
	SortKey         string
}

// Game implements the ebiten.Game interface.
type Game struct {
	ds      *DebugStream
	player  *adl.Player
	status  string
	tickCnt int

	adlFiles []string
	fileIdx  int
	fileName string

	subsongs   []adl.SubsongInfo
	subsongIdx int

	parts       []partView
	soloChannel int
}

func (g *Game) Update() error {
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

	g.updateSoloChannel()

	g.parts = buildPartViews(g.player.ChannelStates())

	g.tickCnt++
	if g.tickCnt%10 == 0 {
		g.status = g.buildStatus()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	ebitenutil.DebugPrintAt(screen, g.status, 16, 16)

	masterVolume := g.ds.getVolume()
	drawMeter(screen, 16, 110, 360, 12, masterVolume, color.RGBA{72, 220, 120, 255}, color.RGBA{36, 52, 42, 255})
	ebitenutil.DebugPrintAt(screen, "Master Output", 16, 92)

	headY := 146
	ebitenutil.DebugPrintAt(screen, "Active Parts grouped by instrument + note", 16, headY)
	ebitenutil.DebugPrintAt(screen, "Each row merges matching note/instrument voices and lists contributing channels.", 16, headY+16)

	rowTop := headY + 44
	rowHeight := 38
	for i, part := range g.parts {
		if i >= maxPartRows {
			break
		}
		y := rowTop + i*rowHeight
		drawPartRow(screen, y, part, g.soloChannel)
	}

	if len(g.parts) == 0 {
		ebitenutil.DebugPrintAt(screen, "No active melodic parts yet. Wait for playback to trigger voices.", 16, rowTop)
	}

	if len(g.parts) > maxPartRows {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("... %d more parts not shown", len(g.parts)-maxPartRows), 16, rowTop+maxPartRows*rowHeight+6)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return windowWidth, windowHeight
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

	g.subsongs = af.NonEmptySubsongs()
	if len(g.subsongs) == 0 {
		g.subsongs = af.ClassifySubsongs()
	}

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

func (g *Game) currentSubsongIndex() int {
	if g.subsongIdx < 0 || g.subsongIdx >= len(g.subsongs) {
		return 0
	}
	return g.subsongs[g.subsongIdx].Index
}

func (g *Game) currentSubsongInfo() adl.SubsongInfo {
	if g.subsongIdx < 0 || g.subsongIdx >= len(g.subsongs) {
		return adl.SubsongInfo{}
	}
	return g.subsongs[g.subsongIdx]
}

func (g *Game) buildStatus() string {
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
	activeVoices := 0
	for _, part := range g.parts {
		activeVoices += part.Voices
	}

	soloStr := "all"
	if g.soloChannel >= 0 {
		soloStr = fmt.Sprintf("ch%d", g.soloChannel)
	}

	return fmt.Sprintf(
		"SpiceSynth ADL Player\n\n"+
			"File: %s\n"+
			"[%s] Subsong: %d (%s) [%d/%d]\n"+
			"Master: %.0f%% | Active parts: %d | Voices: %d | Solo: %s | Read: %d KB\n\n"+
			"Controls: hold 0-9 solo channel | Left/Right subsong | Up/Down file | Space pause | R restart | Q quit",
		g.fileName,
		stateStr,
		info.Index,
		typeLabel,
		g.subsongIdx+1,
		len(g.subsongs),
		g.ds.getVolume()*100,
		len(g.parts),
		activeVoices,
		soloStr,
		g.ds.getBytesRead()/1024,
	)
}

func (g *Game) updateSoloChannel() {
	solo := heldSoloChannel()
	if solo == g.soloChannel {
		return
	}
	g.soloChannel = solo
	g.player.SetSoloChannel(solo)
}

func drawPartRow(screen *ebiten.Image, y int, part partView, soloChannel int) {
	highlighted := soloChannel < 0 || containsChannel(part.Channels, soloChannel)
	lineColor := color.RGBA{42, 48, 58, 255}
	textColor := color.RGBA{255, 255, 255, 255}
	volFill := color.RGBA{100, 210, 255, 255}
	timeFill := color.RGBA{255, 186, 79, 255}
	volBg := color.RGBA{30, 48, 60, 255}
	timeBg := color.RGBA{60, 48, 20, 255}
	if !highlighted {
		lineColor = color.RGBA{28, 31, 38, 255}
		textColor = color.RGBA{168, 172, 180, 255}
		volFill = color.RGBA{52, 86, 104, 255}
		timeFill = color.RGBA{110, 84, 40, 255}
		volBg = color.RGBA{20, 28, 34, 255}
		timeBg = color.RGBA{34, 29, 18, 255}
	}
	for x := 16; x < windowWidth-16; x++ {
		screen.Set(x, y-4, lineColor)
	}

	label := fmt.Sprintf("inst %03d  %-4s  x%d  ch:%s", part.InstrumentID, part.Note, part.Voices, joinChannels(part.Channels))
	if part.Releasing > 0 {
		label += fmt.Sprintf("  rel:%d", part.Releasing)
	}
	drawText(screen, 16, y, label, textColor)
	drawText(screen, 330, y, part.Summary, textColor)
	drawText(screen, 650, y, part.VariantSummary, textColor)

	drawMeter(screen, 24, y+18, 300, 8, part.DisplayVolume, volFill, volBg)
	drawMeter(screen, 340, y+18, 300, 8, part.Remaining, timeFill, timeBg)
	drawText(screen, 24, y+24, fmt.Sprintf("vol %3.0f%% raw %4.1f%%", part.DisplayVolume*100, part.RawVolume*100), textColor)
	drawText(screen, 340, y+24, fmt.Sprintf("time %3.0f%%", part.Remaining*100), textColor)
}

func drawMeter(screen *ebiten.Image, x, y, width, height int, value float64, fill, bg color.RGBA) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			screen.Set(x+px, y+py, bg)
		}
	}
	filled := int(value * float64(width))
	for py := 0; py < height; py++ {
		for px := 0; px < filled; px++ {
			screen.Set(x+px, y+py, fill)
		}
	}
}

func drawText(screen *ebiten.Image, x, y int, msg string, clr color.Color) {
	_ = clr
	ebitenutil.DebugPrintAt(screen, msg, x, y)
}

func containsChannel(channels []int, target int) bool {
	for _, ch := range channels {
		if ch == target {
			return true
		}
	}
	return false
}

func compareChannels(a, b []int) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func buildPartViews(states []adl.ChannelState) []partView {
	parts := map[string]*partView{}
	for _, st := range states {
		if st.ControlChannel || st.InstrumentID < 0 {
			continue
		}
		if !st.BytecodeActive && !st.KeyOn {
			continue
		}

		note := st.Note
		if note == "" {
			note = "rel"
		}
		key := fmt.Sprintf("%03d:%s", st.InstrumentID, note)
		part := parts[key]
		if part == nil {
			part = &partView{
				InstrumentID: st.InstrumentID,
				Note:         note,
				RemainingMin: 1,
				SortKey:      key,
			}
			parts[key] = part
		}

		part.Voices++
		part.Channels = append(part.Channels, st.Channel)
		part.RawVolume += st.OutputLevel
		part.StrongestVolume = max(part.StrongestVolume, st.OutputLevel)
		if st.KeyOn {
			part.KeyOnCount++
		}
		if st.Releasing {
			part.Releasing++
		}

		remaining := 0.0
		if st.InitialDuration > 0 {
			remaining = float64(st.Duration) / float64(st.InitialDuration)
		}
		if st.OutputLevel >= part.StrongestVolume {
			part.Remaining = remaining
		}
		if remaining < part.RemainingMin {
			part.RemainingMin = remaining
		}
		if remaining > part.RemainingMax {
			part.RemainingMax = remaining
		}
	}

	result := make([]partView, 0, len(parts))
	for _, part := range parts {
		sort.Ints(part.Channels)
		if part.Voices > 0 {
			if part.RemainingMax == 0 && part.RemainingMin == 1 {
				part.RemainingMin = 0
			}
		}
		part.RawVolume = clamp01(part.RawVolume)
		part.DisplayVolume = scalePartMeter(part.RawVolume)
		part.Summary = fmt.Sprintf("keyOn:%d peak:%3.0f%% rem:%3.0f-%3.0f%%", part.KeyOnCount, part.StrongestVolume*100, part.RemainingMin*100, part.RemainingMax*100)
		part.VariantSummary = fmt.Sprintf("variants:%d dom:%3.0f%%", part.Voices, part.Remaining*100)
		result = append(result, *part)
	}

	sort.Slice(result, func(i, j int) bool {
		if cmp := compareChannels(result[i].Channels, result[j].Channels); cmp != 0 {
			return cmp < 0
		}
		if math.Abs(result[i].RawVolume-result[j].RawVolume) > 0.001 {
			return result[i].RawVolume > result[j].RawVolume
		}
		if result[i].InstrumentID != result[j].InstrumentID {
			return result[i].InstrumentID < result[j].InstrumentID
		}
		return result[i].SortKey < result[j].SortKey
	})
	return result
}

func joinChannels(channels []int) string {
	parts := make([]string, len(channels))
	for i, ch := range channels {
		parts[i] = strconv.Itoa(ch)
	}
	return strings.Join(parts, ",")
}

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

func heldSoloChannel() int {
	keys := []struct {
		key ebiten.Key
		ch  int
	}{
		{ebiten.Key0, 0},
		{ebiten.Key1, 1},
		{ebiten.Key2, 2},
		{ebiten.Key3, 3},
		{ebiten.Key4, 4},
		{ebiten.Key5, 5},
		{ebiten.Key6, 6},
		{ebiten.Key7, 7},
		{ebiten.Key8, 8},
		{ebiten.Key9, 9},
	}
	for _, item := range keys {
		if ebiten.IsKeyPressed(item.key) {
			return item.ch
		}
	}
	return -1
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func scalePartMeter(raw float64) float64 {
	raw = clamp01(raw)
	if raw == 0 {
		return 0
	}
	// ADL channel output is much quieter per voice than the mixed master meter,
	// so boost and curve it for readability without changing the underlying data.
	return clamp01(math.Pow(raw*12, 0.55))
}

func main() {
	adlPath := "../adl/DUNE1.ADL"
	startSubsong := -1
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

	f, err := os.Open(adlPath)
	if err != nil {
		log.Fatalf("failed to open ADL file: %v", err)
	}
	defer f.Close()

	af, err := adl.Parse(f)
	if err != nil {
		log.Fatalf("failed to parse ADL file: %v", err)
	}

	subsongs := af.NonEmptySubsongs()
	if len(subsongs) == 0 {
		subsongs = af.ClassifySubsongs()
	}

	fmt.Printf("Loaded %s: v%d, %d programs, %d subsongs (%d non-empty)\n",
		filepath.Base(adlPath), af.Version, af.NumPrograms, af.NumSubsongs, len(subsongs))

	subsongIdx := 0
	if startSubsong >= 0 {
		for i, info := range subsongs {
			if info.Index == startSubsong {
				subsongIdx = i
				break
			}
		}
	} else {
		for i, info := range subsongs {
			if info.Type == adl.SubsongMusic {
				subsongIdx = i
				break
			}
		}
	}

	p := adl.NewPlayer(sampleRate, af)
	p.SetSubsong(subsongs[subsongIdx].Index)

	ds := &DebugStream{player: p}

	audioCtx := audio.NewContext(sampleRate)
	ap, err := audioCtx.NewPlayer(ds)
	if err != nil {
		log.Fatalf("failed to create audio player: %v", err)
	}
	ap.SetBufferSize(time.Millisecond * 100)
	ap.Play()

	p.Play()
	fmt.Printf("Playing subsong %d (%s) at %d Hz... close window to stop.\n",
		subsongs[subsongIdx].Index, subsongs[subsongIdx].Type, sampleRate)

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
		ds:          ds,
		player:      p,
		status:      "Initializing...",
		adlFiles:    adlFiles,
		fileIdx:     fileIdx,
		fileName:    filepath.Base(adlPath),
		subsongs:    subsongs,
		subsongIdx:  subsongIdx,
		soloChannel: -1,
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
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

			parts := buildPartViews(p.ChannelStates())
			info := g.currentSubsongInfo()
			line := "none"
			if len(parts) > 0 {
				limit := len(parts)
				if limit > 4 {
					limit = 4
				}
				top := make([]string, 0, limit)
				for _, part := range parts[:limit] {
					top = append(top, fmt.Sprintf("i%03d/%s x%d %.0f%%", part.InstrumentID, part.Note, part.Voices, part.DisplayVolume*100))
				}
				line = strings.Join(top, " | ")
			}

			fmt.Printf("[%s] %s | sub:%d (%s) | vol:%.0f%% | %s\n",
				time.Now().Format("15:04:05"),
				stateStr,
				info.Index,
				info.Type,
				ds.getVolume()*100,
				line,
			)
		}
	}()

	ebiten.SetWindowTitle("SpiceSynth ADL Player - Dune II")
	ebiten.SetWindowSize(windowWidth, windowHeight)
	if err := ebiten.RunGame(g); err != nil {
		if err != ebiten.Termination {
			log.Fatal(err)
		}
	}

	p.Close()
	fmt.Println("Goodbye.")
}
