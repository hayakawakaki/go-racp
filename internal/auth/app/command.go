// Package app contains the application-layer service and command/query types
// for the auth feature.
package app

// CreateCommand holds the input required to register a new user account.
type CreateCommand struct {
	Username string
	Password string
	Email    string
	Gender   string
}

// UpdateCommand holds the fields that may be changed when updating an existing
// user account. Only Password and Email are mutable through this command.
type UpdateCommand struct {
	Password string
	Email    string
}
