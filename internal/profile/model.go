package profile

import "encoding/json"

// SourceMetadata identifies the caller-attributed software export source.
type SourceMetadata struct {
	SoftwareRelease string
	SourceScope     string
}

// RootKind identifies the normalized export root shape.
type RootKind string

const (
	RootBundle RootKind = "bundle"
	RootSingle RootKind = "single"
)

// ProfileBundle is the version-independent normalized export model.
type ProfileBundle struct {
	SchemaVersion int
	Source        SourceMetadata
	RootKind      RootKind
	Profiles      []Profile
	Raw           *RawBundleReference
}

// RawBundleReference retains opaque bundle-level source data.
type RawBundleReference struct {
	Version json.RawMessage
	Unknown map[string]json.RawMessage
}

// Profile is one normalized profile and its ordered controls.
type Profile struct {
	ID       *string
	Name     *string
	Controls []ControlBinding
	Raw      RawProfileReference
}

// RawProfileReference retains opaque profile-level source data.
type RawProfileReference struct {
	ID      json.RawMessage
	Name    json.RawMessage
	Version json.RawMessage
	Unknown map[string]json.RawMessage
}

// ControlBinding is one source control and its trigger outcomes.
type ControlBinding struct {
	Label    *string
	Bindings []TriggerBinding
	Raw      RawBindingReference
}

// RawBindingReference identifies and retains an opaque source input.
type RawBindingReference struct {
	RootKind     RootKind
	ProfileIndex int
	InputIndex   int
	Fields       map[string]json.RawMessage
}

// TriggerKind identifies a trigger slot.
type TriggerKind string

const (
	TriggerSingle  TriggerKind = "single"
	TriggerLong    TriggerKind = "long"
	TriggerDouble  TriggerKind = "double"
	TriggerUnknown TriggerKind = "unknown"
)

// BindingKind identifies interpreted binding semantics.
type BindingKind string

const (
	BindingKeyboard BindingKind = "keyboard"
	BindingUnknown  BindingKind = "unknown"
)

// TriggerBinding is one normalized trigger outcome.
type TriggerBinding struct {
	Trigger           TriggerKind
	Kind              BindingKind
	Actions           []Action
	TriggerDelayMS    *int
	TriggerIntervalMS *int
	ReleaseBehavior   *string
	Unknown           *UnknownBinding
}

// UnknownBinding records a stable reason for uninterpreted semantics.
type UnknownBinding struct {
	Reason string
}

// ActionKind identifies an action family.
type ActionKind string

const ActionKeyboard ActionKind = "keyboard"

// CanonicalCode is a canonical Linux input code name.
type CanonicalCode string

const (
	KEY_U        CanonicalCode = "KEY_U"
	KEY_P        CanonicalCode = "KEY_P"
	KEY_L        CanonicalCode = "KEY_L"
	KEY_I        CanonicalCode = "KEY_I"
	KEY_LEFTCTRL CanonicalCode = "KEY_LEFTCTRL"
)

// Action is one normalized action and its ordered modifiers.
type Action struct {
	Kind      ActionKind
	Code      CanonicalCode
	Modifiers []CanonicalCode
}
