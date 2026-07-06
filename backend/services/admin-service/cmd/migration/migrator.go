package migration

import (
	"context"

	"admin/internal/infrastructure/persistence"
	"admin/internal/support/config"
)

type Migrator struct {
	cfg  *config.Config
	db   *persistence.DB
	repo *persistence.Repository
}

func NewMigrator(cfg *config.Config, db *persistence.DB, repo *persistence.Repository) *Migrator {
	return &Migrator{cfg: cfg, db: db, repo: repo}
}

func (m *Migrator) Run(ctx context.Context) error {
	if err := m.repo.EnsureSchema(ctx); err != nil {
		return err
	}
	return m.repo.SeedDefaults(ctx, m.cfg.RBAC.BootstrapAdminPrefixes, m.cfg.Auth.DefaultAdminPassword)
}

func (m *Migrator) Close() {
	if m.db != nil {
		_ = m.db.Close()
	}
}
