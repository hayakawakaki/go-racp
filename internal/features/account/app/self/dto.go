package self

import "time"

type GetDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	ID       int    `json:"id"`
}

type AccountDTO struct {
	RestrictedUntil time.Time
	Username        string
	Email           string
	Verified        bool
	Restricted      bool
}
