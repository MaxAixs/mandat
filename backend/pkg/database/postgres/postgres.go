// Package postgres provides PostgreSQL connection pool construction
// and goose-based schema migrations.
package postgres

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// NewPool creates a connection pool and verifies it with a ping.
func NewPool(ctx context.Context, dbCfg DBConfig) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn(dbCfg))
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// dsn builds the PostgreSQL connection string from the config parameters.
func dsn(dbCfg DBConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		dbCfg.User,
		dbCfg.Password,
		dbCfg.Host,
		dbCfg.Port,
		dbCfg.Database,
		dbCfg.SSLMode,
	)
}

// Migrate applies all pending goose migrations from fsys.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	goose.SetBaseFS(fsys)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
