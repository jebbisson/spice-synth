package main

/*
#include <stdint.h>
*/
import "C"

import (
	"bytes"
	"sync"
	"unsafe"

	"github.com/jebbisson/spice-synth/adl"
	"github.com/jebbisson/spice-synth/midi"
	"github.com/jebbisson/spice-synth/op2"
	"github.com/jebbisson/spice-synth/player"
	"github.com/jebbisson/spice-synth/stream"
)

type handle uint64

type pcmReader interface {
	Read([]byte) (int, error)
	Close()
}

var (
	handlesMu  sync.Mutex
	nextHandle handle = 1
	streams           = map[handle]*stream.Stream{}
	players           = map[handle]pcmReader{}
)

func main() {}

func addStreamHandle(s *stream.Stream) handle {
	handlesMu.Lock()
	defer handlesMu.Unlock()
	h := nextHandle
	nextHandle++
	streams[h] = s
	return h
}

func addPlayerHandle(p pcmReader) handle {
	handlesMu.Lock()
	defer handlesMu.Unlock()
	h := nextHandle
	nextHandle++
	players[h] = p
	return h
}

func withStream(h C.uint64_t, fn func(*stream.Stream) C.int) C.int {
	handlesMu.Lock()
	s := streams[handle(h)]
	handlesMu.Unlock()
	if s == nil {
		return -1
	}
	return fn(s)
}

func withPlayer(h C.uint64_t, fn func(pcmReader) C.int) C.int {
	handlesMu.Lock()
	p := players[handle(h)]
	handlesMu.Unlock()
	if p == nil {
		return -1
	}
	return fn(p)
}

//export SpiceSynth_Stream_Create
func SpiceSynth_Stream_Create(sampleRate C.int32_t) C.uint64_t {
	if sampleRate <= 0 {
		return 0
	}
	return C.uint64_t(addStreamHandle(stream.New(int(sampleRate))))
}

//export SpiceSynth_Stream_Destroy
func SpiceSynth_Stream_Destroy(h C.uint64_t) C.int {
	handlesMu.Lock()
	s := streams[handle(h)]
	if s != nil {
		delete(streams, handle(h))
	}
	handlesMu.Unlock()
	if s == nil {
		return -1
	}
	s.Close()
	return 0
}

//export SpiceSynth_Stream_Read
func SpiceSynth_Stream_Read(h C.uint64_t, out *C.uint8_t, outLen C.int32_t) C.int {
	if out == nil || outLen <= 0 {
		return -2
	}
	return withStream(h, func(s *stream.Stream) C.int {
		buf := unsafe.Slice((*byte)(unsafe.Pointer(out)), int(outLen))
		n, err := s.Read(buf)
		if err != nil {
			return -3
		}
		return C.int(n)
	})
}

//export SpiceSynth_Player_CreateMIDI
func SpiceSynth_Player_CreateMIDI(sampleRate C.int32_t, midiData *C.uint8_t, midiLen C.int32_t) C.uint64_t {
	if sampleRate <= 0 || midiData == nil || midiLen <= 0 {
		return 0
	}
	data := C.GoBytes(unsafe.Pointer(midiData), C.int(midiLen))
	mf, err := midi.Parse(bytes.NewReader(data))
	if err != nil {
		return 0
	}
	bank, err := op2.DefaultBank()
	if err != nil {
		return 0
	}
	p := player.New(int(sampleRate), bank, mf)
	return C.uint64_t(addPlayerHandle(p))
}

//export SpiceSynth_Player_CreateADL
func SpiceSynth_Player_CreateADL(sampleRate C.int32_t, adlData *C.uint8_t, adlLen C.int32_t) C.uint64_t {
	if sampleRate <= 0 || adlData == nil || adlLen <= 0 {
		return 0
	}
	data := C.GoBytes(unsafe.Pointer(adlData), C.int(adlLen))
	af, err := adl.ParseBytes(data)
	if err != nil {
		return 0
	}
	p := adl.NewPlayer(int(sampleRate), af)
	return C.uint64_t(addPlayerHandle(p))
}

//export SpiceSynth_Player_Destroy
func SpiceSynth_Player_Destroy(h C.uint64_t) C.int {
	handlesMu.Lock()
	p := players[handle(h)]
	if p != nil {
		delete(players, handle(h))
	}
	handlesMu.Unlock()
	if p == nil {
		return -1
	}
	p.Close()
	return 0
}

//export SpiceSynth_Player_Play
func SpiceSynth_Player_Play(h C.uint64_t) C.int {
	return withPlayer(h, func(p pcmReader) C.int {
		switch v := p.(type) {
		case *player.Player:
			v.Play()
		case *adl.Player:
			v.Play()
		default:
			return -2
		}
		return 0
	})
}

//export SpiceSynth_Player_Pause
func SpiceSynth_Player_Pause(h C.uint64_t) C.int {
	return withPlayer(h, func(p pcmReader) C.int {
		switch v := p.(type) {
		case *player.Player:
			v.Pause()
		case *adl.Player:
			v.Pause()
		default:
			return -2
		}
		return 0
	})
}

//export SpiceSynth_Player_Stop
func SpiceSynth_Player_Stop(h C.uint64_t) C.int {
	return withPlayer(h, func(p pcmReader) C.int {
		switch v := p.(type) {
		case *player.Player:
			v.Stop()
		case *adl.Player:
			v.Stop()
		default:
			return -2
		}
		return 0
	})
}

//export SpiceSynth_Player_GetState
func SpiceSynth_Player_GetState(h C.uint64_t) C.int {
	handlesMu.Lock()
	p := players[handle(h)]
	handlesMu.Unlock()
	if p == nil {
		return -1
	}
	switch v := p.(type) {
	case *player.Player:
		return C.int(v.GetState())
	case *adl.Player:
		return C.int(v.GetState())
	default:
		return -2
	}
}

//export SpiceSynth_Player_SetSubsong
func SpiceSynth_Player_SetSubsong(h C.uint64_t, subsong C.int32_t) C.int {
	return withPlayer(h, func(p pcmReader) C.int {
		v, ok := p.(*adl.Player)
		if !ok {
			return -2
		}
		v.SetSubsong(int(subsong))
		return 0
	})
}

//export SpiceSynth_Player_NumSubsongs
func SpiceSynth_Player_NumSubsongs(h C.uint64_t) C.int {
	handlesMu.Lock()
	p := players[handle(h)]
	handlesMu.Unlock()
	if p == nil {
		return -1
	}
	v, ok := p.(*adl.Player)
	if !ok {
		return -2
	}
	return C.int(v.NumSubsongs())
}

//export SpiceSynth_Player_Read
func SpiceSynth_Player_Read(h C.uint64_t, out *C.uint8_t, outLen C.int32_t) C.int {
	if out == nil || outLen <= 0 {
		return -2
	}
	return withPlayer(h, func(p pcmReader) C.int {
		buf := unsafe.Slice((*byte)(unsafe.Pointer(out)), int(outLen))
		n, err := p.Read(buf)
		if err != nil {
			return -3
		}
		return C.int(n)
	})
}
