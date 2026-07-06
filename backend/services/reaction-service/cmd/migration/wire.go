//go:build wireinject

package migration

import (
	"context"

	"reaction-service/internal/support/config"

	"github.com/google/wire"
)

func InitializeMigrator(ctx context.Context, configPath string) (*Migrator, error) {
	wire.Build(
		config.New,
		provideDB,
		NewMigrator,
	)
	return nil, nil
}
