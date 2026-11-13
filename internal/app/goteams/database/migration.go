// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"

	_ "goteams/internal/app/goteams/database/migrations"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// RunMigrations executes the database migration files embedded in the binary.
func RunMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect(`postgres`); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, `migrations`, goose.WithAllowMissing()); err != nil {
		return fmt.Errorf("run goose up: %w", err)
	}
	return nil
}
