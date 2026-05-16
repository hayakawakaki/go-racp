package domain

import "errors"

var (
	ErrCharacterNotFound = errors.New("character not found")
	ErrNotOwner          = errors.New("character does not belong to account")
	ErrCharacterOnline   = errors.New("character is online")
	ErrCooldown          = errors.New("cooldown active")
)
