package app

// GetDTO is the read-model returned by Service methods. It exposes only the
// non-sensitive user fields that callers need (ID, Username, Email) and omits
// the password and gender from the domain model.
type GetDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	ID       int    `json:"id"`
}
