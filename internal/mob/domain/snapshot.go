package domain

import "time"

type Snapshot struct {
	LoadedAt    time.Time
	ByID        map[int]*Mob
	ByAegis     map[string]*Mob
	DroppedBy   map[string][]DropOf
	Sorted      []*Mob
	SourceCount int
}

func EmptySnapshot() *Snapshot {
	return &Snapshot{
		LoadedAt:  time.Time{},
		ByID:      map[int]*Mob{},
		ByAegis:   map[string]*Mob{},
		DroppedBy: map[string][]DropOf{},
		Sorted:    nil,
	}
}
