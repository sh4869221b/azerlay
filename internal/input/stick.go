package input

import (
	"math"

	"github.com/sh4869221b/azerlay/internal/profile"
)

// XboxStickSource selects two distinct display axes from one explicit node.
type XboxStickSource struct {
	Node int
	X, Y uint16
}

// KeyboardStickSource selects the node for each normalized direction's key.
type KeyboardStickSource struct{ Up, Right, Down, Left int }

// StickSource contains explicit input selectors; it never determines the mode.
type StickSource struct {
	Xbox     XboxStickSource
	Keyboard KeyboardStickSource
}

// AxisState retains raw observation and normalization availability separately.
type AxisState struct {
	Raw          int32
	Normalized   float64
	Known, Valid bool
}

// KeyState distinguishes an unobserved key from a released key.
type KeyState struct{ Down, Known bool }

// StickState is a value-only projection with +X right and +Y down.
type StickState struct {
	Mode                                    profile.StickMode
	Connected                               bool
	Sequence                                uint64
	Generations                             Generations
	AxisX, AxisY                            AxisState
	Up, Right, Down, Left                   KeyState
	X, Y, DirectionX, DirectionY, Intensity float64
	Known                                   bool
}

// ProjectStick projects a supported normalized binding from committed state.
// The caller selects the control and source; no physical orientation is inferred.
func ProjectStick(snapshot *Snapshot, binding profile.StickBinding, source StickSource) StickState {
	state := StickState{Mode: binding.Mode, Connected: snapshot.Connected, Sequence: snapshot.Sequence, Generations: snapshot.Generations}
	if !state.Connected {
		return state
	}
	switch binding.Mode {
	case profile.StickModeXbox:
		if source.Xbox.X == source.Xbox.Y {
			return state
		}
		state.AxisX.Raw, state.AxisX.Normalized, state.AxisX.Known, state.AxisX.Valid = snapshot.NormalizedAbsolute(source.Xbox.Node, source.Xbox.X)
		state.AxisY.Raw, state.AxisY.Normalized, state.AxisY.Known, state.AxisY.Valid = snapshot.NormalizedAbsolute(source.Xbox.Node, source.Xbox.Y)
		state.Known = state.AxisX.Known && state.AxisX.Valid && state.AxisY.Known && state.AxisY.Valid
		if state.Known {
			state.X, state.Y = state.AxisX.Normalized, state.AxisY.Normalized
		}
	case profile.StickModeKeyboard:
		state.Up = projectKey(snapshot, source.Keyboard.Up, binding.KeyboardDirections.Up)
		state.Right = projectKey(snapshot, source.Keyboard.Right, binding.KeyboardDirections.Right)
		state.Down = projectKey(snapshot, source.Keyboard.Down, binding.KeyboardDirections.Down)
		state.Left = projectKey(snapshot, source.Keyboard.Left, binding.KeyboardDirections.Left)
		state.Known = state.Up.Known && state.Right.Known && state.Down.Known && state.Left.Known
		if state.Known {
			if state.Right.Down {
				state.X++
			}
			if state.Left.Down {
				state.X--
			}
			if state.Down.Down {
				state.Y++
			}
			if state.Up.Down {
				state.Y--
			}
		}
	default:
		return state
	}
	length := math.Hypot(state.X, state.Y)
	if length > 0 {
		state.DirectionX, state.DirectionY = state.X/length, state.Y/length
		state.Intensity = min(1, length)
	}
	return state
}

func projectKey(snapshot *Snapshot, node int, code profile.CanonicalCode) KeyState {
	var raw uint16
	switch code {
	case profile.KEY_W:
		raw = 17
	case profile.KEY_D:
		raw = 32
	case profile.KEY_S:
		raw = 31
	case profile.KEY_A:
		raw = 30
	default:
		return KeyState{}
	}
	down, known := snapshot.Key(node, raw)
	return KeyState{Down: down, Known: known}
}
