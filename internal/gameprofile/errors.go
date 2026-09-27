package gameprofile

const ERR_GAME_PROFILE_INVALID = "ERR_GAME_PROFILE_INVALID"

type Error struct {
	Code   string
	Stage  string
	Reason string
}

func (e *Error) Error() string { return e.Code }

func invalid(stage, reason string) error {
	return &Error{Code: ERR_GAME_PROFILE_INVALID, Stage: stage, Reason: reason}
}
