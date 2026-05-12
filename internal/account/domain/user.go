package domain

import "time"

type User struct {
	Birthdate time.Time
	Username  string
	Password  string
	Email     string
	Gender    string
	ID        int
	State     int
}
