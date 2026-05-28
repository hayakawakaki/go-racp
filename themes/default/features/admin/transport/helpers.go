package transport

import (
	"encoding/json"

	adminstate "github.com/hayakawakaki/go-racp/internal/features/admin/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/metric/domain"
)

const (
	dashboardOnlineRefreshMillis  = 60000
	dashboardGeneralRefreshMillis = 3600000
)

type alpineSeed struct {
	General domain.GeneralSnapshot `json:"general"`
	Online  domain.OnlineSnapshot  `json:"online"`
}

func dashboardSeed(state adminstate.DashboardState) string {
	encoded, err := json.Marshal(alpineSeed{Online: state.Online, General: state.General})
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
