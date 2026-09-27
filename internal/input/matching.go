package input

import (
	"github.com/sh4869221b/azerlay/internal/matching"
	"github.com/sh4869221b/azerlay/internal/profile"
)

type MatchingOutput struct {
	Code                    profile.CanonicalCode
	Candidates              []matching.Candidate
	Ambiguous               bool
	HasUnresolvedCandidates bool
	Known                   bool
	Down                    bool
}

type MatchingState struct {
	Connected          bool
	Sequence           uint64
	Generations        Generations
	ScopeSupported     bool
	HasUnknownBindings bool
	Controls           []matching.Control
	Outputs            []MatchingOutput
}

func ProjectMatching(snapshot *Snapshot, index matching.Index, sources map[profile.CanonicalCode]int) MatchingState {
	state := MatchingState{
		Connected: snapshot.Connected, Sequence: snapshot.Sequence, Generations: snapshot.Generations,
		ScopeSupported: index.ScopeSupported, HasUnknownBindings: index.HasUnknownBindings,
		Controls: make([]matching.Control, len(index.Controls)), Outputs: make([]MatchingOutput, len(index.Outputs)),
	}
	for i, control := range index.Controls {
		control.Bindings = append([]matching.Binding(nil), control.Bindings...)
		state.Controls[i] = control
	}
	for i, output := range index.Outputs {
		projected := MatchingOutput{
			Code: output.Code, Candidates: append([]matching.Candidate(nil), output.Candidates...),
			Ambiguous: output.Ambiguous, HasUnresolvedCandidates: output.HasUnresolvedCandidates,
		}
		if snapshot.Connected {
			if node, selected := sources[output.Code]; selected {
				if raw, known := matchingRawCode(output.Code); known {
					projected.Down, projected.Known = snapshot.Key(node, raw)
				}
			}
		}
		state.Outputs[i] = projected
	}
	return state
}

func matchingRawCode(code profile.CanonicalCode) (uint16, bool) {
	switch code {
	case profile.KEY_W:
		return 17, true
	case profile.KEY_T:
		return 20, true
	case profile.KEY_A:
		return 30, true
	case profile.KEY_S:
		return 31, true
	case profile.KEY_D:
		return 32, true
	case profile.KEY_U:
		return 22, true
	case profile.KEY_P:
		return 25, true
	case profile.KEY_L:
		return 38, true
	case profile.KEY_I:
		return 23, true
	case profile.KEY_LEFTCTRL:
		return 29, true
	default:
		return 0, false
	}
}
