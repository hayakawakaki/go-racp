package app

import (
	"fmt"
	"time"
)

type CharacterDTO struct {
	LookCDUntil   time.Time
	LocCDUntil    time.Time
	Name          string
	Gender        string
	JobName       string
	CurrentMap    string
	SaveMap       string
	ID            int
	Slot          int
	Zeny          int
	JobID         int
	BaseLevel     int
	JobLevel      int
	HairStyle     int
	HairColor     int
	ClothesColor  int
	BodyID        int
	CurrentX      int
	CurrentY      int
	SaveX         int
	SaveY         int
	CostumeTop    int
	CostumeMid    int
	CostumeBottom int
	CostumeRobe   int
	Online        bool
}

func (d CharacterDTO) LookOnCooldown(now time.Time) bool {
	return !d.LookCDUntil.IsZero() && now.Before(d.LookCDUntil)
}

func (d CharacterDTO) LocationOnCooldown(now time.Time) bool {
	return !d.LocCDUntil.IsZero() && now.Before(d.LocCDUntil)
}

func (d CharacterDTO) ZenyFormatted() string {
	if d.Zeny < 1000 {
		return fmt.Sprintf("%dz", d.Zeny)
	}
	s := fmt.Sprintf("%d", d.Zeny)
	n := len(s)
	out := make([]byte, 0, n+n/3+1)
	for i := range n {
		if i > 0 && (n-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	out = append(out, 'z')

	return string(out)
}
