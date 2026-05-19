package app

import "time"

type NewsItem struct {
	CreatedAt       time.Time
	Title           string
	Body            string
	Category        string
	CategoryDisplay string
	ID              int64
}
