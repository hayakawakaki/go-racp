package app

import (
	domain2 "github.com/hayakawakaki/go-racp/internal/features/users/domain"
	"github.com/hayakawakaki/go-racp/internal/features/users/infra"
)

type ListQuery = infra.ListQuery

type UserPage = infra.UserPage

type UserDetail struct {
	User       *domain2.User
	Characters []domain2.Character
	Recent     []domain2.Action
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
