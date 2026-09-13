package config

const (
	ERR_CONFIG_NOT_FOUND    = "ERR_CONFIG_NOT_FOUND"
	ERR_CONFIG_INVALID      = "ERR_CONFIG_INVALID"
	WARN_CONFIG_UNKNOWN_KEY = "WARN_CONFIG_UNKNOWN_KEY"
)

type Error struct {
	Code   string
	Stage  string
	Reason string
	Cause  error
}

func (e *Error) Error() string { return e.Code }
func (e *Error) Unwrap() error { return e.Cause }

type Warning struct {
	Code   string
	Line   int
	Column int
	Key    []string
}

func (w Warning) String() string { return w.Code }
