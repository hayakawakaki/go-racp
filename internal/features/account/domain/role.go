package domain

import "github.com/hayakawakaki/go-racp/server/config"

type Role struct {
	Name    string
	GroupID int
}

var (
	RoleAuthenticated = Role{Name: "*", GroupID: -1}
	RolePublic        = Role{Name: "Public", GroupID: -2}
	RolePlayer        = Role{Name: "Player", GroupID: 0}
	RoleAdmin         = Role{Name: "Admin", GroupID: 99}
)

func (r Role) String() string {
	if r.Name == "" {
		return "unknown"
	}
	return r.Name
}

type RoleResolver struct {
	byGroupID map[int]Role
	byName    map[string]Role
}

func NewRoleResolver(roles config.RolesConfig) RoleResolver {
	byGroupID := make(map[int]Role, len(roles)+1)
	byName := make(map[string]Role, len(roles)+1)
	byGroupID[RoleAdmin.GroupID] = RoleAdmin
	byName[RoleAdmin.Name] = RoleAdmin
	for name, groupID := range roles {
		role := Role{Name: name, GroupID: groupID}
		byGroupID[groupID] = role
		byName[name] = role
	}

	return RoleResolver{byGroupID: byGroupID, byName: byName}
}

func (r RoleResolver) Resolve(groupID int) Role {
	if role, ok := r.byGroupID[groupID]; ok {
		return role
	}

	return RolePlayer
}

func (r RoleResolver) GetRole(name string) (Role, bool) {
	if name == RoleAuthenticated.Name {
		return RoleAuthenticated, true
	}
	if name == RolePublic.Name {
		return RolePublic, true
	}
	role, ok := r.byName[name]

	return role, ok
}
