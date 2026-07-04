package db

import (
	"time"

	"github.com/google/uuid"
)

// SubscriptionTier enumerates the features available to an organization.
type SubscriptionTier string

const (
	SubscriptionTierFree       SubscriptionTier = "free"
	SubscriptionTierPro        SubscriptionTier = "pro"
	SubscriptionTierEnterprise SubscriptionTier = "enterprise"
)

// BillingStatus enumerates the financial state of an organization.
type BillingStatus string

const (
	BillingStatusActive   BillingStatus = "active"
	BillingStatusPastDue  BillingStatus = "past_due"
	BillingStatusCanceled BillingStatus = "canceled"
	BillingStatusTrialing BillingStatus = "trialing"
)

// MembershipRole enumerates the permission level a user holds within an
// organization.
type MembershipRole string

const (
	MembershipRoleOwner  MembershipRole = "owner"
	MembershipRoleAdmin  MembershipRole = "admin"
	MembershipRoleMember MembershipRole = "member"
)

// InvitationStatus enumerates the lifecycle of an invitation.
type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusExpired  InvitationStatus = "expired"
	InvitationStatusRevoked  InvitationStatus = "revoked"
)

// User represents the individual human being — identity credentials and global
// security data only. Demographics live in Profile.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Organization represents the workspace, company, team, or subscription
// container (the billing entity).
type Organization struct {
	ID               uuid.UUID
	Name             string
	SubscriptionTier SubscriptionTier
	BillingStatus    BillingStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Membership links a User to an Organization with a per-organization role.
type Membership struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           MembershipRole
	JoinedAt       time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Profile holds personal demographics for a User (1:1). Kept separate from the
// User table so the auth path stays fast and lightweight.
type Profile struct {
	UserID    uuid.UUID
	FirstName string
	LastName  string
	AvatarURL string
	Timezone  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Invitation represents a pending membership dispatched via email.
type Invitation struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Email          string
	Role           MembershipRole
	Token          string
	Status         InvitationStatus
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
