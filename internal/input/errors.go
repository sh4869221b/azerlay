package input

import (
	"errors"
	"io/fs"
	"syscall"
)

const (
	ERR_INPUT_EVENT   = "ERR_INPUT_EVENT"
	ERR_INPUT_READ    = "ERR_INPUT_READ"
	ERR_INPUT_DROPPED = "ERR_INPUT_DROPPED"
)

// InputError identifies a failure without retaining raw events or device paths.
type InputError struct {
	Code  string
	cause error
}

func (e *InputError) Error() string { return e.Code }

func (e *InputError) Unwrap() error { return e.cause }

func inputReadError(err error) *InputError {
	result := &InputError{Code: ERR_INPUT_READ}
	switch {
	case errors.Is(err, fs.ErrPermission), errors.Is(err, syscall.EPERM):
		result.cause = fs.ErrPermission
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, fs.ErrClosed), errors.Is(err, syscall.ENODEV), errors.Is(err, syscall.ENXIO):
		result.cause = syscall.ENODEV
	}
	return result
}
