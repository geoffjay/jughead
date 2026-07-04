package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned by repository methods when no row matches the query.
var ErrNotFound = errors.New("record not found")

// querier is the minimal subset of pgxpool.Pool / pgx.Tx needed for the
// pre-auth lookups (the login path runs via SECURITY DEFINER functions so it
// does not need an RLS-gated transaction).
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// UserRepository provides access to the users table.
//
// Tenant-scoped methods (GetByID, Create, Update*, Delete) must run inside a
// WithOrgContext transaction with the UserID RLS scope set; the users_self
// policy restricts them to the current user.
//
// The login-path methods (FindByEmail, FindByID) bypass RLS via SECURITY
// DEFINER functions (see db/migrations/002_auth_functions.sql) because the
// caller has not yet authenticated and cannot set app.current_user_id.
type UserRepository struct{}

// NewUserRepository returns a UserRepository.
func NewUserRepository() *UserRepository { return &UserRepository{} }

// FindByEmail fetches a user by email for the login flow. Uses the
// find_user_by_email SECURITY DEFINER function so it is not blocked by RLS.
func (UserRepository) FindByEmail(ctx context.Context, q querier, email string) (User, error) {
	const stmt = `SELECT id, email, password_hash, is_active, created_at, updated_at FROM find_user_by_email($1)`
	var u User
	err := q.QueryRow(ctx, stmt, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

// FindByID fetches a user by id. Uses the find_user_by_id SECURITY DEFINER
// function so it works pre-authentication (e.g. re-hydrating an identity from a
// session-stored user id).
func (UserRepository) FindByID(ctx context.Context, q querier, id uuid.UUID) (User, error) {
	const stmt = `SELECT id, email, password_hash, is_active, created_at, updated_at FROM find_user_by_id($1)`
	var u User
	err := q.QueryRow(ctx, stmt, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

// GetByID fetches a single user by primary key inside an RLS-gated
// transaction. Requires the UserID scope to be set.
func (UserRepository) GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (User, error) {
	const q = `
		SELECT id, email, password_hash, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`
	var u User
	err := tx.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// Create inserts a new user. passwordHash must already be bcrypt-hashed by the
// caller. Runs inside the caller's RLS-gated transaction.
func (UserRepository) Create(ctx context.Context, tx pgx.Tx, email, passwordHash string) (User, error) {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, is_active, created_at, updated_at
	`
	var u User
	err := tx.QueryRow(ctx, q, email, passwordHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// UpdateIsActive flips the global suspension flag for a user.
func (UserRepository) UpdateIsActive(ctx context.Context, tx pgx.Tx, id uuid.UUID, active bool) error {
	const q = `UPDATE users SET is_active = $2 WHERE id = $1`
	ct, err := tx.Exec(ctx, q, id, active)
	if err != nil {
		return fmt.Errorf("update user active: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePasswordHash replaces the stored password hash.
func (UserRepository) UpdatePasswordHash(ctx context.Context, tx pgx.Tx, id uuid.UUID, hash string) error {
	const q = `UPDATE users SET password_hash = $2 WHERE id = $1`
	ct, err := tx.Exec(ctx, q, id, hash)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a user permanently (cascades to memberships + profile).
func (UserRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	const q = `DELETE FROM users WHERE id = $1`
	ct, err := tx.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
