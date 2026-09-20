package db

import (
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/portico/backend/internal/config"
	"github.com/portico/backend/internal/models"
	"github.com/portico/backend/internal/secretbox"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var dbNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func Connect(driver, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", driver)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := gdb.AutoMigrate(&models.Connection{}, &models.SyncJob{}, &models.SyncJobRelation{}, &models.SyncJobField{}, &models.SyncLog{}); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	if err := sealPlaintextConnectionConfigs(gdb); err != nil {
		return nil, fmt.Errorf("seal connection configs: %w", err)
	}

	return gdb, nil
}

// sealPlaintextConnectionConfigs encrypts secret fields on rows written before encryption existed.
func sealPlaintextConnectionConfigs(gdb *gorm.DB) error {
	var items []models.Connection
	if err := gdb.Session(&gorm.Session{SkipHooks: true}).Find(&items).Error; err != nil {
		return err
	}
	for i := range items {
		if !secretbox.NeedsSeal(items[i].Config) {
			continue
		}
		if err := gdb.Save(&items[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// Open ensures the configured Postgres database exists, then connects and auto-migrates.
func Open(cfg *config.Config) (*gorm.DB, error) {
	if cfg.DBDriver == "postgres" {
		if err := ensurePostgresDatabase(cfg); err != nil {
			return nil, fmt.Errorf("ensure database: %w", err)
		}
	}
	return Connect(cfg.DBDriver, cfg.DSN())
}

func ensurePostgresDatabase(cfg *config.Config) error {
	if !dbNamePattern.MatchString(cfg.DBName) {
		return fmt.Errorf("invalid database name %q", cfg.DBName)
	}

	// Already present — nothing to create.
	if err := pingPostgres(cfg, cfg.DBName); err == nil {
		return nil
	}

	// Create DB_NAME via template1 (always exists; do not use dbname=postgres).
	admin, err := openPostgres(cfg, "template1")
	if err != nil {
		return fmt.Errorf("open admin database: %w", err)
	}
	sqlDB, err := admin.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	var exists bool
	if err := admin.Raw("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)", cfg.DBName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check database: %w", err)
	}
	if exists {
		return nil
	}

	ident := pgx.Identifier{cfg.DBName}.Sanitize()
	if err := admin.Exec("CREATE DATABASE " + ident).Error; err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return nil
}

func pingPostgres(cfg *config.Config, dbName string) error {
	gdb, err := openPostgres(cfg, dbName)
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return sqlDB.Ping()
}

func openPostgres(cfg *config.Config, dbName string) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, dbName, cfg.DBPort, cfg.DBSSLMode,
	)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}
