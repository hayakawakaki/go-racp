package domain

import "context"

// Repository is the persistence contract for User entities. Implementations
// must return ErrUserNotFound when a lookup yields no result so that callers
// can distinguish "not found" from other errors.
type Repository interface {
	// Create persists a new user and returns the stored record (with ID set).
	Create(ctx context.Context, user *User) (*User, error)

	// GetAll returns all users in the store.
	GetAll(ctx context.Context) ([]User, error)

	// GetByID fetches the user with the given primary-key id, or
	// ErrUserNotFound if no such record exists.
	GetByID(ctx context.Context, id int) (*User, error)

	// GetByUsername fetches the user with the given username, or
	// ErrUserNotFound if no such record exists.
	GetByUsername(ctx context.Context, username string) (*User, error)

	// GetByEmail fetches the user with the given email address, or
	// ErrUserNotFound if no such record exists.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// Update persists changes to an existing user and returns the updated
	// record, or ErrUserNotFound if the user no longer exists.
	Update(ctx context.Context, user *User) (*User, error)

	// Delete removes the user identified by id, or returns ErrUserNotFound if
	// no such user exists.
	Delete(ctx context.Context, id int) error
}
