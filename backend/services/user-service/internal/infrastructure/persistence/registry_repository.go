package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "user-service/internal/domain/user"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type registryItemPO struct {
	ID        int64     `gorm:"primaryKey"`
	UserID    int64     `gorm:"not null;index"`
	Domain    *string   `gorm:"column:domain;size:512"`
	Scope     []byte    `gorm:"type:jsonb;not null"`
	Key       string    `gorm:"column:key;size:1024;not null"`
	Value     []byte    `gorm:"type:jsonb;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (registryItemPO) TableName() string { return "registry_items" }

var _ domain.RegistryRepository = (*Repo)(nil)

// SetRegistryItem serializes per-user quota checks with the write so concurrent
// domains cannot bypass account-wide key and storage limits.
func (r *Repo) SetRegistryItem(ctx context.Context, item *domain.RegistryItem) error {
	if err := item.Validate(); err != nil {
		return err
	}
	scope, err := marshalRegistryScope(item.Scope)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRegistryUser(tx, item.UserID); err != nil {
			return err
		}
		if err := ensureUserExists(tx, item.UserID); err != nil {
			return err
		}
		var existing registryItemPO
		err := registryItemScopeQuery(tx, item.UserID, item.Domain, scope).
			Where("key = ?", item.Key).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Take(&existing).Error
		exists := err == nil
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := ensureRegistryCapacity(tx, item, scope, &existing, exists); err != nil {
			return err
		}
		if exists {
			if err := tx.Model(&registryItemPO{}).
				Where("id = ?", existing.ID).
				Updates(map[string]any{"value": item.Value, "updated_at": item.UpdatedAt}).Error; err != nil {
				return err
			}
			item.ID = existing.ID
			item.CreatedAt = existing.CreatedAt
			return nil
		}
		return tx.Create(&registryItemPO{
			ID: item.ID, UserID: item.UserID, Domain: copyRegistryDomain(item.Domain),
			Scope: scope, Key: item.Key, Value: item.Value,
			CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		}).Error
	})
}

func ensureRegistryCapacity(tx *gorm.DB, item *domain.RegistryItem, scope []byte, existing *registryItemPO, exists bool) error {
	if !exists {
		var domainCount int64
		if err := registryItemDomainQuery(tx, item.UserID, item.Domain).Count(&domainCount).Error; err != nil {
			return err
		}
		var userCount int64
		if err := tx.Model(&registryItemPO{}).Where("user_id = ?", item.UserID).Count(&userCount).Error; err != nil {
			return err
		}
		if domainCount >= domain.MaxRegistryKeysPerDomain || userCount >= domain.MaxRegistryKeysPerUser {
			return domain.ErrRegistryKeyLimitReached
		}
	}

	newValueBytes, err := registryJSONBytes(tx, item.Value)
	if err != nil {
		return err
	}
	oldValueBytes := int64(0)
	if exists {
		oldValueBytes = int64(len(existing.Value))
	}
	scopeValueBytes, err := registryStoredValueBytes(registryItemScopeQuery(tx, item.UserID, item.Domain, scope))
	if err != nil {
		return err
	}
	userValueBytes, err := registryStoredValueBytes(tx.Model(&registryItemPO{}).Where("user_id = ?", item.UserID))
	if err != nil {
		return err
	}
	if scopeValueBytes-oldValueBytes+newValueBytes > domain.MaxRegistryScopeValueBytes ||
		userValueBytes-oldValueBytes+newValueBytes > domain.MaxRegistryUserValueBytes {
		return domain.ErrRegistryValueTooLarge
	}
	return nil
}

func registryJSONBytes(tx *gorm.DB, value []byte) (int64, error) {
	var size int64
	err := tx.Raw("SELECT octet_length(?::jsonb::text)", string(value)).Scan(&size).Error
	return size, err
}

func registryStoredValueBytes(query *gorm.DB) (int64, error) {
	var size int64
	err := query.Select("COALESCE(SUM(octet_length(value::text)), 0)").Scan(&size).Error
	return size, err
}

func (r *Repo) GetRegistryItem(ctx context.Context, userID int64, itemDomain *string, scope []string, key string) (*domain.RegistryItem, error) {
	encodedScope, validatedDomain, validatedKey, err := validateRegistryLookup(itemDomain, scope, key)
	if err != nil {
		return nil, err
	}
	var row registryItemPO
	err = registryItemScopeQuery(r.db.WithContext(ctx), userID, validatedDomain, encodedScope).
		Where("key = ?", validatedKey).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRegistryItemNotFound
	}
	if err != nil {
		return nil, err
	}
	return toRegistryDomain(row)
}

// ListRegistryItems returns every key inside one exact scope path, oldest first
// so callers rebuild deterministic maps.
func (r *Repo) ListRegistryItems(ctx context.Context, userID int64, itemDomain *string, scope []string) ([]*domain.RegistryItem, error) {
	encodedScope, validatedDomain, err := validateRegistryScope(itemDomain, scope)
	if err != nil {
		return nil, err
	}
	var rows []registryItemPO
	if err := registryItemScopeQuery(r.db.WithContext(ctx), userID, validatedDomain, encodedScope).
		Order("updated_at ASC").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*domain.RegistryItem, 0, len(rows))
	for _, row := range rows {
		item, err := toRegistryDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *Repo) RemoveRegistryItem(ctx context.Context, userID int64, itemDomain *string, scope []string, key string) error {
	encodedScope, validatedDomain, validatedKey, err := validateRegistryLookup(itemDomain, scope, key)
	if err != nil {
		return err
	}
	result := registryItemScopeQuery(r.db.WithContext(ctx), userID, validatedDomain, encodedScope).
		Where("key = ?", validatedKey).Delete(&registryItemPO{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrRegistryItemNotFound
	}
	return nil
}

// ListRegistryScopeDomains groups distinct scope paths under each domain so
// clients can discover what the account already stores.
func (r *Repo) ListRegistryScopeDomains(ctx context.Context, userID int64) ([]domain.RegistryScopeDomain, error) {
	if userID <= 0 {
		return nil, domain.ErrInvalidID
	}
	var rows []registryItemPO
	if err := r.db.WithContext(ctx).Model(&registryItemPO{}).
		Select("domain", "scope").
		Where("user_id = ?", userID).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	groups := make([]domain.RegistryScopeDomain, 0, len(rows))
	indexes := make(map[string]int, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		scope, err := unmarshalRegistryScope(row.Scope)
		if err != nil {
			return nil, err
		}
		domainKey := registryDomainKey(row.Domain)
		dedupe := domainKey + "\x1f" + domain.RegistryScopeKey(scope)
		if _, exists := seen[dedupe]; exists {
			continue
		}
		seen[dedupe] = struct{}{}
		index, exists := indexes[domainKey]
		if !exists {
			groups = append(groups, domain.RegistryScopeDomain{Domain: copyRegistryDomain(row.Domain), Scopes: [][]string{}})
			index = len(groups) - 1
			indexes[domainKey] = index
		}
		groups[index].Scopes = append(groups[index].Scopes, scope)
	}
	return groups, nil
}

func validateRegistryScope(itemDomain *string, scope []string) ([]byte, *string, error) {
	validatedDomain, err := domain.NormalizeRegistryDomain(itemDomain)
	if err != nil {
		return nil, nil, err
	}
	validatedScope, err := domain.NormalizeRegistryScope(scope)
	if err != nil {
		return nil, nil, err
	}
	encodedScope, err := marshalRegistryScope(validatedScope)
	if err != nil {
		return nil, nil, err
	}
	return encodedScope, validatedDomain, nil
}

func validateRegistryLookup(itemDomain *string, scope []string, key string) ([]byte, *string, string, error) {
	encodedScope, validatedDomain, err := validateRegistryScope(itemDomain, scope)
	if err != nil {
		return nil, nil, "", err
	}
	validatedKey := key
	if key != "" {
		validatedKey, err = domain.NormalizeRegistryKey(key)
		if err != nil {
			return nil, nil, "", err
		}
	}
	return encodedScope, validatedDomain, validatedKey, nil
}

// registryItemScopeQuery matches the exact scope document. Comparing the JSONB
// value keeps ["a","b"] distinct from ["b","a"], which the scope path requires.
func registryItemScopeQuery(tx *gorm.DB, userID int64, itemDomain *string, scope []byte) *gorm.DB {
	return registryItemDomainQuery(tx, userID, itemDomain).Where("scope = ?::jsonb", string(scope))
}

func registryItemDomainQuery(tx *gorm.DB, userID int64, itemDomain *string) *gorm.DB {
	query := tx.Model(&registryItemPO{}).Where("user_id = ?", userID)
	if itemDomain == nil {
		return query.Where("domain IS NULL")
	}
	return query.Where("domain = ?", *itemDomain)
}

func copyRegistryDomain(itemDomain *string) *string {
	if itemDomain == nil {
		return nil
	}
	value := *itemDomain
	return &value
}

func registryDomainKey(itemDomain *string) string {
	if itemDomain == nil {
		return "null"
	}
	return "value:" + *itemDomain
}

func marshalRegistryScope(scope []string) ([]byte, error) {
	if scope == nil {
		scope = []string{}
	}
	return json.Marshal(scope)
}

func unmarshalRegistryScope(raw []byte) ([]string, error) {
	var scope []string
	if err := json.Unmarshal(raw, &scope); err != nil {
		return nil, err
	}
	if scope == nil {
		scope = []string{}
	}
	return scope, nil
}

func lockRegistryUser(tx *gorm.DB, userID int64) error {
	return tx.Exec(
		"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
		fmt.Sprintf("registry-user:%d", userID),
	).Error
}

func toRegistryDomain(row registryItemPO) (*domain.RegistryItem, error) {
	scope, err := unmarshalRegistryScope(row.Scope)
	if err != nil {
		return nil, err
	}
	return &domain.RegistryItem{
		ID: row.ID, UserID: row.UserID, Domain: copyRegistryDomain(row.Domain), Scope: scope,
		Key: row.Key, Value: row.Value, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}
