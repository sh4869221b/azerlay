package layout

type ErrorCode string

const (
	ERR_LAYOUT_JSON        ErrorCode = "ERR_LAYOUT_JSON"
	ERR_LAYOUT_SCHEMA      ErrorCode = "ERR_LAYOUT_SCHEMA"
	ERR_LAYOUT_INVALID     ErrorCode = "ERR_LAYOUT_INVALID"
	ERR_LAYOUT_UNSUPPORTED ErrorCode = "ERR_LAYOUT_UNSUPPORTED"
)

type Error struct {
	Code ErrorCode
	Path string
}

func (e *Error) Error() string {
	return string(e.Code) + " at " + e.Path
}
