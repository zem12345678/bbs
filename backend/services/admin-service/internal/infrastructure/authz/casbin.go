package authz

import (
	"context"
	"strings"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

const rbacModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

type Store interface {
	RoleKeysByUsername(ctx context.Context, username string) ([]string, error)
	EnsureBootstrapAdmin(ctx context.Context, username string) error
	CasbinRules(ctx context.Context) ([]po.CasbinRule, error)
}

type Authorizer struct {
	enforcer               *casbin.SyncedEnforcer
	store                  Store
	bootstrapAdminPrefixes []string
}

func NewAuthorizer(ctx context.Context, store Store, bootstrapAdminPrefixes []string) (*Authorizer, error) {
	m, err := model.NewModelFromString(rbacModel)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, err
	}
	rules, err := store.CasbinRules(ctx)
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if err := addRule(enforcer, rule); err != nil {
			return nil, err
		}
	}
	prefixes := normalizeList(bootstrapAdminPrefixes)
	if len(prefixes) == 0 {
		prefixes = []string{"admin"}
	}
	return &Authorizer{enforcer: enforcer, store: store, bootstrapAdminPrefixes: prefixes}, nil
}

func (a *Authorizer) Reload(ctx context.Context) error {
	a.enforcer.ClearPolicy()
	rules, err := a.store.CasbinRules(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if err := addRule(a.enforcer, rule); err != nil {
			return err
		}
	}
	return nil
}

func (a *Authorizer) Authorize(ctx context.Context, actor domain.Actor, action domain.Action) error {
	if a.isBootstrapAdmin(actor.Username) {
		if err := a.store.EnsureBootstrapAdmin(ctx, actor.Username); err != nil {
			return err
		}
	}
	roles, err := a.store.RoleKeysByUsername(ctx, actor.Username)
	if err != nil {
		return err
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}
	resource := domain.ResourceForAction(action)
	for _, role := range roles {
		ok, err := a.enforcer.Enforce(role, resource, string(action))
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return domain.ErrPermissionDenied
}

func (a *Authorizer) isBootstrapAdmin(username string) bool {
	username = normalize(username)
	for _, prefix := range a.bootstrapAdminPrefixes {
		if strings.HasPrefix(username, prefix) {
			return true
		}
	}
	return false
}

func addRule(enforcer *casbin.SyncedEnforcer, rule po.CasbinRule) error {
	switch rule.Ptype {
	case "p":
		_, err := enforcer.AddPolicy(toInterfaces(nonEmpty(rule.V0, rule.V1, rule.V2, rule.V3, rule.V4, rule.V5))...)
		return err
	case "g":
		_, err := enforcer.AddGroupingPolicy(toInterfaces(nonEmpty(rule.V0, rule.V1, rule.V2, rule.V3, rule.V4, rule.V5))...)
		return err
	default:
		return nil
	}
}

func toInterfaces(values []string) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = normalize(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
