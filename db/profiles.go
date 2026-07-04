package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
)

// ProfileRepository provides access to the profiles table (1:1 with users).
//
// The profiles_self policy restricts rows to the current user, so callers must
// set the UserID scope via WithOrgContext.
type ProfileRepository struct{}

// NewProfileRepository returns a ProfileRepository.
func NewProfileRepository() *ProfileRepository { return &ProfileRepository{} }

// GetByUserID fetches the profile for a user.
func (ProfileRepository) GetByUserID(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (Profile, error) {
	const q = `
		SELECT user_id, first_name, last_name, avatar_url, timezone, created_at, updated_at
		FROM profiles WHERE user_id = $1
	`
	var p Profile
	err := tx.QueryRow(ctx, q, userID).Scan(
		&p.UserID, &p.FirstName, &p.LastName, &p.AvatarURL, &p.Timezone, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

// Upsert inserts a new profile or updates the existing one for the user.
func (ProfileRepository) Upsert(ctx context.Context, tx pgx.Tx, p *Profile) (Profile, error) {
	const q = `
		INSERT INTO profiles (user_id, first_name, last_name, avatar_url, timezone)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
			SET first_name = EXCLUDED.first_name,
			    last_name  = EXCLUDED.last_name,
			    avatar_url = EXCLUDED.avatar_url,
			    timezone   = EXCLUDED.timezone
		RETURNING user_id, first_name, last_name, avatar_url, timezone, created_at, updated_at
	`
	var out Profile
	err := tx.QueryRow(ctx, q, p.UserID, p.FirstName, p.LastName, p.AvatarURL, p.Timezone).Scan(
		&out.UserID, &out.FirstName, &out.LastName, &out.AvatarURL, &out.Timezone, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("upsert profile: %w", err)
	}
	return out, nil
}

// Delete removes the profile for a user.
func (ProfileRepository) Delete(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	const q = `DELETE FROM profiles WHERE user_id = $1`
	ct, err := tx.Exec(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
