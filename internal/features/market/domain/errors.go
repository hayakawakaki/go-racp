package domain

import "errors"

var (
	ErrStorageUnlocked   = errors.New("market: storage is not locked")
	ErrStashItemNotFound = errors.New("market: stash item not found")
	ErrInsufficientStack = errors.New("market: insufficient item amount")
	ErrStorageFull       = errors.New("market: storage is full")
	ErrNotTradable       = errors.New("market: item is not tradable")
	ErrNonStackable      = errors.New("market: item is not stackable")
	ErrInsufficientFunds = errors.New("market: insufficient funds")
	ErrHoldNotFound      = errors.New("market: currency hold not found")
	ErrSelfTrade         = errors.New("market: payer and payee are the same account")
	ErrInvalidSettlement = errors.New("market: payee amount exceeds held amount")
	ErrInvalidAmount     = errors.New("market: amount must be non negative")
	ErrListingNotFound   = errors.New("market: listing not found")
	ErrListingInactive   = errors.New("market: listing is not active")
	ErrInsufficientUnits = errors.New("market: not enough units remaining")
	ErrWantMismatch      = errors.New("market: offered item does not match the want")
	ErrItemBlacklisted   = errors.New("market: item is not tradeable")
	ErrInvalidOffer      = errors.New("market: invalid offer")
)
