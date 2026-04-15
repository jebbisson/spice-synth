package adl

import adplugadl "github.com/jebbisson/spice-adl-adplug"

const (
	StateStopped = adplugadl.StateStopped
	StatePlaying = adplugadl.StatePlaying
	StatePaused  = adplugadl.StatePaused
	StateDone    = adplugadl.StateDone
)

type Player struct {
	*adplugadl.Player
}

func NewPlayer(sampleRate int, file *File) *Player {
	return &Player{Player: adplugadl.NewPlayer(sampleRate, toExternalFile(file))}
}

func (p *Player) ChannelStates() []ChannelState {
	states := p.Player.ChannelStates()
	out := make([]ChannelState, len(states))
	for i, state := range states {
		out[i] = ChannelState{
			Channel:            state.Channel,
			BytecodeActive:     state.BytecodeActive,
			KeyOn:              state.KeyOn,
			Repeating:          state.Repeating,
			Releasing:          state.Releasing,
			ControlChannel:     state.ControlChannel,
			InstrumentID:       state.InstrumentID,
			RawNote:            state.RawNote,
			Note:               state.Note,
			FrequencyHz:        state.FrequencyHz,
			Duration:           state.Duration,
			InitialDuration:    state.InitialDuration,
			Spacing1:           state.Spacing1,
			Spacing2:           state.Spacing2,
			VolumeModifier:     state.VolumeModifier,
			OutputLevel:        state.OutputLevel,
			CarrierLevel:       state.CarrierLevel,
			ModulatorLevel:     state.ModulatorLevel,
			TwoOperatorCarrier: state.TwoOperatorCarrier,
			Dataptr:            state.Dataptr,
		}
	}
	return out
}
