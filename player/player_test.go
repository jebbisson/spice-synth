// Copyright (c) 2026 Jeb Bisson. LGPL-2.1-or-later. See LICENSE.

package player

import (
	"os"
	"testing"

	"github.com/jebbisson/spice-synth/midi"
	"github.com/jebbisson/spice-synth/op2"
)

func loadTestData(t *testing.T) (*op2.Bank, *midi.File) {
	t.Helper()

	bank, err := op2.DefaultBank()
	if err != nil {
		t.Fatalf("DefaultBank() error: %v", err)
	}

	f, err := os.Open("../examples/midi/Title.mid")
	if err != nil {
		t.Skipf("test MIDI file not found: %v", err)
	}
	defer f.Close()

	mf, err := midi.Parse(f)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	return bank, mf
}

func TestNewPlayer(t *testing.T) {
	bank, mf := loadTestData(t)

	p := New(44100, bank, mf)
	defer p.Close()

	if p.GetState() != StateStopped {
		t.Errorf("initial state = %d, want StateStopped", p.GetState())
	}

	if len(p.events) == 0 {
		t.Fatal("no events built")
	}
	t.Logf("built %d timed events", len(p.events))
}

func TestPlayPauseStop(t *testing.T) {
	bank, mf := loadTestData(t)

	p := New(44100, bank, mf)
	defer p.Close()

	p.Play()
	if p.GetState() != StatePlaying {
		t.Errorf("after Play(): state = %d, want StatePlaying", p.GetState())
	}

	p.Pause()
	if p.GetState() != StatePaused {
		t.Errorf("after Pause(): state = %d, want StatePaused", p.GetState())
	}

	p.Play()
	if p.GetState() != StatePlaying {
		t.Errorf("after resume: state = %d, want StatePlaying", p.GetState())
	}

	p.Stop()
	if p.GetState() != StateStopped {
		t.Errorf("after Stop(): state = %d, want StateStopped", p.GetState())
	}
}

func TestReadProducesAudio(t *testing.T) {
	bank, mf := loadTestData(t)

	p := New(44100, bank, mf)
	defer p.Close()

	p.Play()

	// Read ~100ms of audio.
	buf := make([]byte, 44100/10*4) // 4410 frames × 4 bytes
	n, err := p.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != len(buf) {
		t.Errorf("Read() = %d bytes, want %d", n, len(buf))
	}

	// Check that we got some non-zero samples (audio is being generated).
	nonZero := 0
	for i := 0; i < len(buf)-1; i += 2 {
		sample := int16(buf[i]) | int16(buf[i+1])<<8
		if sample != 0 {
			nonZero++
		}
	}
	t.Logf("non-zero samples: %d / %d", nonZero, len(buf)/2)
	if nonZero == 0 {
		t.Error("all samples are zero — no audio generated")
	}
}

func TestMultiChipAllocation(t *testing.T) {
	bank, mf := loadTestData(t)

	p := New(44100, bank, mf)
	defer p.Close()

	p.Play()

	// Read enough audio to trigger some notes.
	buf := make([]byte, 44100*4) // 1 second
	p.Read(buf)

	chips := p.ActiveChips()
	voices := p.ActiveVoices()
	t.Logf("after 1s: %d chips, %d active voices", chips, voices)

	if chips == 0 {
		t.Error("no chips allocated after 1 second of playback")
	}
}

func TestProgress(t *testing.T) {
	bank, mf := loadTestData(t)

	p := New(44100, bank, mf)
	defer p.Close()

	cur, total := p.Progress()
	if cur != 0 {
		t.Errorf("initial current = %d, want 0", cur)
	}
	if total == 0 {
		t.Error("total = 0, want > 0")
	}
	t.Logf("total samples: %d (%.1f seconds)", total, float64(total)/44100.0)

	p.Play()

	buf := make([]byte, 44100*4) // 1 second
	p.Read(buf)

	cur, _ = p.Progress()
	if cur == 0 {
		t.Error("current = 0 after reading 1 second")
	}
	t.Logf("after 1s read: position = %d samples (%.2f seconds)", cur, float64(cur)/44100.0)
}

func TestSilenceWhenStopped(t *testing.T) {
	bank, mf := loadTestData(t)

	p := New(44100, bank, mf)
	defer p.Close()

	// Don't call Play() — state is Stopped.
	buf := make([]byte, 4096)
	n, err := p.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != len(buf) {
		t.Errorf("Read() = %d, want %d", n, len(buf))
	}

	// All samples should be zero (silence).
	for i := 0; i < len(buf); i++ {
		if buf[i] != 0 {
			t.Error("expected silence when stopped, got non-zero sample")
			break
		}
	}
}

func TestDuration(t *testing.T) {
	bank, mf := loadTestData(t)

	p := New(44100, bank, mf)
	defer p.Close()

	dur := p.DurationSeconds()
	t.Logf("duration: %.1f seconds", dur)
	if dur <= 0 {
		t.Error("duration should be > 0")
	}
}
