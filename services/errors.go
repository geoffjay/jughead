// Package services implements the application service layer. Each service owns
// a transaction boundary (db.WithOrgContext) and encodes the business rules for
// one aggregate root, coordinating the repositories in the db package.
//
// Handlers depend on services, never on repositories. Services depend on
// repository interfaces (ports.go), not on the concrete db structs, so the
// service layer is testable without a live database.
package services

import "errors"

// Service-level errors. These map repository-level failures onto
// domain-meaningful outcomes so handlers can branch on them without importing
// the db package.
var (
	// ErrInvalidCredentials is returned by Authenticate when the email is
	// unknown or the password does not match the stored hash.
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrUserInactive is returned by Authenticate when the user account has
	// been deactivated (is_active = false).
	ErrUserInactive = errors.New("user account is inactive")

	// ErrEmailTaken is returned by Register when a user with the email
	// already exists.
	ErrEmailTaken = errors.New("email is already registered")

	// ErrAlreadyMember is returned when attempting to add a user to an
	// organization they already belong to.
	ErrAlreadyMember = errors.New("user is already a member of this organization")

	// ErrInviteExpired is returned when attempting to accept an invitation
	// past its expiry.
	ErrInviteExpired = errors.New("invitation has expired")

	// ErrInviteRevoked is returned when attempting to accept an invitation
	// that has been revoked.
	ErrInviteRevoked = errors.New("invitation has been revoked")

	// ErrInviteAlreadyAccepted is returned when attempting to accept an
	// invitation that was already accepted.
	ErrInviteAlreadyAccepted = errors.New("invitation has already been accepted")
)
