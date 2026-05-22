package transport

import (
	"encoding/json"
	"fmt"

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

func dashboardAlpineState(state adminstate.DashboardState) string {
	encoded, err := json.Marshal(alpineSeed{Online: state.Online, General: state.General})
	if err != nil {
		return "{ online: {}, general: {} }"
	}
	const alpineMethods = `{
		_onlineInterval: null,
		_generalInterval: null,
		init() {
			const onlineTick = async () => {
				const o = await fetch('/api/v1/metrics/online').then(r => r.json());
				this.online = o;
			};
			const generalTick = async () => {
				const g = await fetch('/api/v1/metrics/general').then(r => r.json());
				this.general = g;
			};
			this._onlineInterval = setInterval(() => onlineTick().catch(() => {}), %d);
			this._generalInterval = setInterval(() => generalTick().catch(() => {}), %d);
		},
		destroy() {
			if (this._onlineInterval !== null) clearInterval(this._onlineInterval);
			if (this._generalInterval !== null) clearInterval(this._generalInterval);
		},
	}`
	return fmt.Sprintf("Object.assign(%s, %s)", encoded,
		fmt.Sprintf(alpineMethods, dashboardOnlineRefreshMillis, dashboardGeneralRefreshMillis))
}
