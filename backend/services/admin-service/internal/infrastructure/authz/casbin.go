package authz

import (
	"context"

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
	CasbinRules(ctx context.Context) ([]po.CasbinRule, error)
}

type Authorizer struct {
	enforcer *casbin.SyncedEnforcer
	store    Store
}

func NewAuthorizer(ctx context.Context, store Store) (*Authorizer, error) {
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
	return &Authorizer{enforcer: enforcer, store: store}, nil
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
