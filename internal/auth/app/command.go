package app

type CreateCommand struct {
	Username string
	Password string
	Email    string
	Gender   string
}

type UpdateCommand struct {
	Password string
	Email    string
}
