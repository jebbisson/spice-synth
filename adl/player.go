package adl

import adplugadl "github.com/jebbisson/spice-adl-adplug"

const (
	StateStopped = adplugadl.StateStopped
	StatePlaying = adplugadl.StatePlaying
	StatePaused  = adplugadl.StatePaused
	StateDone    = adplugadl.StateDone
)

type ChannelState = adplugadl.ChannelState

type Player struct {
	*adplugadl.Player
}

func NewPlayer(sampleRate int, file *File) *Player {
	return &Player{Player: adplugadl.NewPlayer(sampleRate, toExternalFile(file))}
}

func (p *Player) ChannelStates() []ChannelState {
	return p.Player.ChannelStates()
}

// Close releases the underlying ADL player resources. The Player must not be
// used after calling Close.
func (p *Player) Close() {
	if c, ok := interface{}(p.Player).(interface{ Close() }); ok {
		c.Close()
	}
}
