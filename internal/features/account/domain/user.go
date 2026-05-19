package domain

import "time"

type User struct {
	Birthdate time.Time
	UnbanTime time.Time
	Username  string
	Password  string
	Email     string
	Gender    string
	ID        int
	GroupID   int
	State     int
}
