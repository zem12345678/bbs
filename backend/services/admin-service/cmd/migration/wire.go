//go:build wireinject

package migration

import (
	"context"

	"admin/internal/infrastructure/persistence"
	"admin/internal/support/config"

	"github.com/google/wire"
)

func InitializeMigrator(ctx context.Context, configPath string) (*Migrator, error) {
	wire.Build(
		config.New,
		persistence.NewDB,
		persistence.NewRepositoryFromDB,
		NewMigrator,
	)
	return nil, nil
}
