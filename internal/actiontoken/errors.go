package actiontoken

import "errors"

var (
	// ErrTokenInvalid signals a malformed token or one whose action does not match the expected action.
	ErrTokenInvalid = errors.New("actiontoken: invalid")
	// ErrTokenExpired signals that the token's expiry has passed.
	ErrTokenExpired = errors.New("actiontoken: expired")
	// ErrTokenAlreadyUsed signals that the token has been consumed and cannot be reused.
	ErrTokenAlreadyUsed = errors.New("actiontoken: already used")
)
