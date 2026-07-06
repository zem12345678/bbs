//go:build wireinject

package cache

import (
	"context"

	"reaction-service/internal/support/config"

	"github.com/google/wire"
)

func InitializeRebuilder(ctx context.Context, configPath string) (*Rebuilder, error) {
	wire.Build(
		config.New,
		provideDB,
		provideRedisClient,
		NewRebuilder,
	)
	return nil, nil
}
