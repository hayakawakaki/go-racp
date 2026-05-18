package app

import (
	"github.com/hayakawakaki/go-racp/internal/users/domain"
	"github.com/hayakawakaki/go-racp/internal/users/infra"
)

type ListQuery = infra.ListQuery

type UserPage = infra.UserPage

type UserDetail struct {
	User       *domain.User
	Characters []domain.Character
	Recent     []domain.Action
}

type BanCommand struct {
	Reason       string
	Days         int
	ActorUserID  int
	TargetUserID int
	Permanent    bool
}

type UnbanCommand struct {
	Reason       string
	ActorUserID  int
	TargetUserID int
}

type SetRoleCommand struct {
	Reason       string
	ActorUserID  int
	TargetUserID int
	NewGroupID   int
}
