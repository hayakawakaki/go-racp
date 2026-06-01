package state

import (
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/apikey/domain"
)

type ListState struct {
	Now          time.Time
	Errors       map[string]string
	RevealedKey  string
	RevealedName string
	FormName     string
	FormTier     string
	Keys         []domain.APIKey
	Tiers        []domain.Tier
}
