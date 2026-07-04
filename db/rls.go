package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rlsKey is an unexported context key for the active tenant/user scope. The
// values stored under this key drive the GUCs set inside WithOrgContext.
type rlsKey struct{}

// Scope carries the identity used to set RLS GUCs for a request.
//
// UserID gates the users and profiles tables (self-scoped policies).
// OrganizationID gates organizations, memberships, and invitations.
// Both may be set together when a request acts within a tenant.
type Scope struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
}

// WithScope returns a copy of ctx carrying the given RLS scope. Handlers pass
// the scoped context to repository methods so a single pool connection is
// configured with the correct tenant before any query runs.
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, rlsKey{}, scope)
}

// ScopeFromContext returns the RLS scope carried by ctx, or the zero Scope.
func ScopeFromContext(ctx context.Context) Scope {
	s, _ := ctx.Value(rlsKey{}).(Scope)
	return s
}

// WithOrgContext acquires a pooled connection, begins a transaction, sets the
// app.current_user_id and app.current_organization_id GUCs (when non-zero) so
// RLS policies scope every subsequent query, and yields a Tx-bound context to
// fn. The transaction is committed on a nil return and rolled back on error.
//
// Use this as the boundary for any tenant-scoped operation:
//
//	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
//	    return orgRepo.List(ctx, tx)
//	})
func WithOrgContext(
	ctx context.Context,
	scope Scope,
	fn func(ctx context.Context, tx pgx.Tx) error,
) error {
	pool, err := poolFromContext(ctx)
	if err != nil {
		return err
	}
	return withScopeTx(ctx, pool, scope, fn)
}

// WithOrgContextTx is the pool-explicit variant of WithOrgContext for code
// paths that hold a direct *pgxpool.Pool reference (e.g. startup wiring before
// a request-scoped pool is attached).
func WithOrgContextTx(
	ctx context.Context,
	pool *pgxpool.Pool,
	scope Scope,
	fn func(ctx context.Context, tx pgx.Tx) error,
) error {
	return withScopeTx(ctx, pool, scope, fn)
}

func withScopeTx(
	ctx context.Context,
	pool *pgxpool.Pool,
	scope Scope,
	fn func(ctx context.Context, tx pgx.Tx) error,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// pgx defers rollback safety: a committed tx rolls back harmlessly.
	defer func() { _ = tx.Rollback(ctx) }()

	if scope.UserID != uuid.Nil {
		// SET LOCAL does not accept parameter placeholders, so the value is
		// interpolated as a string literal. uuid.UUID String() is hex-only,
		// never injectable.
		if _, err := tx.Exec(ctx, "SET LOCAL app.current_user_id = '"+scope.UserID.String()+"'"); err != nil {
			return fmt.Errorf("set app.current_user_id: %w", err)
		}
	}
	if scope.OrganizationID != uuid.Nil {
		if _, err := tx.Exec(ctx, "SET LOCAL app.current_organization_id = '"+scope.OrganizationID.String()+"'"); err != nil {
			return fmt.Errorf("set app.current_organization_id: %w", err)
		}
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// poolContextKey separates the pool attachment from the rls scope key.
type poolContextKey struct{}

// WithPool returns a copy of ctx carrying pool so downstream WithOrgContext
// calls can resolve it without an explicit argument.
func WithPool(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, poolContextKey{}, pool)
}

func poolFromContext(ctx context.Context) (*pgxpool.Pool, error) {
	pool, ok := ctx.Value(poolContextKey{}).(*pgxpool.Pool)
	if !ok || pool == nil {
		return nil, fmt.Errorf("no database pool in context")
	}
	return pool, nil
}
