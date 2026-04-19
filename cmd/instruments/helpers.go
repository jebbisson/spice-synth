package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/jebbisson/spice-synth/dsl"
	"github.com/jebbisson/spice-synth/instrument"
	"github.com/jebbisson/spice-synth/stream"
	"github.com/jebbisson/spice-synth/voice"
)

const sampleRate = 44100

var (
	audioCtxOnce sync.Once
	audioCtx     *audio.Context
	audioCtxErr  error
	previewMu    sync.Mutex
	previewCur   *previewSession
)

type previewSession struct {
	player    *audio.Player
	stream    *stream.Stream
	closeOnce sync.Once
}

func (s *previewSession) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.player != nil {
			s.player.Pause()
			s.player.Close()
		}
		if s.stream != nil {
			s.stream.Close()
		}
	})
}

func sortedGroups(f *instrument.File) []string {
	groups := f.Groups()
	sort.Strings(groups)
	return groups
}

func allVariantKeys(f *instrument.File) []string {
	keys := make([]string, 0)
	for _, inst := range f.Instruments {
		for _, variant := range inst.Variants {
			keys = append(keys, inst.Name+"."+variant.Name)
		}
	}
	sort.Strings(keys)
	return keys
}

func findVariantDef(f *instrument.File, key string) (*instrument.FileInstrument, *instrument.InstrumentDef, error) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid variant key %q", key)
	}
	for i := range f.Instruments {
		if !strings.EqualFold(f.Instruments[i].Name, parts[0]) {
			continue
		}
		for j := range f.Instruments[i].Variants {
			if strings.EqualFold(f.Instruments[i].Variants[j].Name, parts[1]) {
				return &f.Instruments[i], &f.Instruments[i].Variants[j], nil
			}
		}
	}
	return nil, nil, fmt.Errorf("variant %q not found", key)
}

func fullKey(inst *instrument.FileInstrument, variant *instrument.InstrumentDef) string {
	return inst.Name + "." + variant.Name
}

func defaultNoteForInstrument(inst *instrument.FileInstrument) string {
	if inst == nil {
		return "C4"
	}
	note := strings.TrimSpace(inst.DefaultNote)
	if note == "" {
		return "C4"
	}
	return note
}

func instrumentCode(name string, inst *voice.Instrument) string {
	return fmt.Sprintf(`var %s = &voice.Instrument{
	Name: %q,
	Op1: voice.Operator{Attack: %d, Decay: %d, Sustain: %d, Release: %d, Level: %d, Multiply: %d, KeyScaleRate: %t, KeyScaleLevel: %d, Tremolo: %t, Vibrato: %t, Sustaining: %t, Waveform: %d},
	Op2: voice.Operator{Attack: %d, Decay: %d, Sustain: %d, Release: %d, Level: %d, Multiply: %d, KeyScaleRate: %t, KeyScaleLevel: %d, Tremolo: %t, Vibrato: %t, Sustaining: %t, Waveform: %d},
	Feedback: %d,
	Connection: %d,
}
`, sanitizeIdentifier(name), inst.Name,
		inst.Op1.Attack, inst.Op1.Decay, inst.Op1.Sustain, inst.Op1.Release, inst.Op1.Level, inst.Op1.Multiply, inst.Op1.KeyScaleRate, inst.Op1.KeyScaleLevel, inst.Op1.Tremolo, inst.Op1.Vibrato, inst.Op1.Sustaining, inst.Op1.Waveform,
		inst.Op2.Attack, inst.Op2.Decay, inst.Op2.Sustain, inst.Op2.Release, inst.Op2.Level, inst.Op2.Multiply, inst.Op2.KeyScaleRate, inst.Op2.KeyScaleLevel, inst.Op2.Tremolo, inst.Op2.Vibrato, inst.Op2.Sustaining, inst.Op2.Waveform,
		inst.Feedback, inst.Connection)
}

func sanitizeIdentifier(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	clean := b.String()
	if clean == "" {
		return "instrument"
	}
	if clean[0] >= '0' && clean[0] <= '9' {
		return "instrument_" + clean
	}
	return clean
}

func previewInstrument(inst *voice.Instrument, note string, seconds time.Duration) error {
	ctx, err := sharedAudioContext()
	if err != nil {
		return err
	}
	s := stream.New(sampleRate)
	s.Voices().LoadBank("preview", []*voice.Instrument{inst})
	if err := dsl.Note(note).Sound(inst.Name).Play(s, 0); err != nil {
		s.Close()
		return err
	}
	p, err := ctx.NewPlayer(s)
	if err != nil {
		s.Close()
		return err
	}
	session := &previewSession{player: p, stream: s}
	previewMu.Lock()
	old := previewCur
	previewCur = session
	previewMu.Unlock()
	old.close()
	p.Play()
	time.Sleep(seconds)
	previewMu.Lock()
	if previewCur == session {
		previewCur = nil
	}
	previewMu.Unlock()
	session.close()
	return nil
}

func stopPreview() {
	previewMu.Lock()
	current := previewCur
	previewCur = nil
	previewMu.Unlock()
	current.close()
}

func sharedAudioContext() (*audio.Context, error) {
	audioCtxOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				audioCtxErr = fmt.Errorf("audio: %v", r)
			}
		}()
		audioCtx = audio.NewContext(sampleRate)
	})
	if audioCtxErr != nil {
		return nil, audioCtxErr
	}
	if audioCtx == nil {
		return nil, fmt.Errorf("audio: context unavailable")
	}
	return audioCtx, nil
}
