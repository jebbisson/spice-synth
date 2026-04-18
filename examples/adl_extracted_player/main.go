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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/stream"
)

const sampleRate = 44100

const (
	windowWidth   = 920
	windowHeight  = 760
	maxTrackRows  = 14
	extractedName = "DUNE1.ADL subsong 6"
)

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

func (d *DebugStream) elapsedSeconds() float64 {
	return float64(d.getBytesRead()) / float64(sampleRate*4)
}

func (d *DebugStream) currentTick(bpm float64) int {
	return int(d.elapsedSeconds() * ((bpm * 4.0) / 60.0))
}

type trackView struct {
	Channel      int
	Instrument   string
	Note         string
	Progress     float64
	EventCount   int
	LastEvent    int
	NextEvent    int
	RecentEvents []string
	Active       bool
}

// Game implements the ebiten.Game interface.
type Game struct {
	ds         *DebugStream
	status     string
	tickCnt    int
	song       *dsl.Song
	songLength int
	trackViews []trackView
	audio      *audio.Player
	paused     bool
	pausedRead int64
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.audio != nil {
		if !g.paused {
			g.paused = true
			g.pausedRead = g.ds.getBytesRead()
			g.audio.SetVolume(0)
			g.audio.Pause()
		} else {
			g.paused = false
			g.audio.SetVolume(1)
			g.audio.Play()
		}
	}

	g.tickCnt++
	if g.tickCnt%2 == 0 {
		g.trackViews = buildTrackViews(g.song, g.ds, g.currentTick(), g.songLength)
		g.status = g.buildStatus()
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{12, 14, 18, 255})
	ebitenutil.DebugPrintAt(screen, g.status, 16, 16)
	ebitenutil.DebugPrintAt(screen, "Controls: Space pause/resume | Q / Escape quit", 16, 106)

	masterVolume := g.ds.getVolume()
	drawMeter(screen, 16, 148, 360, 12, masterVolume, color.RGBA{72, 220, 120, 255}, color.RGBA{36, 52, 42, 255})
	ebitenutil.DebugPrintAt(screen, "Master Output", 16, 130)

	headY := 184
	ebitenutil.DebugPrintAt(screen, "Active extracted tracks derived from the generated DSL timeline", 16, headY)
	ebitenutil.DebugPrintAt(screen, "Rows show the current note/instrument state per channel plus recent track events.", 16, headY+16)

	rowTop := headY + 44
	rowHeight := 52
	shown := 0
	for _, tv := range g.trackViews {
		if shown >= maxTrackRows {
			break
		}
		y := rowTop + shown*rowHeight
		drawTrackRow(screen, y, tv)
		shown++
	}
	if shown == 0 {
		ebitenutil.DebugPrintAt(screen, "No extracted tracks available.", 16, rowTop)
	}
	if len(g.trackViews) > maxTrackRows {
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("... %d more tracks not shown", len(g.trackViews)-maxTrackRows), 16, rowTop+maxTrackRows*rowHeight+6)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return windowWidth, windowHeight
}

func (g *Game) buildStatus() string {
	bpm := 0.0
	if g.song != nil {
		bpm = g.song.BPM()
	}
	curTick := g.currentTick()
	curSec := g.currentSeconds()
	activeTracks := 0
	for _, tv := range g.trackViews {
		if tv.Active {
			activeTracks++
		}
	}
	return fmt.Sprintf(
		"SpiceSynth ADL Extracted Player\n\n"+
			"Song: %s\n"+
			"Source: generated Go via adl2dsl\n"+
			"Time: %.1fs | Tick: %d/%d | BPM: %.0f | Master: %.0f%% | Active tracks: %d | Read: %d KB\n"+
			"Playback: %s",
		extractedName,
		curSec,
		curTick,
		g.songLength,
		bpm,
		g.ds.getVolume()*100,
		activeTracks,
		g.currentReadBytes()/1024,
		g.playbackState(),
	)
}

func (g *Game) playbackState() string {
	if g.paused {
		return "paused"
	}
	return "playing"
}

func (g *Game) currentReadBytes() int64 {
	if g.paused {
		return g.pausedRead
	}
	return g.ds.getBytesRead()
}

func (g *Game) currentSeconds() float64 {
	return float64(g.currentReadBytes()) / float64(sampleRate*4)
}

func (g *Game) currentTick() int {
	if g.song == nil {
		return 0
	}
	return wrapTick(int(g.currentSeconds()*((g.song.BPM()*4.0)/60.0)), g.songLength)
}

func buildTrackViews(song *dsl.Song, ds *DebugStream, curTick, songLength int) []trackView {
	if song == nil || ds == nil {
		return nil
	}
	views := make([]trackView, 0, len(song.Tracks()))
	for _, tr := range song.Tracks() {
		if tr == nil {
			continue
		}
		view := trackView{
			Channel:    tr.Channel(),
			Instrument: tr.Instrument(),
			EventCount: len(tr.Events()),
			LastEvent:  -1,
			NextEvent:  -1,
		}
		applyTrackLoopState(&view, tr.Events(), curTick)
		for _, ev := range recentEventsAtTick(tr.Events(), curTick, songLength, 3) {
			view.RecentEvents = append(view.RecentEvents, eventSummary(ev, songLength))
		}
		view.LastEvent = lastEventTick(tr.Events(), curTick)
		view.NextEvent = nextEventTick(tr.Events(), curTick, songLength)
		view.Progress = trackProgress(curTick, view.LastEvent, view.NextEvent, songLength)
		if !view.Active {
			view.Progress = 0
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool {
		return views[i].Channel < views[j].Channel
	})
	return views
}

func appendRecent(events []string, item string) []string {
	events = append(events, item)
	if len(events) > 3 {
		events = events[len(events)-3:]
	}
	return events
}

func shortenInstrument(name string) string {
	return name
}

func displayInstrument(name string) string {
	name = strings.TrimSuffix(name, ".default")
	name = shortenInstrument(name)
	if len(name) > 20 {
		return name[:20]
	}
	return name
}

func drawTrackRow(screen *ebiten.Image, y int, tv trackView) {
	lineColor := color.RGBA{42, 48, 58, 255}
	meterFill := color.RGBA{100, 210, 255, 255}
	meterBg := color.RGBA{30, 48, 60, 255}
	if !tv.Active {
		lineColor = color.RGBA{28, 31, 38, 255}
		meterFill = color.RGBA{52, 86, 104, 255}
		meterBg = color.RGBA{20, 28, 34, 255}
	}
	for x := 16; x < windowWidth-16; x++ {
		screen.Set(x, y-4, lineColor)
	}
	state := "idle"
	if tv.Active {
		state = "on"
	}
	if tv.Note == "" {
		tv.Note = "-"
	}
	left := fmt.Sprintf("ch%-1d  %-4s  %-6s  %-18s", tv.Channel, state, tv.Note, displayInstrument(tv.Instrument))
	right := fmt.Sprintf("events:%-4d", tv.EventCount)
	if tv.NextEvent >= 0 {
		right += fmt.Sprintf(" next:%-5d", tv.NextEvent)
	}
	ebitenutil.DebugPrintAt(screen, left, 16, y)
	ebitenutil.DebugPrintAt(screen, right, 320, y)
	drawMeter(screen, 16, y+20, 360, 10, tv.Progress, meterFill, meterBg)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%3.0f%%", tv.Progress*100), 384, y+16)
	if len(tv.RecentEvents) > 0 {
		ebitenutil.DebugPrintAt(screen, fitText(strings.Join(tv.RecentEvents, " | "), 62), 16, y+34)
	}
}

func drawMeter(screen *ebiten.Image, x, y, width, height int, value float64, fill, bg color.RGBA) {
	value = clamp01(value)
	for yy := y; yy < y+height; yy++ {
		for xx := x; xx < x+width; xx++ {
			screen.Set(xx, yy, bg)
		}
	}
	filled := int(value * float64(width))
	for yy := y; yy < y+height; yy++ {
		for xx := x; xx < x+filled; xx++ {
			screen.Set(xx, yy, fill)
		}
	}
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

func countVisibleTracks(views []trackView) int {
	count := 0
	for _, tv := range views {
		if tv.Active {
			count++
		}
	}
	return count
}

func lastEventTick(events []dsl.TrackEvent, curTick int) int {
	last := -1
	for _, ev := range events {
		if ev.Tick > curTick {
			break
		}
		last = ev.Tick
	}
	return last
}

func trackProgress(curTick, lastTick, nextTick, songLength int) float64 {
	if nextTick < 0 {
		return 0
	}
	if lastTick < 0 {
		lastTick = 0
	}
	span := tickDistance(lastTick, nextTick, songLength)
	if span <= 0 {
		return 0
	}
	pos := tickDistance(lastTick, curTick, songLength)
	if pos < 0 {
		return 0
	}
	return clamp01(float64(pos) / float64(span))
}

func songLength(song *dsl.Song) int {
	if song == nil {
		return 0
	}
	length := 0
	for _, tr := range song.Tracks() {
		if tr == nil {
			continue
		}
		if tr.Length() > length {
			length = tr.Length()
		}
		for _, ev := range tr.Events() {
			if ev.Tick+1 > length {
				length = ev.Tick + 1
			}
		}
	}
	return length
}

func wrapTick(tick, length int) int {
	if length <= 0 {
		return tick
	}
	return tick % length
}

func tickDistance(curTick, eventTick, length int) int {
	if length <= 0 {
		return eventTick - curTick
	}
	if eventTick >= curTick {
		return eventTick - curTick
	}
	return (length - curTick) + eventTick
}

func recentEventsAtTick(events []dsl.TrackEvent, curTick, songLength, count int) []dsl.TrackEvent {
	past := make([]dsl.TrackEvent, 0, len(events))
	for _, ev := range events {
		if ev.Tick <= curTick {
			past = append(past, ev)
		}
	}
	if len(past) <= count {
		return past
	}
	return past[len(past)-count:]
}

func nextEventTick(events []dsl.TrackEvent, curTick, songLength int) int {
	bestTick := -1
	bestDistance := -1
	for _, ev := range events {
		for _, candidate := range []int{ev.Tick, ev.Tick + songLength} {
			distance := tickDistance(curTick, candidate, songLength)
			if distance <= 0 {
				continue
			}
			if bestDistance < 0 || distance < bestDistance {
				bestDistance = distance
				bestTick = wrapTick(candidate, songLength)
			}
		}
	}
	return bestTick
}

func applyTrackEvent(view *trackView, ev dsl.TrackEvent) {
	if view == nil {
		return
	}
	switch ev.Type {
	case dsl.TrackInstrumentChange:
		if ev.Instrument != "" {
			view.Instrument = ev.Instrument
		}
	case dsl.TrackNoteOn:
		view.Active = true
		if ev.NoteStr != "" {
			view.Note = ev.NoteStr
		} else {
			view.Note = fmt.Sprintf("%.1fHz", float64(ev.Note))
		}
		if ev.Instrument != "" {
			view.Instrument = ev.Instrument
		}
	case dsl.TrackNoteOff:
		view.Active = false
		view.Note = "-"
	}
}

func applyTrackLoopState(view *trackView, events []dsl.TrackEvent, curTick int) {
	if view == nil {
		return
	}
	for _, ev := range events {
		applyTrackEvent(view, ev)
	}
	for _, ev := range events {
		if ev.Tick > curTick {
			break
		}
		applyTrackEvent(view, ev)
	}
}

func eventSummary(ev dsl.TrackEvent, songLength int) string {
	tick := wrapTick(ev.Tick, songLength)
	switch ev.Type {
	case dsl.TrackInstrumentChange:
		return fmt.Sprintf("t%d inst %s", tick, displayInstrument(ev.Instrument))
	case dsl.TrackNoteOn:
		note := ev.NoteStr
		if note == "" {
			note = fmt.Sprintf("%.1fHz", float64(ev.Note))
		}
		if ev.Override != nil && !ev.Override.Empty() {
			return fmt.Sprintf("t%d on %s *", tick, note)
		}
		return fmt.Sprintf("t%d on %s", tick, note)
	case dsl.TrackNoteOff:
		return fmt.Sprintf("t%d off", tick)
	case dsl.TrackFrequencyChange:
		return fmt.Sprintf("t%d freq %.1f", tick, ev.Frequency)
	case dsl.TrackLevelChange:
		return fmt.Sprintf("t%d op%d=%d", tick, ev.Operator, ev.Level)
	case dsl.TrackFeedbackChange:
		return fmt.Sprintf("t%d fb=%d", tick, ev.Feedback)
	default:
		return fmt.Sprintf("t%d event", tick)
	}
}

func fitText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func main() {
	fmt.Println("--- SpiceSynth ADL Extracted Player ---")
	fmt.Printf("Host OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// 1. Create the audio stream.
	s := stream.New(sampleRate)
	song := NewDUNE1Song6()
	length := songLength(song)

	// 2. Load and play the extracted song.
	//    ---------------------------------------------------------------
	//    To play a different extracted song, replace song.go with the
	//    output of: adl2dsl -package main -output song.go <your.ADL>
	//    Then update the function call below to match.
	//    ---------------------------------------------------------------
	if err := song.Play(s); err != nil {
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

	// 4. Run the Ebiten window.
	g := &Game{
		ds:         ds,
		song:       song,
		songLength: length,
		status:     "Initializing...",
		audio:      player,
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			currentRead := g.currentReadBytes()
			currentTick := g.currentTick()
			trackViews := buildTrackViews(song, ds, currentTick, length)
			active := make([]string, 0, len(trackViews))
			for _, tv := range trackViews {
				if !tv.Active {
					continue
				}
				label := fmt.Sprintf("ch%d:%s@%s", tv.Channel, tv.Note, shortenInstrument(tv.Instrument))
				label += "*"
				active = append(active, label)
				if len(active) >= 5 {
					break
				}
			}
			if len(active) == 0 {
				active = append(active, "none")
			}
			fmt.Printf("[%s] Tick:%d/%d Vol:%.0f%% State:%s Active:%s Read:%d KB\n",
				time.Now().Format("15:04:05"),
				currentTick,
				length,
				ds.getVolume()*100,
				g.playbackState(),
				strings.Join(active, ", "),
				currentRead/1024,
			)
		}
	}()

	ebiten.SetWindowTitle("SpiceSynth - ADL Extracted Song")
	ebiten.SetWindowSize(windowWidth, windowHeight)
	if err := ebiten.RunGame(g); err != nil {
		if err != ebiten.Termination {
			log.Fatal(err)
		}
	}

	s.Close()
	fmt.Println("Goodbye.")
}
