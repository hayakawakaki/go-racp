package self

import "net"

type CreateCommand struct {
	Username        string
	Password        string
	PasswordConfirm string
	Email           string
	Gender          string
	Birthdate       string
}

type LoginCommand struct {
	Username  string
	Password  string
	UserAgent string
	IP        net.IP
}
