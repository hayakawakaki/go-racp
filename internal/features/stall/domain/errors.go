package domain

import "errors"

var (
	ErrSnapshotNotReady = errors.New("vendor: snapshot not ready")
	ErrVendorNotFound   = errors.New("vendor: not found")
)
