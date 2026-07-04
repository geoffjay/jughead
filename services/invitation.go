package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/geoffjay/jughead/db"

	"github.com/jackc/pgx/v5"
)

// defaultInviteTTL is the default validity window for a new invitation.
const defaultInviteTTL = 48 * time.Hour

// tokenBytes is the length of random bytes generated for invitation tokens
// (32 bytes = 256 bits, rendered as a 64-char hex string).
const tokenBytes = 32

// InvitationService owns the invitation lifecycle: creating, revoking, and
// accepting pending memberships. All methods run inside db.WithOrgContext,
// which resolves the pool from the request context.
type InvitationService struct {
	Invitations InvitationRepo
	Memberships MembershipRepo
	Users       UserRepo
}

// NewInvitationService wires an InvitationService with the concrete db
// repositories. The pool is resolved per-request from the context (attached by
// the middleware in server.go).
func NewInvitationService() *InvitationService {
	return &InvitationService{
		Invitations: db.NewInvitationRepository(),
		Memberships: db.NewMembershipRepository(),
		Users:       db.NewUserRepository(),
	}
}

// Invite creates a pending invitation for email to join the scoped organization
// with the given role. A cryptographically-random token is generated for the
// accept link. Requires the OrganizationID scope.
func (s *InvitationService) Invite(
	ctx context.Context,
	scope db.Scope,
	email string,
	role db.MembershipRole,
) (db.Invitation, error) {
	token, err := generateToken()
	if err != nil {
		return db.Invitation{}, fmt.Errorf("generate invite token: %w", err)
	}
	expiresAt := time.Now().Add(defaultInviteTTL)

	var inv db.Invitation
	err = db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		inv, err = s.Invitations.Create(ctx, tx, scope.OrganizationID, email, role, token, expiresAt)
		return err
	})
	if err != nil {
		return db.Invitation{}, fmt.Errorf("create invitation: %w", err)
	}
	return inv, nil
}

// List returns all invitations for the scoped organization. Requires the
// OrganizationID scope.
func (s *InvitationService) List(ctx context.Context, scope db.Scope) ([]db.Invitation, error) {
	var invs []db.Invitation
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		invs, err = s.Invitations.ListByOrganization(ctx, tx, scope.OrganizationID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	return invs, nil
}

// Revoke marks an invitation as revoked. Requires the OrganizationID scope.
func (s *InvitationService) Revoke(ctx context.Context, scope db.Scope, id uuid.UUID) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Invitations.UpdateStatus(ctx, tx, id, db.InvitationStatusRevoked)
	})
}

// Delete permanently removes an invitation. Requires the OrganizationID scope.
func (s *InvitationService) Delete(ctx context.Context, scope db.Scope, id uuid.UUID) error {
	return db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		return s.Invitations.Delete(ctx, tx, id)
	})
}

// Accept is called by the invitee to accept an invitation and join the
// organization. It validates the invitation's status and expiry, then creates a
// membership and marks the invitation as accepted — all in one transaction.
//
// The caller must pass the invitation's organization ID as scope.OrganizationID
// (the invitations_visible policy requires it). To discover the org ID from a
// token pre-acceptance, a SECURITY DEFINER find_invitation_by_token function
// would be needed (analogous to find_user_by_email in 002_auth_functions.sql).
// Until that migration exists, the caller must obtain the org ID out-of-band,
// e.g. from an email link that encodes it.
func (s *InvitationService) Accept(ctx context.Context, scope db.Scope, token string, userID uuid.UUID) (db.Membership, error) {
	var mem db.Membership
	err := db.WithOrgContext(ctx, scope, func(ctx context.Context, tx pgx.Tx) error {
		inv, err := s.Invitations.GetByToken(ctx, tx, token)
		if err != nil {
			return fmt.Errorf("load invitation: %w", err)
		}

		// Guard against double-accept and revocation.
		switch inv.Status {
		case db.InvitationStatusAccepted:
			return ErrInviteAlreadyAccepted
		case db.InvitationStatusRevoked:
			return ErrInviteRevoked
		case db.InvitationStatusExpired:
			return ErrInviteExpired
		}

		if time.Now().After(inv.ExpiresAt) {
			// Normalize: mark as expired in the DB so future lookups see it.
			if err := s.Invitations.UpdateStatus(ctx, tx, inv.ID, db.InvitationStatusExpired); err != nil {
				return fmt.Errorf("mark invitation expired: %w", err)
			}
			return ErrInviteExpired
		}

		// The invited email must match the accepting user's email. Load the
		// user via the SECURITY DEFINER function (the RLS-gated GetByID would
		// fail because the user may not belong to any org yet).
		u, err := s.Users.FindByID(ctx, tx, userID)
		if err != nil {
			return fmt.Errorf("load accepting user: %w", err)
		}
		if u.Email != inv.Email {
			return ErrInvalidCredentials
		}

		// Create the membership and flip the invitation status atomically.
		mem, err = s.Memberships.Create(ctx, tx, userID, inv.OrganizationID, inv.Role)
		if err != nil {
			return fmt.Errorf("create membership from invitation: %w", err)
		}
		if err := s.Invitations.UpdateStatus(ctx, tx, inv.ID, db.InvitationStatusAccepted); err != nil {
			return fmt.Errorf("mark invitation accepted: %w", err)
		}
		return nil
	})
	if err != nil {
		return db.Membership{}, err
	}
	return mem, nil
}

// generateToken produces a 32-byte cryptographically-random hex string for use
// as an invitation accept-link token.
func generateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
