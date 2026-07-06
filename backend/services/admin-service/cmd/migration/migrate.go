package migration

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var configFile string

var MigrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Synchronize admin-service database schema and seed RBAC policies",
	Example:      "admin-service migrate -c configs/config.yaml",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		migrator, err := InitializeMigrator(context.Background(), configFile)
		if err != nil {
			return err
		}
		defer migrator.Close()
		if err := migrator.Run(context.Background()); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "admin-service database migration completed")
		return nil
	},
}

func init() {
	MigrateCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run migration with provided configuration file")
}
