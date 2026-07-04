package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
)

// OrganizationRepository provides access to the organizations table.
//
// All methods run inside an RLS-gated transaction (WithOrgContext) with the
// OrganizationID scope set; the org_is_visible_to_member policy restricts rows
// to organizations the current user belongs to.
type OrganizationRepository struct{}

// NewOrganizationRepository returns an OrganizationRepository.
func NewOrganizationRepository() *OrganizationRepository { return &OrganizationRepository{} }

// GetByID fetches a single organization by primary key.
func (OrganizationRepository) GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Organization, error) {
	const q = `
		SELECT id, name, subscription_tier, billing_status, created_at, updated_at
		FROM organizations WHERE id = $1
	`
	var o Organization
	err := tx.QueryRow(ctx, q, id).Scan(
		&o.ID, &o.Name, &o.SubscriptionTier, &o.BillingStatus, &o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, ErrNotFound
	}
	if err != nil {
		return Organization{}, fmt.Errorf("get organization by id: %w", err)
	}
	return o, nil
}

// List returns every organization visible to the current user (RLS-scoped).
func (OrganizationRepository) List(ctx context.Context, tx pgx.Tx) ([]Organization, error) {
	const q = `
		SELECT id, name, subscription_tier, billing_status, created_at, updated_at
		FROM organizations ORDER BY created_at
	`
	rows, err := tx.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	defer rows.Close()

	var out []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(
			&o.ID, &o.Name, &o.SubscriptionTier, &o.BillingStatus, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organizations: %w", err)
	}
	return out, nil
}

// Create inserts a new organization. The caller is responsible for also
// creating an owner Membership in the same transaction.
func (OrganizationRepository) Create(ctx context.Context, tx pgx.Tx, name string, tier SubscriptionTier, billing BillingStatus) (Organization, error) {
	const q = `
		INSERT INTO organizations (name, subscription_tier, billing_status)
		VALUES ($1, $2, $3)
		RETURNING id, name, subscription_tier, billing_status, created_at, updated_at
	`
	var o Organization
	err := tx.QueryRow(ctx, q, name, tier, billing).Scan(
		&o.ID, &o.Name, &o.SubscriptionTier, &o.BillingStatus, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return Organization{}, fmt.Errorf("create organization: %w", err)
	}
	return o, nil
}

// Update modifies an organization's name, tier, and billing status.
func (OrganizationRepository) Update(ctx context.Context, tx pgx.Tx, id uuid.UUID, name string, tier SubscriptionTier, billing BillingStatus) error {
	const q = `
		UPDATE organizations
		SET name = $2, subscription_tier = $3, billing_status = $4
		WHERE id = $1
	`
	ct, err := tx.Exec(ctx, q, id, name, tier, billing)
	if err != nil {
		return fmt.Errorf("update organization: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes an organization permanently (cascades to memberships +
// invitations).
func (OrganizationRepository) Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	const q = `DELETE FROM organizations WHERE id = $1`
	ct, err := tx.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
