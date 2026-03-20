package security

import (
	"net/http"
	"strings"
)

type Permission string

const (
	PermReadAgents    Permission = "read:agents"
	PermWriteAgents   Permission = "write:agents"
	PermReadMessages  Permission = "read:messages"
	PermWriteMessages Permission = "write:messages"
	PermManageGroups  Permission = "manage:groups"
	PermBroadcast     Permission = "broadcast"
	PermAdmin         Permission = "admin"
)

var rolePermissions = map[string][]Permission{
	"admin": {
		PermReadAgents, PermWriteAgents,
		PermReadMessages, PermWriteMessages,
		PermManageGroups, PermBroadcast, PermAdmin,
	},
	"agent": {
		PermReadAgents, PermWriteAgents,
		PermReadMessages, PermWriteMessages,
		PermManageGroups, PermBroadcast,
	},
	"observer": {
		PermReadAgents, PermReadMessages,
	},
}

type Authorizer struct{}

func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

func (a *Authorizer) RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetRole(r.Context())
			if !HasPermission(role, perm) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (a *Authorizer) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetRole(r.Context())
			for _, required := range roles {
				if strings.EqualFold(role, required) {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		})
	}
}

func HasPermission(role string, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}
