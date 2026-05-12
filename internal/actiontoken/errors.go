package actiontoken

import "errors"

var (
	ErrTokenInvalid     = errors.New("actiontoken: invalid")
	ErrTokenExpired     = errors.New("actiontoken: expired")
	ErrTokenAlreadyUsed = errors.New("actiontoken: already used")
)
