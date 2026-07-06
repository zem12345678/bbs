package persistence

import (
	"content-service/internal/support/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func OpenDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if cfg.Postgres.Debug {
		db = db.Debug()
	}
	return db, nil
}

func CloseDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
