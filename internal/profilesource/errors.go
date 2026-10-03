package profilesource

import "errors"

const (
	ERR_PROFILE_LOCAL_STATE       = "ERR_PROFILE_LOCAL_STATE"
	ERR_PROFILE_LOCAL_READ        = "ERR_PROFILE_LOCAL_READ"
	ERR_PROFILE_LOCAL_UNSUPPORTED = "ERR_PROFILE_LOCAL_UNSUPPORTED"
	ERR_PROFILE_LOCAL_WATCH       = "ERR_PROFILE_LOCAL_WATCH"
	ERR_PROFILE_STORAGE           = "ERR_PROFILE_STORAGE"
	ERR_PROFILE_NOT_FOUND         = "ERR_PROFILE_NOT_FOUND"
)

// Error exposes a stable privacy-safe code while retaining inspectable causes.
type Error struct {
	Code  string
	Cause error
}

func (e *Error) Error() string { return e.Code }
func (e *Error) Unwrap() error { return e.Cause }

var (
	errUnsafePath       = errors.New("unsafe application path")
	errInvalidSelection = errors.New("invalid import selection")
	errSourceConflict   = errors.New("original or metadata conflict")
)
