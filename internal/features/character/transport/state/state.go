package state

import (
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/character/app"
)

type DetailState struct {
	Char   *app.CharacterDTO
	Now    time.Time
	Notice string
}
