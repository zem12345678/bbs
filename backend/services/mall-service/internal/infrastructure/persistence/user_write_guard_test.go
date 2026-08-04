package persistence

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestLockActiveMallUserLocksResolvedIdentityBeforeTombstoneCheck(t *testing.T) {
	db := &userWriteGuardQueryer{resolvedUserID: 7}

	if err := lockActiveMallUser(context.Background(), db, -101); err != nil {
		t.Fatalf("lockActiveMallUser() error = %v", err)
	}

	if want := []string{"resolve", "lock", "check"}; !reflect.DeepEqual(db.calls, want) {
		t.Fatalf("calls = %v, want %v", db.calls, want)
	}
	if len(db.lockArgs) != 1 || db.lockArgs[0] != int64(7) {
		t.Fatalf("lock args = %#v, want resolved original user ID 7", db.lockArgs)
	}
	if len(db.checkArgs) != 1 || db.checkArgs[0] != int64(-101) {
		t.Fatalf("check args = %#v, want supplied identity -101", db.checkArgs)
	}
}

func TestLockActiveMallUserRejectsOriginalOrAnonymizedTombstone(t *testing.T) {
	db := &userWriteGuardQueryer{resolvedUserID: 7, erased: true}

	err := lockActiveMallUser(context.Background(), db, -101)
	if !errors.Is(err, domain.ErrAccountErased) {
		t.Fatalf("lockActiveMallUser() error = %v, want ErrAccountErased", err)
	}
	if want := []string{"resolve", "lock", "check"}; !reflect.DeepEqual(db.calls, want) {
		t.Fatalf("calls = %v, want %v", db.calls, want)
	}
}

func TestUserMutationTransactionsAcquireMallUserGuardFirst(t *testing.T) {
	targets := map[string]bool{
		"CreateProductReview":  false,
		"ClaimCoupon":          false,
		"CreateOrder":          false,
		"CreateOrderFromCart":  false,
		"BeginOrderPayment":    false,
		"CompleteOrderPayment": false,
		"FailOrderPayment":     false,
		"CancelOrder":          false,
		"ConfirmOrder":         false,
		"CloseExpiredOrder":    false,
		"SetCartItem":          false,
		"RemoveCartItem":       false,
		"ClearCart":            false,
		"AddProductFavorite":   false,
		"CreateAddress":        false,
		"UpdateAddress":        false,
		"DeleteAddress":        false,
		"SetDefaultAddress":    false,
		"CreateRefundRequest":  false,
		"CancelRefundRequest":  false,
	}

	file, err := parser.ParseFile(token.NewFileSet(), "postgres_repository.go", nil, 0)
	if err != nil {
		t.Fatalf("parse postgres_repository.go: %v", err)
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || function.Body == nil {
			continue
		}
		if _, ok := targets[function.Name.Name]; !ok {
			continue
		}
		targets[function.Name.Name] = true
		if len(function.Body.List) < 4 || !nodeCalls(function.Body.List[3], "lockActiveMallUser") {
			t.Errorf("%s must acquire the mall user guard immediately after Begin/error handling/Rollback defer", function.Name.Name)
		}
	}
	for name, found := range targets {
		if !found {
			t.Errorf("target repository method %s not found", name)
		}
	}
}

func nodeCalls(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(candidate ast.Node) bool {
		call, ok := candidate.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == name {
			found = true
			return false
		}
		return true
	})
	return found
}

type userWriteGuardQueryer struct {
	resolvedUserID int64
	erased         bool
	calls          []string
	lockArgs       []any
	checkArgs      []any
}

func (q *userWriteGuardQueryer) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if !strings.Contains(query, "pg_advisory_xact_lock") {
		return pgconn.CommandTag{}, errors.New("unexpected exec")
	}
	q.calls = append(q.calls, "lock")
	q.lockArgs = append([]any(nil), args...)
	return pgconn.CommandTag{}, nil
}

func (q *userWriteGuardQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (q *userWriteGuardQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	switch {
	case strings.Contains(query, "SELECT COALESCE"):
		q.calls = append(q.calls, "resolve")
		return userWriteGuardRow{scan: func(dest ...any) error {
			*dest[0].(*int64) = q.resolvedUserID
			return nil
		}}
	case strings.Contains(query, "SELECT EXISTS"):
		q.calls = append(q.calls, "check")
		q.checkArgs = append([]any(nil), args...)
		return userWriteGuardRow{scan: func(dest ...any) error {
			*dest[0].(*bool) = q.erased
			return nil
		}}
	default:
		return userWriteGuardRow{scan: func(...any) error { return errors.New("unexpected query row") }}
	}
}

type userWriteGuardRow struct {
	scan func(...any) error
}

func (r userWriteGuardRow) Scan(dest ...any) error {
	return r.scan(dest...)
}
