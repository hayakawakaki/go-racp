package state

import (
	"fmt"
	"slices"
	"time"

	currency "github.com/hayakawakaki/go-racp/internal/features/account/app/currency"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

type ListState struct {
	Now     time.Time
	Query   string
	BaseURL string
	Page    app.UserPage
}

type RoleOption struct {
	Name    string
	GroupID int
}

//nolint:govet // grouped for readability
type DetailState struct {
	Now             time.Time
	Location        *time.Location
	Detail          app.UserDetail
	AllowedRoles    []RoleOption
	Deposits        currency.DepositPage
	Withdraws       currency.WithdrawHistoryPage
	DepositsFailed  bool
	WithdrawsFailed bool
}

func BuildRoleOptions(allowed map[int]string) []RoleOption {
	out := make([]RoleOption, 0, len(allowed))
	for id, name := range allowed {
		out = append(out, RoleOption{GroupID: id, Name: name})
	}
	slices.SortFunc(out, func(a, b RoleOption) int { return a.GroupID - b.GroupID })

	return out
}

func RoleNameFor(state DetailState, groupID int) string {
	for _, opt := range state.AllowedRoles {
		if opt.GroupID == groupID {
			return opt.Name
		}
	}
	if groupID == accdomain.RoleAdmin.GroupID {
		return "Admin"
	}

	return fmt.Sprintf("group_%d", groupID)
}
