package domain

import "time"

type User struct {
	UnbanTime time.Time
	LastLogin time.Time
	Username  string
	Email     string
	LastIP    string
	ID        int
	GroupID   int
	State     int
}

func (u *User) IsAdmin() bool {
	return u.GroupID == 99
}
