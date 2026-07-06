package persistence

import (
	"admin/internal/support/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	db *gorm.DB
}

func NewDB(cfg *config.Config) (*DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if cfg.Postgres.Debug {
		db = db.Debug()
	}
	return &DB{db: db}, nil
}

func (d *DB) Gorm() *gorm.DB {
	return d.db
}

func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
