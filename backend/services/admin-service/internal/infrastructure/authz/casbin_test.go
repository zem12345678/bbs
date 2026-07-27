package authz

import (
	"context"
	"testing"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"
)

func TestAuthorizerAllowsMallRecoverPayingOrdersAction(t *testing.T) {
	ctx := context.Background()
	store := &authorizerStoreStub{
		rolesByUsername: map[string][]string{
			"mall-operator": {"mall_operator"},
		},
		rules: []po.CasbinRule{
			{Ptype: "p", V0: "mall_operator", V1: domain.ResourceMall, V2: string(domain.ActionRecoverPayingMallOrders)},
		},
	}
	authorizer, err := NewAuthorizer(ctx, store)
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}

	err = authorizer.Authorize(ctx, domain.Actor{ID: 1, Username: "mall-operator"}, domain.ActionRecoverPayingMallOrders)
	if err != nil {
		t.Fatalf("Authorize(ActionRecoverPayingMallOrders) error = %v", err)
	}
}

type authorizerStoreStub struct {
	rolesByUsername map[string][]string
	rules           []po.CasbinRule
}

func (s *authorizerStoreStub) RoleKeysByUsername(_ context.Context, username string) ([]string, error) {
	return s.rolesByUsername[username], nil
}

func (s *authorizerStoreStub) CasbinRules(context.Context) ([]po.CasbinRule, error) {
	return s.rules, nil
}
