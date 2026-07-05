// Package postgres owns the pgx connection pool and tenant scoping (Postgres RLS).
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (db *DB) Close() { db.Pool.Close() }

// WithTx runs fn in a single transaction with no tenant scope (used by org provisioning,
// which sets the tenant mid-transaction once the org id exists).
func (db *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetTenant scopes the current transaction to one org via the app.current_org GUC that
// the RLS policies key on. Transaction-local (set_config(...,true)).
func SetTenant(ctx context.Context, tx pgx.Tx, orgID string) error {
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_org', $1, true)", orgID); err != nil {
		return fmt.Errorf("postgres: set tenant: %w", err)
	}
	return nil
}

// WithTenant runs fn in a transaction scoped to one org; RLS filters every row.
func (db *DB) WithTenant(ctx context.Context, orgID string, fn func(tx pgx.Tx) error) error {
	return db.WithTx(ctx, func(tx pgx.Tx) error {
		if err := SetTenant(ctx, tx, orgID); err != nil {
			return err
		}
		return fn(tx)
	})
}
