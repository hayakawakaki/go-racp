package domain

import "errors"

var (
	ErrGuildNotFound       = errors.New("guild not found")
	ErrEmblemEmpty         = errors.New("guild emblem empty")
	ErrEmblemUnknownFormat = errors.New("guild emblem unknown format")
)
