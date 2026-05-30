package domain

import "errors"

var (
	ErrProviderUnavailable = errors.New("billing: no payment provider available")
	ErrUnknownPackage      = errors.New("billing: unknown package")
	ErrPurchaseNotFound    = errors.New("billing: purchase not found")
)
