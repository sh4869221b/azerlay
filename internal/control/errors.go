package control

const (
	ERR_CONTROL_REQUEST            = "ERR_CONTROL_REQUEST"
	ERR_CONTROL_VERSION            = "ERR_CONTROL_VERSION"
	ERR_CONTROL_METHOD             = "ERR_CONTROL_METHOD"
	ERR_CONTROL_TOO_LARGE          = "ERR_CONTROL_TOO_LARGE"
	ERR_CONTROL_TIMEOUT            = "ERR_CONTROL_TIMEOUT"
	ERR_CONTROL_UNAVAILABLE        = "ERR_CONTROL_UNAVAILABLE"
	ERR_CONTROL_PERMISSION         = "ERR_CONTROL_PERMISSION"
	ERR_CONTROL_RUNTIME            = "ERR_CONTROL_RUNTIME"
	ERR_PROFILE_AMBIGUOUS          = "ERR_PROFILE_AMBIGUOUS"
	ERR_PROFILE_SOURCE_UNAVAILABLE = "ERR_PROFILE_SOURCE_UNAVAILABLE"
)

// Error contains only public, static diagnostics, never underlying causes.
type Error struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Summary     string `json:"summary"`
	Remediation string `json:"remediation"`
}

func (e *Error) Error() string { return e.Code }

func NewError(code string) *Error {
	e := &Error{Code: code}
	switch code {
	case ERR_CONTROL_REQUEST:
		e.Stage, e.Summary, e.Remediation = "protocol", "Invalid control message.", "Use a valid protocol version 1 message."
	case ERR_CONTROL_VERSION:
		e.Stage, e.Summary, e.Remediation = "protocol", "Unsupported control protocol version.", "Use protocol version 1."
	case ERR_CONTROL_METHOD:
		e.Stage, e.Summary, e.Remediation = "protocol", "Unknown control method.", "Use a documented control method."
	case ERR_CONTROL_TOO_LARGE:
		e.Stage, e.Summary, e.Remediation = "protocol", "Control message exceeds the size limit.", "Keep messages within 65536 bytes."
	case ERR_CONTROL_TIMEOUT:
		e.Stage, e.Summary, e.Remediation = "transport", "Control connection timed out.", "Check instance status before repeating an operation."
	case ERR_CONTROL_PERMISSION:
		e.Stage, e.Summary, e.Remediation = "transport", "Control connection permission denied.", "Use the same user and private runtime directory."
	case ERR_CONTROL_RUNTIME:
		e.Stage, e.Summary, e.Remediation = "path", "Control runtime directory is unavailable or unsafe.", "Provide an owned private XDG_RUNTIME_DIR."
	case "ERR_PROFILE_NOT_FOUND":
		e.Stage, e.Summary, e.Remediation = "selection", "Profile was not found.", "Select an existing imported profile."
	case "ERR_PROFILE_STORAGE":
		e.Stage, e.Summary, e.Remediation = "storage", "Stored profile could not be read.", "Check the imported profile storage."
	case ERR_PROFILE_AMBIGUOUS:
		e.Stage, e.Summary, e.Remediation = "selection", "Profile selection is ambiguous.", "Use a unique profile ID or name."
	case ERR_PROFILE_SOURCE_UNAVAILABLE:
		e.Stage, e.Summary, e.Remediation = "selection", "Profile source is unavailable.", "Configure an imported source and restart."
	case "ERR_PROFILE_LOCAL_UNSUPPORTED":
		e.Stage, e.Summary, e.Remediation = "profile", "Stored local definition is unsupported.", "Export the profile with Azeron Software 2.0.2 and import the official export."
	case "ERR_PROFILE_LOCAL_READ":
		e.Stage, e.Summary, e.Remediation = "profile", "Selected local definition could not be read safely.", "Check the configured local selection or import an official export."
	case "ERR_PROFILE_LOCAL_WATCH":
		e.Stage, e.Summary, e.Remediation = "watch", "Local profile watching is unavailable.", "Check the selected profile directory and restart, or import an official export."
	case "ERR_PROFILE_LOCAL_STATE":
		e.Stage, e.Summary, e.Remediation = "state", "Local last-good state is unavailable or could not be saved.", "Check Azerlay state directory permissions and keep it outside the local store."
	default:
		e.Code = ERR_CONTROL_UNAVAILABLE
		e.Stage, e.Summary, e.Remediation = "transport", "Control instance is unavailable.", "Check that the instance is running."
	}
	return e
}
