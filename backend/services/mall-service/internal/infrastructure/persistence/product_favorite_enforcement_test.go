package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAddProductFavoriteInsertsOnlyActiveProduct(t *testing.T) {
	now := time.Date(2026, 7, 17, 11, 0, 0, 0, time.UTC)
	db := &productFavoriteQueryer{values: []any{true, true, true}}

	added, err := addProductFavorite(context.Background(), db, 42, 101, now)

	if err != nil {
		t.Fatalf("addProductFavorite() error = %v", err)
	}
	if !added {
		t.Fatal("addProductFavorite() = false, want true")
	}
	for _, want := range []string{
		"mall_products",
		"WHERE id = $2",
		"WHERE status = $4",
		"INSERT INTO mall_product_favorites",
		"ON CONFLICT (user_id, product_id) DO NOTHING",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("add favorite query = %q, want %q", db.query, want)
		}
	}
	wantArgs := []any{int64(42), int64(101), now, string(domain.ProductStatusActive)}
	if len(db.args) != len(wantArgs) {
		t.Fatalf("add favorite args = %+v, want %+v", db.args, wantArgs)
	}
	for i := range wantArgs {
		if db.args[i] != wantArgs[i] {
			t.Fatalf("add favorite arg %d = %#v, want %#v", i, db.args[i], wantArgs[i])
		}
	}
}

func TestAddProductFavoriteReturnsFalseForExistingActiveFavorite(t *testing.T) {
	db := &productFavoriteQueryer{values: []any{true, true, false}}

	added, err := addProductFavorite(context.Background(), db, 42, 101, time.Now())

	if err != nil {
		t.Fatalf("addProductFavorite() error = %v", err)
	}
	if added {
		t.Fatal("addProductFavorite() = true, want false for duplicate favorite")
	}
}

func TestAddProductFavoriteRejectsUnavailableProduct(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		want   error
	}{
		{name: "missing product", values: []any{false, false, false}, want: domain.ErrProductNotFound},
		{name: "inactive product", values: []any{true, false, false}, want: domain.ErrProductUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &productFavoriteQueryer{values: tt.values}

			added, err := addProductFavorite(context.Background(), db, 42, 101, time.Now())

			if !errors.Is(err, tt.want) {
				t.Fatalf("addProductFavorite() error = %v, want %v", err, tt.want)
			}
			if added {
				t.Fatal("addProductFavorite() = true, want false")
			}
		})
	}
}

type productFavoriteQueryer struct {
	values []any
	err    error
	query  string
	args   []any
}

func (q *productFavoriteQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *productFavoriteQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *productFavoriteQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.query = query
	q.args = args
	return productFavoriteScanRow{values: q.values, err: q.err}
}

type productFavoriteScanRow struct {
	values []any
	err    error
}

func (r productFavoriteScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return testScanner(r.values).Scan(dest...)
}
