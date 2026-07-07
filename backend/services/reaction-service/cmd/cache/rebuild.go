package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	configFile string
	verify     bool
)

var RebuildCmd = &cobra.Command{
	Use:          "rebuild-cache",
	Short:        "Rebuild reaction Redis cache from PostgreSQL",
	Example:      "reaction-service rebuild-cache -c configs/config.yaml --verify",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		rebuilder, err := CreateRebuilder(configFile)
		if err != nil {
			return err
		}
		defer rebuilder.Close()
		stats, err := rebuilder.Rebuild(ctx)
		if err != nil {
			return err
		}
		if verify {
			if err := rebuilder.Verify(ctx); err != nil {
				return err
			}
		}
		out := struct {
			DeletedKeys     int64 `json:"deleted_keys"`
			LikesLoaded     int64 `json:"likes_loaded"`
			FavoritesLoaded int64 `json:"favorites_loaded"`
			HotEntries      int64 `json:"hot_entries"`
			Verified        bool  `json:"verified"`
		}{
			DeletedKeys:     stats.DeletedKeys,
			LikesLoaded:     stats.LikesLoaded,
			FavoritesLoaded: stats.FavoritesLoaded,
			HotEntries:      stats.HotEntries,
			Verified:        verify,
		}
		b, err := json.Marshal(out)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return nil
	},
}

func init() {
	RebuildCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "Run cache rebuild with provided configuration file")
	RebuildCmd.Flags().BoolVar(&verify, "verify", false, "Verify Redis reaction cache against PostgreSQL after rebuild")
}
