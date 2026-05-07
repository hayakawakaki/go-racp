package domain

// User is the core entity representing a registered account. Password is
// stored as-received (hashing is expected to happen before this layer) and
// should never be exposed in read DTOs.
type User struct {
	Username string
	Password string
	Email    string
	Gender   string
	ID       int
}
