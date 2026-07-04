package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/geoffjay/jughead/db"
	"github.com/jackc/pgx/v5"
)

// The port interfaces mirror the exact method sets the concrete db repository
// structs expose, so db.NewUserRepository() etc. satisfy them without any
// adapter code. Defining them here keeps the service package decoupled from the
// db package's concrete types and makes fakes trivial in tests.

// UserRepo is the repository contract for the users table.
type UserRepo interface {
	FindByEmail(ctx context.Context, q db.Querier, email string) (db.User, error)
	FindByID(ctx context.Context, q db.Querier, id uuid.UUID) (db.User, error)
	GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (db.User, error)
	Create(ctx context.Context, tx pgx.Tx, email, passwordHash string) (db.User, error)
	UpdateIsActive(ctx context.Context, tx pgx.Tx, id uuid.UUID, active bool) error
	UpdatePasswordHash(ctx context.Context, tx pgx.Tx, id uuid.UUID, hash string) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}

// ProfileRepo is the repository contract for the profiles table.
type ProfileRepo interface {
	GetByUserID(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (db.Profile, error)
	Upsert(ctx context.Context, tx pgx.Tx, p *db.Profile) (db.Profile, error)
	Delete(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error
}

// OrganizationRepo is the repository contract for the organizations table.
type OrganizationRepo interface {
	GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (db.Organization, error)
	List(ctx context.Context, tx pgx.Tx) ([]db.Organization, error)
	Create(ctx context.Context, tx pgx.Tx, name string, tier db.SubscriptionTier, billing db.BillingStatus) (db.Organization, error)
	Update(ctx context.Context, tx pgx.Tx, id uuid.UUID, name string, tier db.SubscriptionTier, billing db.BillingStatus) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}

// MembershipRepo is the repository contract for the memberships table.
type MembershipRepo interface {
	GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (db.Membership, error)
	ListByOrganization(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]db.Membership, error)
	ListByUser(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]db.Membership, error)
	Create(ctx context.Context, tx pgx.Tx, userID, orgID uuid.UUID, role db.MembershipRole) (db.Membership, error)
	UpdateRole(ctx context.Context, tx pgx.Tx, id uuid.UUID, role db.MembershipRole) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}

// InvitationRepo is the repository contract for the invitations table.
type InvitationRepo interface {
	GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (db.Invitation, error)
	GetByToken(ctx context.Context, tx pgx.Tx, token string) (db.Invitation, error)
	ListByOrganization(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) ([]db.Invitation, error)
	Create(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, email string, role db.MembershipRole, token string, expiresAt time.Time) (db.Invitation, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status db.InvitationStatus) error
	Delete(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}
