package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/geoffjay/jughead/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MembershipService owns the membership junction: adding/removing users to/from
// organizations and changing their roles. All methods run inside
// db.WithOrgContext, which resolves the pool from the request context.
type MembershipService struct {
	Memberships MembershipRepo
}

// NewMembershipService wires a MembershipService with the concrete db
// repository. The pool is resolved per-request from the context (attached by
// the middleware in server.go).
func NewMembershipService() *MembershipService {
	return &MembershipService{
		Memberships: db.NewMembershipRepository(),
	}
}

// ListByOrganization returns all members of the scoped organization. Requires
// the OrganizationID scope.
func (s *MembershipService) ListByOrganization(ctx context.Context, scope db.Scope, orgID uuid.UUID) ([]db.Membership, error) {
	var mems []db.Membership
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		mems, err = s.Memberships.ListByOrganization(ctx, tx, orgID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}
	return mems, nil
}

// ListByUser returns all memberships for a user across organizations. Requires
// the UserID scope (the memberships_visible policy admits rows where
// user_id = current_user_id).
func (s *MembershipService) ListByUser(ctx context.Context, scope db.Scope, userID uuid.UUID) ([]db.Membership, error) {
	var mems []db.Membership
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		mems, err = s.Memberships.ListByUser(ctx, tx, userID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list user memberships: %w", err)
	}
	return mems, nil
}

// AddMember creates a membership linking a user to the scoped organization with
// the given role. Requires the OrganizationID scope.
func (s *MembershipService) AddMember(ctx context.Context, scope db.Scope, userID uuid.UUID, role db.MembershipRole) (db.Membership, error) {
	var mem db.Membership
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		mem, err = s.Memberships.Create(ctx, tx, userID, scope.OrganizationID, role)
		if err != nil {
			// The unique (user_id, organization_id) constraint prevents
			// duplicates; map the violation to a domain error.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrAlreadyMember
			}
		}
		return err
	})
	if err != nil {
		return db.Membership{}, fmt.Errorf("add member: %w", err)
	}
	return mem, nil
}

// ChangeRole updates the role of an existing membership. Requires the
// OrganizationID scope.
func (s *MembershipService) ChangeRole(ctx context.Context, scope db.Scope, membershipID uuid.UUID, role db.MembershipRole) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Memberships.UpdateRole(ctx, tx, membershipID, role)
	})
}

// RemoveMember deletes a membership, revoking the user's access to the scoped
// organization. Requires the OrganizationID scope.
func (s *MembershipService) RemoveMember(ctx context.Context, scope db.Scope, membershipID uuid.UUID) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Memberships.Delete(ctx, tx, membershipID)
	})
}
