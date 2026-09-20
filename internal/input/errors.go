package input

const (
	ERR_INPUT_EVENT   = "ERR_INPUT_EVENT"
	ERR_INPUT_READ    = "ERR_INPUT_READ"
	ERR_INPUT_DROPPED = "ERR_INPUT_DROPPED"
)

// InputError identifies a failure without retaining raw events or device paths.
type InputError struct {
	Code string
}

func (e *InputError) Error() string { return e.Code }
