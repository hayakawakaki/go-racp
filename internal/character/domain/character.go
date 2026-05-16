package domain

import "context"

type Character struct {
	Name          string
	Gender        string
	CurrentMap    string
	SaveMap       string
	ID            int
	AccountID     int
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

type Repository interface {
	ListByAccount(ctx context.Context, accountID int) ([]Character, error)
	GetByID(ctx context.Context, charID int) (*Character, error)
	UpdateLook(ctx context.Context, charID, hair, hairColor, clothesColor int) error
	UpdateLocation(ctx context.Context, charID int, mapName string, x, y int) error
}
