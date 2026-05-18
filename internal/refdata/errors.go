package refdata

import "errors"

var (
	ErrReloadConflict = errors.New("refdata: reload in progress")
	ErrParseFailed    = errors.New("refdata: parse failed")
)
