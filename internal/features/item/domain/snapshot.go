package domain

import "time"

type Snapshot struct {
	LoadedAt    time.Time
	ByID        map[int]*Item
	ByName      map[string]*Item
	Sorted      []*Item
	SourceCount int
}

func EmptySnapshot() *Snapshot {
	return &Snapshot{
		LoadedAt: time.Time{},
		ByID:     map[int]*Item{},
		ByName:   map[string]*Item{},
		Sorted:   nil,
	}
}
