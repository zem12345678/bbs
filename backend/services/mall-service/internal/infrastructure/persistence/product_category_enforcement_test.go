package persistence

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "mall-service/internal/domain/mall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestEnsureProductCategoryUsableLocksAndRequiresExistingCategory(t *testing.T) {
	db := &productCategoryQueryer{categoryStatus: string(domain.ProductCategoryStatusActive)}

	err := ensureProductCategoryUsable(context.Background(), db, domain.Product{
		Category: "digital",
		Status:   domain.ProductStatusActive,
	})
	if err != nil {
		t.Fatalf("ensureProductCategoryUsable() error = %v, want nil", err)
	}
	if !strings.Contains(db.query, "mall_product_categories") || !strings.Contains(db.query, "FOR SHARE") {
		t.Fatalf("category query = %q, want locked category lookup", db.query)
	}
	if len(db.args) != 1 || db.args[0] != "digital" {
		t.Fatalf("category query args = %+v, want [digital]", db.args)
	}

	err = ensureProductCategoryUsable(context.Background(), &productCategoryQueryer{categoryErr: pgx.ErrNoRows}, domain.Product{
		Category: "missing",
		Status:   domain.ProductStatusActive,
	})
	if !errors.Is(err, domain.ErrProductCategoryNotFound) {
		t.Fatalf("missing category error = %v, want product category not found", err)
	}
}

func TestEnsureProductCategoryUsableRequiresActiveCategoryForActiveProducts(t *testing.T) {
	err := ensureProductCategoryUsable(context.Background(), &productCategoryQueryer{categoryStatus: string(domain.ProductCategoryStatusDraft)}, domain.Product{
		Category: "draft-benefits",
		Status:   domain.ProductStatusActive,
	})
	if !errors.Is(err, domain.ErrProductCategoryUnavailable) {
		t.Fatalf("active product in draft category error = %v, want category unavailable", err)
	}

	err = ensureProductCategoryUsable(context.Background(), &productCategoryQueryer{categoryStatus: string(domain.ProductCategoryStatusDraft)}, domain.Product{
		Category: "draft-benefits",
		Status:   domain.ProductStatusDraft,
	})
	if err != nil {
		t.Fatalf("draft product in draft category error = %v, want nil", err)
	}
}

func TestEnsureProductCategoryChangeAllowedBlocksSlugRenameWithProducts(t *testing.T) {
	err := ensureProductCategoryChangeAllowed(context.Background(), &productCategoryQueryer{referencedProducts: 1},
		domain.ProductCategory{Slug: "digital", Status: domain.ProductCategoryStatusActive},
		domain.ProductCategory{Slug: "premium", Status: domain.ProductCategoryStatusActive},
	)
	if !errors.Is(err, domain.ErrProductCategorySlugLocked) {
		t.Fatalf("rename referenced category error = %v, want product category slug locked", err)
	}
}

func TestEnsureProductCategoryChangeAllowedBlocksActiveProductsOnDisable(t *testing.T) {
	err := ensureProductCategoryChangeAllowed(context.Background(), &productCategoryQueryer{activeProducts: 2},
		domain.ProductCategory{Slug: "digital", Status: domain.ProductCategoryStatusActive},
		domain.ProductCategory{Slug: "digital", Status: domain.ProductCategoryStatusArchived},
	)
	if !errors.Is(err, domain.ErrProductCategoryLocked) {
		t.Fatalf("disable category error = %v, want product category locked", err)
	}

	db := &productCategoryQueryer{activeProducts: 2}
	err = ensureProductCategoryChangeAllowed(context.Background(), db,
		domain.ProductCategory{Slug: "digital", Status: domain.ProductCategoryStatusActive},
		domain.ProductCategory{Slug: "digital", Status: domain.ProductCategoryStatusActive},
	)
	if err != nil {
		t.Fatalf("keep category active error = %v, want nil", err)
	}
	if db.queryRows != 0 {
		t.Fatalf("active category query rows = %d, want 0", db.queryRows)
	}
}

func TestSchemaSeedsPremiumProductCategories(t *testing.T) {
	joined := strings.Join(schemaStatements, "\n")
	for _, slug := range []string{"'digital'", "'badge'", "'theme'", "'physical'"} {
		if !strings.Contains(joined, slug) {
			t.Fatalf("schemaStatements missing seeded category %s", slug)
		}
	}
}

type productCategoryQueryer struct {
	categoryStatus     string
	categoryErr        error
	referencedProducts int64
	activeProducts     int64
	query              string
	args               []any
	queryRows          int
}

func (q *productCategoryQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (q *productCategoryQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *productCategoryQueryer) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.queryRows++
	q.query = query
	q.args = args
	if strings.Contains(query, "COUNT(*)") {
		if !strings.Contains(query, "AND status") {
			return productCategoryCountRow{value: q.referencedProducts}
		}
		return productCategoryCountRow{value: q.activeProducts}
	}
	if q.categoryErr != nil {
		return productCategoryStatusRow{err: q.categoryErr}
	}
	return productCategoryStatusRow{status: q.categoryStatus}
}

type productCategoryStatusRow struct {
	status string
	err    error
}

func (r productCategoryStatusRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}
	status, ok := dest[0].(*string)
	if !ok {
		return errors.New("expected string scan destination")
	}
	*status = r.status
	return nil
}

type productCategoryCountRow struct {
	value int64
}

func (r productCategoryCountRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}
	count, ok := dest[0].(*int64)
	if !ok {
		return errors.New("expected int64 scan destination")
	}
	*count = r.value
	return nil
}
