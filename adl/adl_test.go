// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package adl

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

const testADLDir = "../examples/adl"

// loadTestFile opens a Dune II ADL file from the examples directory.
// Skips the test if the file is not found.
func loadTestFile(t *testing.T, name string) *File {
	t.Helper()
	f, err := os.Open(filepath.Join(testADLDir, name))
	if err != nil {
		t.Skipf("test ADL file not found: %v", err)
	}
	defer f.Close()

	af, err := Parse(f)
	if err != nil {
		t.Fatalf("Parse(%s) error: %v", name, err)
	}
	return af
}

// --- Parser tests ---

func TestParseAllFiles(t *testing.T) {
	entries, err := os.ReadDir(testADLDir)
	if err != nil {
		t.Skipf("ADL directory not found: %v", err)
	}

	parsed := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".ADL" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			af := loadTestFile(t, e.Name())
			if af.Version != 3 {
				t.Errorf("Version = %d, want 3 (Dune II)", af.Version)
			}
			if af.NumPrograms != 250 {
				t.Errorf("NumPrograms = %d, want 250", af.NumPrograms)
			}
			if af.NumSubsongs == 0 {
				t.Error("NumSubsongs = 0, want > 0")
			}
			if len(af.SoundData) == 0 {
				t.Error("SoundData is empty")
			}
			t.Logf("subsongs=%d, soundData=%d bytes", af.NumSubsongs, len(af.SoundData))
		})
		parsed++
	}
	if parsed == 0 {
		t.Skip("no .ADL files found in test directory")
	}
	t.Logf("successfully parsed %d ADL files", parsed)
}

func TestParseDUNE1(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	// Version 3, 250 programs.
	if af.Version != 3 {
		t.Fatalf("Version = %d, want 3", af.Version)
	}
	if af.NumPrograms != 250 {
		t.Fatalf("NumPrograms = %d, want 250", af.NumPrograms)
	}

	// Must have at least one subsong.
	if af.NumSubsongs == 0 {
		t.Fatal("NumSubsongs = 0")
	}
	t.Logf("DUNE1.ADL: %d subsongs", af.NumSubsongs)

	// Track entry for subsong 0 should be a valid program ID.
	track := af.TrackForSubsong(0)
	if track < 0 {
		t.Errorf("TrackForSubsong(0) = %d, want >= 0", track)
	}
	t.Logf("subsong 0 → track %d", track)
}

func TestGetProgram(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	track := af.TrackForSubsong(0)
	if track < 0 {
		t.Fatal("no valid track for subsong 0")
	}

	prog := af.GetProgram(track)
	if prog == nil {
		t.Fatalf("GetProgram(%d) returned nil", track)
	}
	if len(prog) < 2 {
		t.Fatalf("program data too short: %d bytes", len(prog))
	}
	t.Logf("program %d: %d bytes available from offset", track, len(prog))
}

func TestGetInstrument(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	// Try to find a valid instrument.
	found := 0
	for i := 0; i < af.NumPrograms; i++ {
		data := af.GetInstrument(i)
		if data != nil && len(data) >= 11 {
			ri, err := ParseRawInstrument(data)
			if err != nil {
				t.Errorf("ParseRawInstrument(%d) error: %v", i, err)
				continue
			}
			inst := ri.ToVoiceInstrument("test")
			if inst == nil {
				t.Errorf("ToVoiceInstrument(%d) returned nil", i)
			}
			found++
		}
	}
	if found == 0 {
		t.Error("no valid instruments found")
	}
	t.Logf("found %d valid instruments out of %d slots", found, af.NumPrograms)
}

func TestExtractInstruments(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	instruments := af.ExtractInstruments("dune2")
	if len(instruments) == 0 {
		t.Fatal("ExtractInstruments returned 0 instruments")
	}

	// Verify naming convention.
	for i, inst := range instruments {
		if inst.Name == "" {
			t.Errorf("instrument %d has empty name", i)
		}
	}
	t.Logf("extracted %d instruments", len(instruments))

	// Spot-check first instrument has valid OPL fields.
	first := instruments[0]
	if first.Op1.Attack > 15 || first.Op2.Attack > 15 {
		t.Errorf("attack out of range: op1=%d, op2=%d", first.Op1.Attack, first.Op2.Attack)
	}
	if first.Feedback > 7 {
		t.Errorf("feedback out of range: %d", first.Feedback)
	}
	if first.Connection > 1 {
		t.Errorf("connection out of range: %d", first.Connection)
	}
}

func TestParseRawInstrument(t *testing.T) {
	// Valid 11-byte instrument.
	data := []byte{0x21, 0x31, 0x04, 0x01, 0x02, 0x3F, 0x00, 0xF5, 0xF2, 0x11, 0x22}
	ri, err := ParseRawInstrument(data)
	if err != nil {
		t.Fatalf("ParseRawInstrument() error: %v", err)
	}
	if ri.ModChar != 0x21 {
		t.Errorf("ModChar = 0x%02X, want 0x21", ri.ModChar)
	}
	if ri.CarChar != 0x31 {
		t.Errorf("CarChar = 0x%02X, want 0x31", ri.CarChar)
	}
	if ri.FeedConn != 0x04 {
		t.Errorf("FeedConn = 0x%02X, want 0x04", ri.FeedConn)
	}

	// Conversion to voice.Instrument.
	inst := ri.ToVoiceInstrument("test_inst")
	if inst.Name != "test_inst" {
		t.Errorf("Name = %q, want %q", inst.Name, "test_inst")
	}
	if inst.Feedback != 2 { // (0x04 >> 1) & 0x07 = 2
		t.Errorf("Feedback = %d, want 2", inst.Feedback)
	}
	if inst.Connection != 0 { // 0x04 & 0x01 = 0
		t.Errorf("Connection = %d, want 0", inst.Connection)
	}

	// Short data should error.
	_, err = ParseRawInstrument([]byte{0x00, 0x01})
	if err == nil {
		t.Error("expected error for short instrument data")
	}
}

func TestTrackForSubsong(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	// Valid subsong.
	track := af.TrackForSubsong(0)
	if track < 0 || track >= af.NumPrograms {
		t.Errorf("TrackForSubsong(0) = %d, want [0, %d)", track, af.NumPrograms)
	}

	// Out-of-range subsong.
	track = af.TrackForSubsong(999)
	if track != -1 {
		t.Errorf("TrackForSubsong(999) = %d, want -1", track)
	}
	track = af.TrackForSubsong(-1)
	if track != -1 {
		t.Errorf("TrackForSubsong(-1) = %d, want -1", track)
	}
}

func TestFileTooSmall(t *testing.T) {
	_, err := ParseBytes(make([]byte, 100))
	if err == nil {
		t.Error("expected error for tiny file")
	}
}

// --- Player tests ---

func TestNewPlayer(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	p := NewPlayer(44100, af)
	defer p.Close()

	if p.GetState() != StateStopped {
		t.Errorf("initial state = %d, want StateStopped", p.GetState())
	}
	if p.NumSubsongs() == 0 {
		t.Error("NumSubsongs() = 0")
	}
	t.Logf("player created: %d subsongs", p.NumSubsongs())
}

func TestPlayPauseStop(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	p := NewPlayer(44100, af)
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

func TestSetSubsong(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	p := NewPlayer(44100, af)
	defer p.Close()

	if p.CurrentSubsong() != 0 {
		t.Errorf("initial subsong = %d, want 0", p.CurrentSubsong())
	}

	p.Play()
	p.SetSubsong(2)
	if p.CurrentSubsong() != 2 {
		t.Errorf("after SetSubsong(2): current = %d, want 2", p.CurrentSubsong())
	}
}

func TestReadProducesAudio(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	p := NewPlayer(44100, af)
	defer p.Close()

	// Subsong 0 is typically a "reset/silence" track in Dune II ADL files.
	// Use subsong 2 which is the first real music track.
	p.SetSubsong(2)
	p.Play()

	// Read ~500ms of audio (enough for the bytecode VM to start generating notes).
	buf := make([]byte, 44100/2*4) // 22050 frames × 4 bytes/frame
	n, err := p.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != len(buf) {
		t.Errorf("Read() = %d bytes, want %d", n, len(buf))
	}

	// Check for non-zero samples.
	nonZero := 0
	for i := 0; i < len(buf)-1; i += 2 {
		sample := int16(buf[i]) | int16(buf[i+1])<<8
		if sample != 0 {
			nonZero++
		}
	}
	t.Logf("non-zero samples: %d / %d (%.1f%%)", nonZero, len(buf)/2, float64(nonZero)/float64(len(buf)/2)*100)
	if nonZero == 0 {
		t.Error("all samples are zero — no audio generated")
	}
}

func TestSilenceWhenStopped(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	p := NewPlayer(44100, af)
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

	for i := 0; i < len(buf); i++ {
		if buf[i] != 0 {
			t.Error("expected silence when stopped, got non-zero sample")
			break
		}
	}
}

func TestReadMultipleSubsongs(t *testing.T) {
	af := loadTestFile(t, "DUNE9.ADL")

	p := NewPlayer(44100, af)
	defer p.Close()

	maxSubsongs := af.NumSubsongs
	if maxSubsongs > 5 {
		maxSubsongs = 5
	}

	for sub := 0; sub < maxSubsongs; sub++ {
		t.Run(string(rune('0'+sub)), func(t *testing.T) {
			p.Stop()
			p.SetSubsong(sub)
			p.Play()

			// Read 200ms.
			buf := make([]byte, 44100/5*4)
			n, err := p.Read(buf)
			if err != nil {
				t.Fatalf("Read() error for subsong %d: %v", sub, err)
			}
			if n != len(buf) {
				t.Errorf("Read() = %d, want %d", n, len(buf))
			}
			t.Logf("subsong %d: read %d bytes OK", sub, n)
		})
	}
}

func TestVolumeAndGain(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	p := NewPlayer(44100, af)
	defer p.Close()

	p.SetMasterVolume(0.5)
	p.SetGain(2.0)
	p.Play()

	buf := make([]byte, 4096)
	n, err := p.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != len(buf) {
		t.Errorf("Read() = %d, want %d", n, len(buf))
	}
}

func TestAllDuneFilesPlayWithoutPanic(t *testing.T) {
	entries, err := os.ReadDir(testADLDir)
	if err != nil {
		t.Skipf("ADL directory not found: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".ADL" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			af := loadTestFile(t, e.Name())
			p := NewPlayer(44100, af)
			defer p.Close()

			p.Play()

			// Read 200ms — enough to exercise the bytecode VM without
			// taking too long. The main goal is "doesn't panic".
			buf := make([]byte, 44100/5*4)
			n, err := p.Read(buf)
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			if n != len(buf) {
				t.Errorf("Read() = %d bytes, want %d", n, len(buf))
			}
		})
	}
}

func TestDUNE1Subsong6FastRetriggerStaysAudible(t *testing.T) {
	af := loadTestFile(t, "DUNE1.ADL")

	p := NewPlayer(44100, af)
	defer p.Close()

	p.SetSubsong(6)
	p.SetSoloChannel(0)
	p.Play()

	buf := make([]byte, 44100/2*4)
	soloPCM := make([]float64, 0, 12)

	for i := 0; i < 12; i++ {
		if _, err := p.Read(buf); err != nil {
			t.Fatalf("Read() block %d error: %v", i, err)
		}
		states := p.ChannelStates()
		if len(states) <= 0 {
			t.Fatalf("ChannelStates() returned %d states, want > 0", len(states))
		}
		soloPCM = append(soloPCM, pcmPeak(buf))
	}

	earlyPCM := maxSlice(soloPCM[:4])
	laterPCM := maxSlice(soloPCM[4:])

	if earlyPCM <= 0 {
		t.Fatalf("expected audible early ch0 output, got %.4f", earlyPCM)
	}
	if laterPCM < earlyPCM*0.18 {
		t.Fatalf("soloed ch0 output decayed too far during repeated notes: early=%.4f later=%.4f", earlyPCM, laterPCM)
	}
	if math.IsNaN(laterPCM) {
		t.Fatal("unexpected NaN in measured peaks")
	}
}

// parseVolFromLine extracts reg43 value from setupInstrument trace lines.
func parseVolFromLine(line string, chVols map[int]*channelVolInfo) {
	// Format: "setupInstrument: ch%d ... → reg40=0x%02X reg43=0x%02X"
	for ch := 0; ch < 9; ch++ {
		prefix := fmt.Sprintf("setupInstrument: ch%d ", ch)
		if !contains(line, prefix) {
			continue
		}
		// Find "reg43=0x" and parse the hex byte.
		idx := indexOf(line, "reg43=0x")
		if idx < 0 || idx+10 > len(line) {
			break
		}
		val := parseHexByte(line[idx+8 : idx+10])
		atten := val & 0x3F
		cv := chVols[ch]
		if atten < cv.minReg43 {
			cv.minReg43 = atten
		}
		if atten > cv.maxReg43 {
			cv.maxReg43 = atten
		}
		cv.volAdjs++
		break
	}
}

// parseAdjustVolFromLine extracts reg43 value from adjustVolume trace lines.
func parseAdjustVolFromLine(line string, chVols map[int]*channelVolInfo) {
	for ch := 0; ch < 9; ch++ {
		prefix := fmt.Sprintf("adjustVolume: ch%d ", ch)
		if !contains(line, prefix) {
			continue
		}
		idx := indexOf(line, "reg43=0x")
		if idx < 0 || idx+10 > len(line) {
			break
		}
		val := parseHexByte(line[idx+8 : idx+10])
		atten := val & 0x3F
		cv := chVols[ch]
		if atten < cv.minReg43 {
			cv.minReg43 = atten
		}
		if atten > cv.maxReg43 {
			cv.maxReg43 = atten
		}
		cv.volAdjs++
		break
	}
}

// indexOf finds the first occurrence of substr in s, returns -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// parseHexByte parses a 2-char hex string to a byte.
func parseHexByte(s string) uint8 {
	if len(s) < 2 {
		return 0
	}
	return hexDigit(s[0])<<4 | hexDigit(s[1])
}

func hexDigit(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

func pcmPeak(buf []byte) float64 {
	peak := 0.0
	for i := 0; i < len(buf)-1; i += 2 {
		sample := int16(buf[i]) | int16(buf[i+1])<<8
		abs := math.Abs(float64(sample)) / 32768.0
		if abs > peak {
			peak = abs
		}
	}
	return peak
}

func maxSlice(vals []float64) float64 {
	peak := 0.0
	for _, v := range vals {
		if v > peak {
			peak = v
		}
	}
	return peak
}

// channelVolInfo tracks per-channel volume statistics during tracing.
type channelVolInfo struct {
	minReg43 uint8 // Lowest (loudest) carrier attenuation seen
	maxReg43 uint8 // Highest (quietest) carrier attenuation seen
	noteOns  int
	volAdjs  int
}

// contains checks if s contains substr (simple helper to avoid strings import in test).
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
