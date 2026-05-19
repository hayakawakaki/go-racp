package moderation

import (
	accdomain "github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/infra"
)

type ListQuery = infra.ListQuery

type UserPage = infra.UserPage

type UserDetail struct {
	User       *accdomain.User
	Characters []accdomain.Character
	Recent     []accdomain.Action
}

type BanCommand struct {
	Reason       string
	Days         int
	ActorUserID  int
	TargetUserID int
	ActorIsAdmin bool
	Permanent    bool
}

type UnbanCommand struct {
	Reason       string
	ActorUserID  int
	TargetUserID int
	ActorIsAdmin bool
}

type SetRoleCommand struct {
	Reason       string
	ActorUserID  int
	TargetUserID int
	NewGroupID   int
	ActorIsAdmin bool
}
