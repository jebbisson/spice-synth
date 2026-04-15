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
