package domain

import "errors"

var (
	ErrStorageUnlocked   = errors.New("market: storage is not locked")
	ErrStashItemNotFound = errors.New("market: stash item not found")
	ErrInsufficientStack = errors.New("market: insufficient item amount")
	ErrStorageFull       = errors.New("market: storage is full")
)
