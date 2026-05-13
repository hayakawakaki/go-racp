package domain

import "github.com/hayakawakaki/go-racp/server/config"

type Role int

const (
	RoleAuthenticated Role = -1
	RolePlayer        Role = 0
	RoleEvent         Role = 1
	RoleModerator     Role = 2
	RoleEnforcer      Role = 3
	RoleAdmin         Role = 4
)

func (r Role) String() string {
	switch r {
	case RoleAuthenticated:
		return "authenticated"
	case RolePlayer:
		return "player"
	case RoleEvent:
		return "event"
	case RoleModerator:
		return "moderator"
	case RoleEnforcer:
		return "enforcer"
	case RoleAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

func (r Role) AtLeast(minimum Role) bool {
	return r >= minimum
}

type RoleResolver struct {
	moderator int
	enforcer  int
	event     int
}

func NewRoleResolver(cfg config.GroupConfig) RoleResolver {
	return RoleResolver{
		moderator: cfg.Moderator,
		enforcer:  cfg.Enforcer,
		event:     cfg.Event,
	}
}

func (r RoleResolver) Resolve(groupID int) Role {
	switch groupID {
	case 99:
		return RoleAdmin
	case r.enforcer:
		return RoleEnforcer
	case r.moderator:
		return RoleModerator
	case r.event:
		return RoleEvent
	default:
		return RolePlayer
	}
}
