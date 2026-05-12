package app

type GetDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	ID       int    `json:"id"`
}

type AccountDTO struct {
	Username string
	Email    string
	Verified bool
}
