package domain

import "time"

type User struct {
	Birthdate time.Time
	UnbanTime time.Time
	LastLogin time.Time
	Username  string
	Email     string
	Gender    string
	LastIP    string
	ID        int
	GroupID   int
	State     int
}

func (u *User) IsAdmin() bool  { return u.GroupID == RoleAdmin.GroupID }
func (u *User) IsPlayer() bool { return u.GroupID == 0 }
