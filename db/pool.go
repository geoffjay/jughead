// Package db provides the Postgres connection pool and repository layer for
// jughead. The pool connects as the non-superuser jughead_app role so
// Row-Level Security policies apply at runtime.
//
// Tenant isolation is enforced per-request via the RLS context helpers
// (WithUserContext / WithOrgContext), which set the app.current_user_id and
// app.current_organization_id GUCs inside a transaction.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the database connection settings. All fields default from the
// DATABASE_URL environment variable when constructed via NewConfig.
type Config struct {
	DatabaseURL string
	MinConns    int32
	MaxConns    int32
}

const (
	defaultMinConns = 2
	defaultMaxConns = 10
)

// NewConfig builds a Config from the environment. DATABASE_URL is required.
func NewConfig() (Config, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set")
	}
	return Config{
		DatabaseURL: url,
		MinConns:    defaultMinConns,
		MaxConns:    defaultMaxConns,
	}, nil
}

// NewPool creates and validates a pgxpool connected to the configured database.
// The caller is responsible for closing the pool on shutdown.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pcfg.MinConns = cfg.MinConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	slog.Info("Database pool ready", "max_conns", pcfg.MaxConns)
	return pool, nil
}
