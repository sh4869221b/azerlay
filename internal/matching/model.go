package matching

import "github.com/sh4869221b/azerlay/internal/profile"

type MappingState string

const (
	MappingMapped     MappingState = "mapped"
	MappingUnresolved MappingState = "unresolved"
)

type Binding struct {
	Trigger profile.TriggerKind
	Kind    profile.BindingKind
}

type Control struct {
	ControlIndex int
	RegionID     string
	MappingState MappingState
	Bindings     []Binding
}

type Candidate struct {
	ControlIndex int
	BindingIndex int
	Trigger      profile.TriggerKind
	Kind         profile.BindingKind
	RegionID     string
	MappingState MappingState
}

type Output struct {
	Code                    profile.CanonicalCode
	Candidates              []Candidate
	Ambiguous               bool
	HasUnresolvedCandidates bool
}

type Index struct {
	ScopeSupported     bool
	HasUnknownBindings bool
	Controls           []Control
	Outputs            []Output
}
