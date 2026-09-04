package profileraw

// ErrorCode is a stable machine-readable raw import failure category.
type ErrorCode string

const (
	ERR_IMPORT_ROOT                ErrorCode = "ERR_IMPORT_ROOT"
	ERR_IMPORT_LIMIT_EXCEEDED      ErrorCode = "ERR_IMPORT_LIMIT_EXCEEDED"
	ERR_IMPORT_UNSUPPORTED_VERSION ErrorCode = "ERR_IMPORT_UNSUPPORTED_VERSION"
)

// ParseError reports a stable code without disclosing source contents.
type ParseError struct {
	Code ErrorCode
}

func (e *ParseError) Error() string {
	return string(e.Code)
}
