package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/geoffjay/jughead/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// UserService owns the user aggregate: identity credentials, password lifecycle,
// and profile demographics. It is the only service that should mutate the users
// or profiles tables.
type UserService struct {
	Users    UserRepo
	Profiles ProfileRepo
	// Pool is used for pre-auth lookups (FindByEmail/FindByID) that bypass RLS
	// via SECURITY DEFINER functions and therefore do not run inside
	// WithOrgContext. For scoped operations the pool attached to the request
	// context is used instead.
	Pool *pgxpool.Pool
}

// NewUserService wires a UserService with the concrete db repositories and the
// shared connection pool.
func NewUserService(pool *pgxpool.Pool) *UserService {
	return &UserService{
		Users:    db.NewUserRepository(),
		Profiles: db.NewProfileRepository(),
		Pool:     pool,
	}
}

// Authenticate verifies email/password credentials. This is the login-path
// method: it runs BEFORE the caller has a session, so it uses the SECURITY
// DEFINER find_user_by_email function (no RLS scope needed). The caller is
// responsible for creating a session from the returned user.
func (s *UserService) Authenticate(ctx context.Context, email, password string) (db.User, error) {
	u, err := s.Users.FindByEmail(ctx, s.poolOrCtx(ctx), email)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return db.User{}, ErrInvalidCredentials
		}
		return db.User{}, fmt.Errorf("user service authenticate: %w", err)
	}

	if !u.IsActive {
		return db.User{}, ErrUserInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return db.User{}, ErrInvalidCredentials
	}
	return u, nil
}

// Register creates a new user with a bcrypt-hashed password and an empty
// profile. It does NOT create an organization or membership — use
// OrganizationService.CreateWithOwner for the full signup flow. The scope must
// set UserID to uuid.Nil (the users_self WITH CHECK policy requires
// id = current_user_id, so the insert is performed by the SECURITY DEFINER
// path via a superuser-seeded context, OR — the recommended path — the caller
// sets up the user via an unscoped transaction using the pool directly).
//
// In practice, user creation happens pre-tenancy, so this method opens its own
// transaction without RLS scoping and relies on the SECURITY DEFINER
// find_user_by_id function to validate the result.
func (s *UserService) Register(ctx context.Context, email, password string) (db.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return db.User{}, fmt.Errorf("hash password: %w", err)
	}

	pool := s.poolOrCtx(ctx)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return db.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	u, err := s.Users.Create(ctx, tx, email, string(hash))
	if err != nil {
		// users.email is a CITEXT UNIQUE column. pgx wraps the Postgres
		// unique_violation (23505) as a *pgconn.PgError.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return db.User{}, ErrEmailTaken
		}
		return db.User{}, fmt.Errorf("create user: %w", err)
	}

	// Create an empty profile in the same transaction so the 1:1 invariant
	// holds before the user is ever visible.
	if _, err := s.Profiles.Upsert(ctx, tx, &db.Profile{
		UserID:   u.ID,
		Timezone: "UTC",
	}); err != nil {
		return db.User{}, fmt.Errorf("create profile: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.User{}, fmt.Errorf("commit registration: %w", err)
	}
	return u, nil
}

// GetProfile returns the demographics profile for the authenticated user.
// Requires the UserID scope to be set in the context (set by auth middleware).
func (s *UserService) GetProfile(ctx context.Context, scope db.Scope) (db.Profile, error) {
	var prof db.Profile
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		prof, err = s.Profiles.GetByUserID(ctx, tx, scope.UserID)
		return err
	})
	if err != nil {
		return db.Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return prof, nil
}

// UpdateProfile upserts the demographics profile for the authenticated user.
// Requires the UserID scope.
func (s *UserService) UpdateProfile(ctx context.Context, scope db.Scope, prof *db.Profile) (db.Profile, error) {
	prof.UserID = scope.UserID
	var out db.Profile
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = s.Profiles.Upsert(ctx, tx, prof)
		return err
	})
	if err != nil {
		return db.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return out, nil
}

// ChangePassword verifies the current password, then stores a new bcrypt hash.
// Requires the UserID scope.
func (s *UserService) ChangePassword(ctx context.Context, scope db.Scope, currentPassword, newPassword string) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		u, err := s.Users.GetByID(ctx, tx, scope.UserID)
		if err != nil {
			return fmt.Errorf("load user: %w", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
			return ErrInvalidCredentials
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash new password: %w", err)
		}
		return s.Users.UpdatePasswordHash(ctx, tx, scope.UserID, string(hash))
	})
}

// Deactivate sets is_active = false for the user. The user is not deleted;
// this is a soft suspension. Requires the UserID scope (an admin acting on
// another user would need a different scope — a future admin-scope variant).
func (s *UserService) Deactivate(ctx context.Context, scope db.Scope) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Users.UpdateIsActive(ctx, tx, scope.UserID, false)
	})
}

// Delete permanently removes the user and cascades to memberships + profile.
// Requires the UserID scope (self-deletion). Use with caution.
func (s *UserService) Delete(ctx context.Context, scope db.Scope) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Users.Delete(ctx, tx, scope.UserID)
	})
}

// LoadByID re-hydrates a user from a session-stored user ID. Uses the
// find_user_by_id SECURITY DEFINER function so it works without an RLS scope
// (the caller may not have set app.current_user_id yet — e.g. during middleware
// hydration).
func (s *UserService) LoadByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	u, err := s.Users.FindByID(ctx, s.poolOrCtx(ctx), id)
	if err != nil {
		return db.User{}, fmt.Errorf("load user by id: %w", err)
	}
	return u, nil
}

// poolOrCtx returns the service's pool, or the pool attached to ctx if the
// service was constructed without one (e.g. in tests where the pool is
// attached per-request). The ctx-attached pool takes precedence so request-
// scoped connections are reused.
func (s *UserService) poolOrCtx(ctx context.Context) *pgxpool.Pool {
	if pool, err := db.PoolFromContext(ctx); err == nil {
		return pool
	}
	return s.Pool
}
