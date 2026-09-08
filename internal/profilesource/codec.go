package profilesource

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sh4869221b/azerlay/internal/profile"
)

// allow: SIZE_OK - Task 2 confines the complete schema-1 codec to this file;
// the explicit DTO table, conversions and boundary checks share one disk contract.
const (
	storageSchemaVersion = 1
	modelSchemaVersion   = 1
	// Parser/adapter semantic changes must bump the relevant interpretation revision.
	decoderVersion    = "1"
	normalizerVersion = "1"
	maxIndexBytes     = 16 << 20
	maxCacheBytes     = 512 << 20
	// These mirror the existing admitted export structural bounds.
	maxModelProfiles = 512
	maxModelControls = 256
)

var errCodecShape = errors.New("invalid storage shape")
var errCodecLimit = errors.New("storage size limit exceeded")

type codecError struct{ cause error }

func (e *codecError) Error() string { return "invalid storage codec" }
func (e *codecError) Unwrap() error { return e.cause }

type diskOrigin struct {
	Kind string  `json:"kind"`
	Path *string `json:"path"`
}
type diskSource struct {
	SourceHash         string     `json:"source_hash"`
	SoftwareRelease    string     `json:"software_release"`
	SourceScope        string     `json:"source_scope"`
	InputKind          string     `json:"input_kind"`
	Origin             diskOrigin `json:"origin"`
	ImportedAt         string     `json:"imported_at"`
	DecoderVersion     string     `json:"decoder_version"`
	NormalizerVersion  string     `json:"normalizer_version"`
	ModelSchemaVersion int        `json:"model_schema_version"`
	ProfileCount       int        `json:"profile_count"`
	ExportVersion      *string    `json:"export_version"`
}
type diskSelection struct {
	SourceHash   string `json:"source_hash"`
	ProfileIndex int    `json:"profile_index"`
}
type diskIndex struct {
	SchemaVersion int            `json:"schema_version"`
	Sources       []diskSource   `json:"sources"`
	Selected      *diskSelection `json:"selected"`
}
type diskBundleEnvelope struct {
	SchemaVersion      int        `json:"schema_version"`
	SourceHash         string     `json:"source_hash"`
	DecoderVersion     string     `json:"decoder_version"`
	NormalizerVersion  string     `json:"normalizer_version"`
	ModelSchemaVersion int        `json:"model_schema_version"`
	Bundle             diskBundle `json:"bundle"`
}
type diskProfileEnvelope struct {
	SchemaVersion      int         `json:"schema_version"`
	SourceHash         string      `json:"source_hash"`
	DecoderVersion     string      `json:"decoder_version"`
	NormalizerVersion  string      `json:"normalizer_version"`
	ModelSchemaVersion int         `json:"model_schema_version"`
	ProfileIndex       int         `json:"profile_index"`
	Profile            diskProfile `json:"profile"`
}
type diskBundle struct {
	SchemaVersion int              `json:"schema_version"`
	Source        diskMetadata     `json:"source"`
	RootKind      profile.RootKind `json:"root_kind"`
	Profiles      []diskProfile    `json:"profiles"`
	Raw           *diskBundleRaw   `json:"raw"`
}
type diskMetadata struct {
	SoftwareRelease string `json:"software_release"`
	SourceScope     string `json:"source_scope"`
}
type diskBundleRaw struct {
	Version *string       `json:"version"`
	Unknown diskRawFields `json:"unknown"`
}
type diskProfile struct {
	ID       *string        `json:"id"`
	Name     *string        `json:"name"`
	Controls []diskControl  `json:"controls"`
	Raw      diskProfileRaw `json:"raw"`
}
type diskProfileRaw struct {
	ID      *string       `json:"id"`
	Name    *string       `json:"name"`
	Version *string       `json:"version"`
	Unknown diskRawFields `json:"unknown"`
}
type diskControl struct {
	Label    *string        `json:"label"`
	Bindings []diskTrigger  `json:"bindings"`
	Raw      diskBindingRaw `json:"raw"`
}
type diskBindingRaw struct {
	RootKind     profile.RootKind `json:"root_kind"`
	ProfileIndex diskPosition     `json:"profile_index"`
	InputIndex   diskPosition     `json:"input_index"`
	Fields       diskRawFields    `json:"fields"`
}
type diskTrigger struct {
	Trigger           profile.TriggerKind `json:"trigger"`
	Kind              profile.BindingKind `json:"kind"`
	Actions           []diskAction        `json:"actions"`
	TriggerDelayMS    *int                `json:"trigger_delay_ms"`
	TriggerIntervalMS *int                `json:"trigger_interval_ms"`
	ReleaseBehavior   *string             `json:"release_behavior"`
	Unknown           *diskUnknown        `json:"unknown"`
}
type diskUnknown struct {
	Reason string `json:"reason"`
}
type diskAction struct {
	Kind      profile.ActionKind      `json:"kind"`
	Code      profile.CanonicalCode   `json:"code"`
	Modifiers []profile.CanonicalCode `json:"modifiers"`
}
type diskRawFields map[string]string
type diskPosition int

// decodeObject requires every known field, including nullable carriers. Filtering
// also prevents encoding/json's case-insensitive aliases from overriding a key.
// Unknown keys are ignored only after full-stream duplicate-key checking.
func decodeObject(data []byte, target any, fields string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	if object == nil {
		return errCodecShape
	}
	known := make(map[string]json.RawMessage)
	for _, name := range strings.Fields(fields) {
		value, ok := object[name]
		if !ok {
			return errCodecShape
		}
		known[name] = value
	}
	filtered, err := json.Marshal(known)
	if err != nil {
		return err
	}
	return json.Unmarshal(filtered, target)
}
func (v *diskOrigin) UnmarshalJSON(b []byte) error {
	type plain diskOrigin
	return decodeObject(b, (*plain)(v), "kind path")
}
func (v *diskSource) UnmarshalJSON(b []byte) error {
	type plain diskSource
	return decodeObject(b, (*plain)(v), "source_hash software_release source_scope input_kind origin imported_at decoder_version normalizer_version model_schema_version profile_count export_version")
}
func (v *diskSelection) UnmarshalJSON(b []byte) error {
	type plain diskSelection
	return decodeObject(b, (*plain)(v), "source_hash profile_index")
}
func (v *diskIndex) UnmarshalJSON(b []byte) error {
	type plain diskIndex
	return decodeObject(b, (*plain)(v), "schema_version sources selected")
}
func (v *diskBundleEnvelope) UnmarshalJSON(b []byte) error {
	type plain diskBundleEnvelope
	return decodeObject(b, (*plain)(v), "schema_version source_hash decoder_version normalizer_version model_schema_version bundle")
}
func (v *diskProfileEnvelope) UnmarshalJSON(b []byte) error {
	type plain diskProfileEnvelope
	return decodeObject(b, (*plain)(v), "schema_version source_hash decoder_version normalizer_version model_schema_version profile_index profile")
}
func (v *diskBundle) UnmarshalJSON(b []byte) error {
	type plain diskBundle
	return decodeObject(b, (*plain)(v), "schema_version source root_kind profiles raw")
}
func (v *diskMetadata) UnmarshalJSON(b []byte) error {
	type plain diskMetadata
	return decodeObject(b, (*plain)(v), "software_release source_scope")
}
func (v *diskBundleRaw) UnmarshalJSON(b []byte) error {
	type plain diskBundleRaw
	return decodeObject(b, (*plain)(v), "version unknown")
}
func (v *diskProfile) UnmarshalJSON(b []byte) error {
	type plain diskProfile
	return decodeObject(b, (*plain)(v), "id name controls raw")
}
func (v *diskProfileRaw) UnmarshalJSON(b []byte) error {
	type plain diskProfileRaw
	return decodeObject(b, (*plain)(v), "id name version unknown")
}
func (v *diskControl) UnmarshalJSON(b []byte) error {
	type plain diskControl
	return decodeObject(b, (*plain)(v), "label bindings raw")
}
func (v *diskBindingRaw) UnmarshalJSON(b []byte) error {
	type plain diskBindingRaw
	return decodeObject(b, (*plain)(v), "root_kind profile_index input_index fields")
}
func (v *diskTrigger) UnmarshalJSON(b []byte) error {
	type plain diskTrigger
	return decodeObject(b, (*plain)(v), "trigger kind actions trigger_delay_ms trigger_interval_ms release_behavior unknown")
}
func (v *diskUnknown) UnmarshalJSON(b []byte) error {
	type plain diskUnknown
	return decodeObject(b, (*plain)(v), "reason")
}
func (v *diskAction) UnmarshalJSON(b []byte) error {
	type plain diskAction
	return decodeObject(b, (*plain)(v), "kind code modifiers")
}
func (v *diskPosition) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return errCodecShape
	}
	type plain diskPosition
	return json.Unmarshal(b, (*plain)(v))
}
func (v *diskRawFields) UnmarshalJSON(b []byte) error {
	var fields map[string]*string
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	if fields == nil {
		*v = nil
		return nil
	}
	result := make(diskRawFields, len(fields))
	for key, value := range fields {
		if value == nil {
			return errCodecShape
		}
		result[key] = *value
	}
	*v = result
	return nil
}

// scanStorageValue scans JSON syntax, not the contents of opaque token strings.
func scanStorageValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return errCodecShape
			}
			seen[key] = true
			if err := scanStorageValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanStorageValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errCodecShape
	}
	_, err = decoder.Token()
	return err
}
func readStorage(reader io.Reader, target any, limit int64) error {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return &codecError{err}
	}
	if int64(len(data)) > limit {
		return &codecError{errCodecLimit}
	}
	if !utf8.Valid(data) || !json.Valid(data) {
		return &codecError{errCodecShape}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanStorageValue(decoder); err != nil {
		return &codecError{err}
	}
	if err := json.Unmarshal(data, target); err != nil {
		return &codecError{err}
	}
	return nil
}
func marshalStorage(value any, limit int) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, &codecError{err}
	}
	if len(data) > limit {
		return nil, &codecError{errCodecLimit}
	}
	return data, nil
}
func validSourceHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	for _, c := range hash {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func validDiskSource(source diskSource) bool {
	if !validSourceHash(source.SourceHash) || source.SoftwareRelease == "" || source.SourceScope == "" || source.DecoderVersion == "" || source.NormalizerVersion == "" || source.ModelSchemaVersion < 1 || source.ProfileCount < 1 || source.ProfileCount > maxModelProfiles {
		return false
	}
	switch source.InputKind {
	case "text", "reader":
	default:
		return false
	}
	switch source.Origin.Kind {
	case "file", "stdin", "text":
	default:
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, source.ImportedAt)
	return err == nil && strings.HasSuffix(source.ImportedAt, "Z") && !strings.Contains(source.ImportedAt, ",")
}
func validDiskIndex(index diskIndex) bool {
	if index.SchemaVersion != storageSchemaVersion || index.Sources == nil {
		return false
	}
	counts := make(map[string]int, len(index.Sources))
	for _, source := range index.Sources {
		if !validDiskSource(source) || counts[source.SourceHash] != 0 {
			return false
		}
		counts[source.SourceHash] = source.ProfileCount
	}
	if index.Selected == nil {
		return true
	}
	return validSourceHash(index.Selected.SourceHash) && index.Selected.ProfileIndex > 0 && index.Selected.ProfileIndex <= counts[index.Selected.SourceHash]
}
func encodeIndex(index diskIndex) ([]byte, error) {
	if !validDiskIndex(index) {
		return nil, &codecError{errCodecShape}
	}
	return marshalStorage(index, maxIndexBytes)
}
func decodeIndex(reader io.Reader) (diskIndex, error) {
	var index diskIndex
	if err := readStorage(reader, &index, maxIndexBytes); err != nil {
		return diskIndex{}, err
	}
	if !validDiskIndex(index) {
		return diskIndex{}, &codecError{errCodecShape}
	}
	return index, nil
}

func tokenToDisk(raw json.RawMessage) *string {
	if raw == nil {
		return nil
	}
	token := string(raw)
	return &token
}
func tokenFromDisk(token *string) json.RawMessage {
	if token == nil {
		return nil
	}
	return json.RawMessage(*token)
}
func fieldsToDisk(fields map[string]json.RawMessage) diskRawFields {
	if fields == nil {
		return nil
	}
	result := make(diskRawFields, len(fields))
	for key, value := range fields {
		result[key] = string(value)
	}
	return result
}
func fieldsFromDisk(fields diskRawFields) map[string]json.RawMessage {
	if fields == nil {
		return nil
	}
	result := make(map[string]json.RawMessage, len(fields))
	for key, value := range fields {
		result[key] = json.RawMessage(value)
	}
	return result
}
func profileToDisk(value profile.Profile) diskProfile {
	result := diskProfile{ID: value.ID, Name: value.Name, Raw: diskProfileRaw{ID: tokenToDisk(value.Raw.ID), Name: tokenToDisk(value.Raw.Name), Version: tokenToDisk(value.Raw.Version), Unknown: fieldsToDisk(value.Raw.Unknown)}}
	if value.Controls != nil {
		result.Controls = make([]diskControl, len(value.Controls))
	}
	for i, control := range value.Controls {
		out := diskControl{Label: control.Label, Raw: diskBindingRaw{RootKind: control.Raw.RootKind, ProfileIndex: diskPosition(control.Raw.ProfileIndex), InputIndex: diskPosition(control.Raw.InputIndex), Fields: fieldsToDisk(control.Raw.Fields)}}
		if control.Bindings != nil {
			out.Bindings = make([]diskTrigger, len(control.Bindings))
		}
		for j, binding := range control.Bindings {
			trigger := diskTrigger{Trigger: binding.Trigger, Kind: binding.Kind, TriggerDelayMS: binding.TriggerDelayMS, TriggerIntervalMS: binding.TriggerIntervalMS, ReleaseBehavior: binding.ReleaseBehavior}
			if binding.Unknown != nil {
				trigger.Unknown = &diskUnknown{Reason: binding.Unknown.Reason}
			}
			if binding.Actions != nil {
				trigger.Actions = make([]diskAction, len(binding.Actions))
			}
			for k, action := range binding.Actions {
				trigger.Actions[k] = diskAction{Kind: action.Kind, Code: action.Code, Modifiers: action.Modifiers}
			}
			out.Bindings[j] = trigger
		}
		result.Controls[i] = out
	}
	return result
}
func profileFromDisk(value diskProfile) profile.Profile {
	result := profile.Profile{ID: value.ID, Name: value.Name, Raw: profile.RawProfileReference{ID: tokenFromDisk(value.Raw.ID), Name: tokenFromDisk(value.Raw.Name), Version: tokenFromDisk(value.Raw.Version), Unknown: fieldsFromDisk(value.Raw.Unknown)}}
	if value.Controls != nil {
		result.Controls = make([]profile.ControlBinding, len(value.Controls))
	}
	for i, control := range value.Controls {
		out := profile.ControlBinding{Label: control.Label, Raw: profile.RawBindingReference{RootKind: control.Raw.RootKind, ProfileIndex: int(control.Raw.ProfileIndex), InputIndex: int(control.Raw.InputIndex), Fields: fieldsFromDisk(control.Raw.Fields)}}
		if control.Bindings != nil {
			out.Bindings = make([]profile.TriggerBinding, len(control.Bindings))
		}
		for j, binding := range control.Bindings {
			trigger := profile.TriggerBinding{Trigger: binding.Trigger, Kind: binding.Kind, TriggerDelayMS: binding.TriggerDelayMS, TriggerIntervalMS: binding.TriggerIntervalMS, ReleaseBehavior: binding.ReleaseBehavior}
			if binding.Unknown != nil {
				trigger.Unknown = &profile.UnknownBinding{Reason: binding.Unknown.Reason}
			}
			if binding.Actions != nil {
				trigger.Actions = make([]profile.Action, len(binding.Actions))
			}
			for k, action := range binding.Actions {
				trigger.Actions[k] = profile.Action{Kind: action.Kind, Code: action.Code, Modifiers: action.Modifiers}
			}
			out.Bindings[j] = trigger
		}
		result.Controls[i] = out
	}
	return result
}
func bundleToDisk(bundle profile.ProfileBundle) diskBundle {
	result := diskBundle{SchemaVersion: bundle.SchemaVersion, Source: diskMetadata(bundle.Source), RootKind: bundle.RootKind}
	if bundle.Raw != nil {
		result.Raw = &diskBundleRaw{Version: tokenToDisk(bundle.Raw.Version), Unknown: fieldsToDisk(bundle.Raw.Unknown)}
	}
	if bundle.Profiles != nil {
		result.Profiles = make([]diskProfile, len(bundle.Profiles))
	}
	for i, value := range bundle.Profiles {
		result.Profiles[i] = profileToDisk(value)
	}
	return result
}
func bundleFromDisk(bundle diskBundle) profile.ProfileBundle {
	result := profile.ProfileBundle{SchemaVersion: bundle.SchemaVersion, Source: profile.SourceMetadata(bundle.Source), RootKind: bundle.RootKind}
	if bundle.Raw != nil {
		result.Raw = &profile.RawBundleReference{Version: tokenFromDisk(bundle.Raw.Version), Unknown: fieldsFromDisk(bundle.Raw.Unknown)}
	}
	if bundle.Profiles != nil {
		result.Profiles = make([]profile.Profile, len(bundle.Profiles))
	}
	for i, value := range bundle.Profiles {
		result.Profiles[i] = profileFromDisk(value)
	}
	return result
}
func validTrigger(binding profile.TriggerBinding) bool {
	switch binding.Kind {
	case profile.BindingUnknown:
		return binding.Actions == nil && binding.TriggerDelayMS == nil && binding.TriggerIntervalMS == nil && binding.ReleaseBehavior == nil && binding.Unknown != nil && binding.Unknown.Reason == "unmapped_binding"
	case profile.BindingKeyboard:
		if binding.Unknown != nil || len(binding.Actions) == 0 {
			return false
		}
		for _, action := range binding.Actions {
			if action.Kind != profile.ActionKeyboard {
				return false
			}
			switch action.Code {
			case profile.KEY_U, profile.KEY_P, profile.KEY_L, profile.KEY_I:
			default:
				return false
			}
			for _, modifier := range action.Modifiers {
				if modifier != profile.KEY_LEFTCTRL {
					return false
				}
			}
		}
		switch binding.Trigger {
		case profile.TriggerSingle:
			return binding.TriggerDelayMS == nil && binding.TriggerIntervalMS == nil && binding.ReleaseBehavior != nil && *binding.ReleaseBehavior == "regular"
		case profile.TriggerLong:
			return binding.TriggerDelayMS != nil && *binding.TriggerDelayMS >= 0 && binding.TriggerIntervalMS == nil && binding.ReleaseBehavior != nil && *binding.ReleaseBehavior == "regular"
		case profile.TriggerDouble:
			return binding.TriggerDelayMS == nil && binding.TriggerIntervalMS != nil && *binding.TriggerIntervalMS >= 0 && binding.ReleaseBehavior == nil
		case profile.TriggerUnknown:
			return false
		default:
			return false
		}
	default:
		return false
	}
}
func validProfile(value profile.Profile, root profile.RootKind, index int) bool {
	if value.Controls == nil || len(value.Controls) > maxModelControls {
		return false
	}
	for i, control := range value.Controls {
		if control.Raw.RootKind != root || control.Raw.ProfileIndex != index || control.Raw.InputIndex != i {
			return false
		}
		switch len(control.Bindings) {
		case 1:
			if control.Bindings[0].Trigger != profile.TriggerUnknown {
				return false
			}
		case 3:
			if control.Bindings[0].Trigger != profile.TriggerSingle || control.Bindings[1].Trigger != profile.TriggerLong || control.Bindings[2].Trigger != profile.TriggerDouble {
				return false
			}
		default:
			return false
		}
		for _, binding := range control.Bindings {
			if !validTrigger(binding) {
				return false
			}
		}
	}
	return true
}
func validBundle(bundle profile.ProfileBundle, source diskSource) bool {
	if !validDiskSource(source) || bundle.SchemaVersion != modelSchemaVersion || bundle.Source.SoftwareRelease != source.SoftwareRelease || bundle.Source.SourceScope != source.SourceScope || len(bundle.Profiles) != source.ProfileCount {
		return false
	}
	var version *string
	switch bundle.RootKind {
	case profile.RootSingle:
		if len(bundle.Profiles) != 1 || bundle.Raw != nil {
			return false
		}
		version = tokenToDisk(bundle.Profiles[0].Raw.Version)
	case profile.RootBundle:
		if bundle.Raw == nil {
			return false
		}
		version = tokenToDisk(bundle.Raw.Version)
	default:
		return false
	}
	if !reflect.DeepEqual(version, source.ExportVersion) {
		return false
	}
	for i, value := range bundle.Profiles {
		if !validProfile(value, bundle.RootKind, i) {
			return false
		}
	}
	return true
}
func encodeBundleCache(source diskSource, bundle profile.ProfileBundle) ([]byte, error) {
	if !validBundle(bundle, source) {
		return nil, &codecError{errCodecShape}
	}
	return marshalStorage(diskBundleEnvelope{SchemaVersion: storageSchemaVersion, SourceHash: source.SourceHash, DecoderVersion: decoderVersion, NormalizerVersion: normalizerVersion, ModelSchemaVersion: modelSchemaVersion, Bundle: bundleToDisk(bundle)}, maxCacheBytes)
}
func decodeBundleCache(reader io.Reader, source diskSource) (profile.ProfileBundle, error) {
	var envelope diskBundleEnvelope
	if err := readStorage(reader, &envelope, maxCacheBytes); err != nil {
		return profile.ProfileBundle{}, err
	}
	if envelope.SchemaVersion != storageSchemaVersion || envelope.SourceHash != source.SourceHash || envelope.DecoderVersion != decoderVersion || envelope.NormalizerVersion != normalizerVersion || envelope.ModelSchemaVersion != modelSchemaVersion {
		return profile.ProfileBundle{}, &codecError{errCodecShape}
	}
	bundle := bundleFromDisk(envelope.Bundle)
	if !validBundle(bundle, source) {
		return profile.ProfileBundle{}, &codecError{errCodecShape}
	}
	return bundle, nil
}
func encodeProfileCache(source diskSource, bundle profile.ProfileBundle, index int) ([]byte, error) {
	if !validBundle(bundle, source) || index < 1 || index > source.ProfileCount {
		return nil, &codecError{errCodecShape}
	}
	return marshalStorage(diskProfileEnvelope{SchemaVersion: storageSchemaVersion, SourceHash: source.SourceHash, DecoderVersion: decoderVersion, NormalizerVersion: normalizerVersion, ModelSchemaVersion: modelSchemaVersion, ProfileIndex: index, Profile: profileToDisk(bundle.Profiles[index-1])}, maxCacheBytes)
}
func decodeProfileCache(reader io.Reader, source diskSource, index int) (profile.Profile, error) {
	var envelope diskProfileEnvelope
	if err := readStorage(reader, &envelope, maxCacheBytes); err != nil {
		return profile.Profile{}, err
	}
	if !validDiskSource(source) || index < 1 || index > source.ProfileCount || envelope.ProfileIndex != index || envelope.SchemaVersion != storageSchemaVersion || envelope.SourceHash != source.SourceHash || envelope.DecoderVersion != decoderVersion || envelope.NormalizerVersion != normalizerVersion || envelope.ModelSchemaVersion != modelSchemaVersion {
		return profile.Profile{}, &codecError{errCodecShape}
	}
	value := profileFromDisk(envelope.Profile)
	root := profile.RootBundle
	if len(value.Controls) > 0 {
		root = value.Controls[0].Raw.RootKind
	}
	switch root {
	case profile.RootBundle:
	case profile.RootSingle:
		if source.ProfileCount != 1 || index != 1 {
			return profile.Profile{}, &codecError{errCodecShape}
		}
	default:
		return profile.Profile{}, &codecError{errCodecShape}
	}
	if !validProfile(value, root, index-1) {
		return profile.Profile{}, &codecError{errCodecShape}
	}
	return value, nil
}

// decodeCacheSet treats the bundle as authority and requires every pN artifact.
// Callers close their readers and map any codec error to a whole-set cache miss.
func decodeCacheSet(source diskSource, bundleReader io.Reader, profileReaders []io.Reader) (profile.ProfileBundle, error) {
	if len(profileReaders) != source.ProfileCount {
		return profile.ProfileBundle{}, &codecError{errCodecShape}
	}
	bundle, err := decodeBundleCache(bundleReader, source)
	if err != nil {
		return profile.ProfileBundle{}, err
	}
	for i, reader := range profileReaders {
		value, err := decodeProfileCache(reader, source, i+1)
		if err != nil {
			return profile.ProfileBundle{}, err
		}
		if !reflect.DeepEqual(value, bundle.Profiles[i]) {
			return profile.ProfileBundle{}, &codecError{errCodecShape}
		}
	}
	return bundle, nil
}
