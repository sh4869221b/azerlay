package profileadapter

// ErrorCode is a stable machine-readable normalization failure category.
type ErrorCode string

const (
	ERR_IMPORT_UNSUPPORTED_VERSION ErrorCode = "ERR_IMPORT_UNSUPPORTED_VERSION"
	ERR_IMPORT_LIMIT_EXCEEDED      ErrorCode = "ERR_IMPORT_LIMIT_EXCEEDED"
)

// NormalizeError reports a stable code without disclosing source contents.
type NormalizeError struct {
	Code ErrorCode
}

func (e *NormalizeError) Error() string {
	return string(e.Code)
}
