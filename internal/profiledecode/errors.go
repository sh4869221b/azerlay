package profiledecode

// ErrorCode is a stable machine-readable decode failure category.
type ErrorCode string

const (
	ERR_IMPORT_ENCODING        ErrorCode = "ERR_IMPORT_ENCODING"
	ERR_IMPORT_LZMA_HEADER     ErrorCode = "ERR_IMPORT_LZMA_HEADER"
	ERR_IMPORT_LZMA_CORRUPT    ErrorCode = "ERR_IMPORT_LZMA_CORRUPT"
	ERR_IMPORT_MSGPACK_TYPE    ErrorCode = "ERR_IMPORT_MSGPACK_TYPE"
	ERR_IMPORT_LENGTH_MISMATCH ErrorCode = "ERR_IMPORT_LENGTH_MISMATCH"
	ERR_IMPORT_UTF8            ErrorCode = "ERR_IMPORT_UTF8"
	ERR_IMPORT_JSON            ErrorCode = "ERR_IMPORT_JSON"
	ERR_IMPORT_ROOT            ErrorCode = "ERR_IMPORT_ROOT"
	ERR_IMPORT_LIMIT_EXCEEDED  ErrorCode = "ERR_IMPORT_LIMIT_EXCEEDED"
)

// DecodeError reports a stable code without disclosing source contents.
type DecodeError struct {
	Code  ErrorCode
	cause error
}

func (e *DecodeError) Error() string {
	return string(e.Code)
}

func (e *DecodeError) Unwrap() error {
	return e.cause
}

func newDecodeError(code ErrorCode, cause error) *DecodeError {
	return &DecodeError{Code: code, cause: cause}
}
