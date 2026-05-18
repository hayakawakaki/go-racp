package domain

import "time"

type Character struct {
	LastLogin time.Time
	Name      string
	LastMap   string
	ID        int
	AccountID int
	Class     int
	BaseLevel int
	JobLevel  int
	Zeny      int
	Online    bool
}
