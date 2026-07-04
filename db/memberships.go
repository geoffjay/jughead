package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
)

// MembershipRepository provides access to the memberships junction table.
//
// The memberships_visible policy allows a row to be read when its
// organization_id matches app.current_organization_id OR its user_id matches
// app.current_user_id, so callers should set whichever scope is relevant:
//   - ListByOrganization: set OrganizationID
//   - ListByUser:         set UserID
type MembershipRepository struct{}

// NewMembershipRepository returns a MembershipRepository.
func NewMembershipRepository() *MembershipRepository { return &MembershipRepository{} }

// GetByID fetches a single membership by primary key.
func (MembershipRepository) GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Membership, error) {
	const q = `
		SELECT id, user_id, organization_id, role, joined_at, created_at, updated_at
		FROM memberships WHERE id = $1
	`
	var m Membership
	err := tx.QueryRow(ctx, q, id).Scan(
		&m.ID, &m.UserID, &m.OrganizationID, &m.Role, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrNotFound
	}
	if err != nil {
		return Membership{}, fmt.Errorf("get membership by id: %w", err)
	}
	return m, nil
}

// ListByOrganization returns all memberships for the active organization
// (requires the OrganizationID scope).
func (MembershipRepository) ListByOrganization(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]Membership, error) {
	const q = `
		SELECT id, user_id, organization_id, role, joined_at, created_at, updated_at
		FROM memberships WHERE organization_id = $1 ORDER BY joined_at
	`
	rows, err := tx.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list memberships by org: %w", err)
	}
	return scanMemberships(rows)
}

// ListByUser returns all memberships for a user across organizations (requires
// the UserID scope; the memberships_visible policy admits rows where
// user_id = current_user_id).
func (MembershipRepository) ListByUser(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]Membership, error) {
	const q = `
		SELECT id, user_id, organization_id, role, joined_at, created_at, updated_at
		FROM memberships WHERE user_id = $1 ORDER BY joined_at
	`
	rows, err := tx.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships by user: %w", err)
	}
	return scanMemberships(rows)
}

// scanMemberships drains a rows iterator into a slice, closing the rows and
// surfacing iteration errors. Shared by ListByOrganization and ListByUser to
// avoid duplicating the scan loop.
func scanMemberships(rows pgx.Rows) ([]Membership, error) {
	defer rows.Close()

	var out []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.OrganizationID, &m.Role, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships: %w", err)
	}
	return out, nil
}

// Create adds a user to an organization with the given role. The unique
// (user_id, organization_id) constraint prevents duplicates.
func (MembershipRepository) Create(ctx context.Context, tx pgx.Tx, userID, orgID uuid.UUID, role MembershipRole) (Membership, error) {
	const q = `
		INSERT INTO memberships (user_id, organization_id, role)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, organization_id, role, joined_at, created_at, updated_at
	`
	var m Membership
	err := tx.QueryRow(ctx, q, userID, orgID, role).Scan(
		&m.ID, &m.UserID, &m.OrganizationID, &m.Role, &m.JoinedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return Membership{}, fmt.Errorf("create membership: %w", err)
	}
	return m, nil
}

// UpdateRole changes the role of an existing membership.
func (MembershipRepository) UpdateRole(ctx context.Context, tx pgx.Tx, id uuid.UUID, role MembershipRole) error {
	const q = `UPDATE memberships SET role = $2 WHERE id = $1`
	ct, err := tx.Exec(ctx, q, id, role)
	if err != nil {
		return fmt.Errorf("update membership role: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a membership (revokes the user's access to the organization).
func (MembershipRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	const q = `DELETE FROM memberships WHERE id = $1`
	ct, err := tx.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
