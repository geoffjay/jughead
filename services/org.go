package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/geoffjay/jughead/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrganizationService owns the organization aggregate and the signup flow that
// creates an organization with an owner membership in a single transaction.
type OrganizationService struct {
	Orgs        OrganizationRepo
	Memberships MembershipRepo
	Pool        *pgxpool.Pool
}

// NewOrganizationService wires an OrganizationService with the concrete db
// repositories and the shared connection pool.
func NewOrganizationService(pool *pgxpool.Pool) *OrganizationService {
	return &OrganizationService{
		Orgs:        db.NewOrganizationRepository(),
		Memberships: db.NewMembershipRepository(),
		Pool:        pool,
	}
}

// CreateWithOwner is the full signup flow: creates an organization and an owner
// membership linking the given user to it, in a single transaction. The scope
// must carry the new owner's UserID and a uuid.Nil OrganizationID (the org
// doesn't exist yet — the WITH CHECK policy on organizations requires the
// membership to exist, so this method seeds it via the same connection before
// the RLS check fires on subsequent queries).
//
// Note: The org_is_visible_to_member WITH CHECK policy on organizations
// requires a membership row to exist for current_user_id. Because the INSERT
// into organizations and the INSERT into memberships happen in the same
// transaction, the membership is created immediately after the org insert
// succeeds, and the transaction commits atomically.
func (s *OrganizationService) CreateWithOwner(
	ctx context.Context,
	scope db.Scope,
	name string,
	ownerID uuid.UUID,
) (db.Organization, error) {
	var org db.Organization
	err := db.WithOrgContextTx(ctx, s.poolOrCtx(ctx), db.Scope{UserID: scope.UserID}, func(ctx context.Context, tx pgx.Tx) error {
		// Insert the organization. The WITH CHECK policy on organizations
		// validates membership existence at commit; the membership insert
		// below satisfies it within the same transaction.
		o, err := s.Orgs.Create(ctx, tx, name, db.SubscriptionTierFree, db.BillingStatusTrialing)
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}

		// Link the owner via a membership in the same transaction.
		if _, err := s.Memberships.Create(ctx, tx, ownerID, o.ID, db.MembershipRoleOwner); err != nil {
			return fmt.Errorf("create owner membership: %w", err)
		}

		org = o
		return nil
	})
	if err != nil {
		return db.Organization{}, err
	}
	return org, nil
}

// List returns every organization visible to the scoped user (RLS-restricted
// to orgs the user is a member of).
func (s *OrganizationService) List(ctx context.Context, scope db.Scope) ([]db.Organization, error) {
	var orgs []db.Organization
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		orgs, err = s.Orgs.List(ctx, tx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	return orgs, nil
}

// GetByID fetches a single organization. Requires the OrganizationID scope
// (the user must be a member).
func (s *OrganizationService) GetByID(ctx context.Context, scope db.Scope, id uuid.UUID) (db.Organization, error) {
	var org db.Organization
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		org, err = s.Orgs.GetByID(ctx, tx, id)
		return err
	})
	if err != nil {
		return db.Organization{}, fmt.Errorf("get organization: %w", err)
	}
	return org, nil
}

// Update modifies an organization's name, tier, and billing status. Requires
// the OrganizationID scope.
func (s *OrganizationService) Update(
	ctx context.Context,
	scope db.Scope,
	id uuid.UUID,
	name string,
	tier db.SubscriptionTier,
	billing db.BillingStatus,
) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Orgs.Update(ctx, tx, id, name, tier, billing)
	})
}

// Delete permanently removes an organization (cascades to memberships and
// invitations). Requires the OrganizationID scope.
func (s *OrganizationService) Delete(ctx context.Context, scope db.Scope, id uuid.UUID) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Orgs.Delete(ctx, tx, id)
	})
}

func (s *OrganizationService) poolOrCtx(ctx context.Context) *pgxpool.Pool {
	if pool, err := db.PoolFromContext(ctx); err == nil {
		return pool
	}
	return s.Pool
}
