//go:build wireinject

package migration

import (
	"context"

	"content-service/internal/infrastructure/persistence"
	"content-service/internal/support/config"

	"github.com/google/wire"
)

func InitializeMigrator(ctx context.Context, configPath string) (*Migrator, error) {
	wire.Build(
		config.New,
		persistence.OpenDB,
		NewMigrator,
	)
	return nil, nil
}
