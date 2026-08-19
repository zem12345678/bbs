package grpc

import (
	"context"
	"testing"
	"time"

	pb "user-service/api/proto/userpb"
	"user-service/internal/application/user/command"
	"user-service/internal/application/user/query"
	domain "user-service/internal/domain/user"

	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

type registryHandlerRepo struct {
	domain.Repository
	stored       *domain.RegistryItem
	removedKey   string
	removedScope []string
	removeErr    error
	groups       []domain.RegistryScopeDomain
}

func (r *registryHandlerRepo) SetRegistryItem(_ context.Context, item *domain.RegistryItem) error {
	copyItem := *item
	copyItem.Domain = copyRegistryTestDomain(item.Domain)
	copyItem.Scope = append([]string(nil), item.Scope...)
	copyItem.Value = append([]byte(nil), item.Value...)
	r.stored = &copyItem
	return nil
}

func (r *registryHandlerRepo) GetRegistryItem(context.Context, int64, *string, []string, string) (*domain.RegistryItem, error) {
	if r.stored == nil {
		return nil, domain.ErrRegistryItemNotFound
	}
	return r.stored, nil
}

func (r *registryHandlerRepo) ListRegistryItems(context.Context, int64, *string, []string) ([]*domain.RegistryItem, error) {
	if r.stored == nil {
		return []*domain.RegistryItem{}, nil
	}
	return []*domain.RegistryItem{r.stored}, nil
}

func (r *registryHandlerRepo) RemoveRegistryItem(_ context.Context, _ int64, _ *string, scope []string, key string) error {
	r.removedKey = key
	r.removedScope = append([]string(nil), scope...)
	return r.removeErr
}

func (r *registryHandlerRepo) ListRegistryScopeDomains(context.Context, int64) ([]domain.RegistryScopeDomain, error) {
	return r.groups, nil
}

func TestRegistryHandlersPreserveDomainPresenceAndRawJSON(t *testing.T) {
	emptyDomain := ""
	repo := &registryHandlerRepo{groups: []domain.RegistryScopeDomain{
		{Domain: nil, Scopes: [][]string{{}, {"native"}}},
		{Domain: &emptyDomain, Scopes: [][]string{{"empty"}}},
	}}
	handler := NewHandler(
		command.NewService(repo, &handlerIDGen{next: 9007199254740992}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil),
		query.NewService(repo, nil),
	)
	ctx := context.Background()

	set, err := handler.SetRegistryItem(ctx, &pb.SetRegistryItemRequest{
		UserId: 42, Domain: &pb.RegistryDomain{Value: ""}, Scope: []string{"client", "preferences"},
		Key: " large ", ValueJson: []byte(`{"id":9223372036854775807}`),
	})
	if err != nil {
		t.Fatalf("SetRegistryItem() error = %v", err)
	}
	item := set.GetItem()
	if item.GetId() != "9007199254740993" || item.GetUserId() != 42 || item.GetKey() != " large " {
		t.Fatalf("set response item = %+v", item)
	}
	if item.GetDomain() == nil || item.GetDomain().GetValue() != "" {
		t.Fatalf("set response domain = %+v, want present empty string", item.GetDomain())
	}
	if string(item.GetValueJson()) != `{"id":9223372036854775807}` {
		t.Fatalf("set response value = %s", item.GetValueJson())
	}

	listed, err := handler.ListRegistryItems(ctx, &pb.ListRegistryItemsRequest{UserId: 42, Scope: []string{"client", "preferences"}})
	if err != nil || len(listed.GetItems()) != 1 || listed.GetItems()[0].GetId() != item.GetId() {
		t.Fatalf("ListRegistryItems() response = %+v, error = %v", listed, err)
	}

	if _, err := handler.RemoveRegistryItem(ctx, &pb.GetRegistryItemRequest{UserId: 42, Scope: []string{}, Key: ""}); err != nil {
		t.Fatalf("RemoveRegistryItem() error = %v", err)
	}
	if repo.removedKey != "" || len(repo.removedScope) != 0 {
		t.Fatalf("remove address key=%q scope=%v", repo.removedKey, repo.removedScope)
	}

	scopeDomains, err := handler.ListRegistryScopeDomains(ctx, &pb.UserIDRequest{Id: 42})
	if err != nil {
		t.Fatalf("ListRegistryScopeDomains() error = %v", err)
	}
	if len(scopeDomains.GetItems()) != 2 || scopeDomains.GetItems()[0].GetDomain() != nil {
		t.Fatalf("scope/domain response = %+v", scopeDomains.GetItems())
	}
	if got := scopeDomains.GetItems()[1].GetDomain(); got == nil || got.GetValue() != "" {
		t.Fatalf("empty domain response = %+v", got)
	}
}

func TestRemoveRegistryItemMapsMissingItemToNotFound(t *testing.T) {
	repo := &registryHandlerRepo{removeErr: domain.ErrRegistryItemNotFound}
	handler := NewHandler(
		command.NewService(repo, &handlerIDGen{}, nil, nil, "test-secret", time.Hour, 8, nil, nil, nil),
		query.NewService(repo, nil),
	)

	_, err := handler.RemoveRegistryItem(context.Background(), &pb.GetRegistryItemRequest{
		UserId: 42,
		Scope:  []string{"client", "preferences"},
		Key:    "missing",
	})
	if got := grpcstatus.Code(err); got != codes.NotFound {
		t.Fatalf("RemoveRegistryItem() code = %v, want %v (error = %v)", got, codes.NotFound, err)
	}
}

func TestRegistryStatusMappings(t *testing.T) {
	tests := []struct {
		err  error
		code codes.Code
	}{
		{domain.ErrRegistryItemNotFound, codes.NotFound},
		{domain.ErrRegistryKeyLimitReached, codes.FailedPrecondition},
		{domain.ErrRegistryValueRequired, codes.InvalidArgument},
		{domain.ErrRegistryValueTooLarge, codes.InvalidArgument},
		{domain.ErrRegistryRepositoryUnavailable, codes.Unavailable},
	}
	for _, test := range tests {
		if got := grpcstatus.Code(toStatus(test.err)); got != test.code {
			t.Fatalf("toStatus(%v) = %v, want %v", test.err, got, test.code)
		}
	}
}

func copyRegistryTestDomain(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
