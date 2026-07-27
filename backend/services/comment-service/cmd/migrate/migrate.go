package migrate

import (
	"context"
	"fmt"

	"comment-service/internal/infrastructure/persistence"
	"comment-service/internal/ioc/config"
	iocmongo "comment-service/internal/ioc/db/mongo"
	ioclogger "comment-service/internal/ioc/logger"

	"github.com/spf13/cobra"
)

var configFile string

// MigrateCmd creates the MongoDB indexes required by comment queries. Keeping
// it separate from `server` lets deployers control the potentially expensive
// index build and prevents multiple replicas from attempting it concurrently.
var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Create or update comment-service MongoDB indexes",
	Example:      "comment-service migrate -c configs/config.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd.Context())
	},
}

func init() {
	MigrateCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run migration with provided configuration file")
}

func run(ctx context.Context) error {
	v, err := config.New(configFile)
	if err != nil {
		return err
	}
	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		return err
	}
	log, err := ioclogger.New(logOptions)
	if err != nil {
		return err
	}
	mongoOptions, err := iocmongo.NewOptions(v, log)
	if err != nil {
		return err
	}
	mongodb, err := iocmongo.New(mongoOptions)
	if err != nil {
		return err
	}
	defer func() { _ = mongodb.Client.Disconnect(ctx) }()

	if err := persistence.NewRepository(mongodb.DB).EnsureIndexes(ctx); err != nil {
		return fmt.Errorf("migrate comment-service indexes: %w", err)
	}
	return nil
}
