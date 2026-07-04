package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects as jughead_app (the non-superuser role) so RLS policies
// apply, mirroring real runtime behavior. testAdmin connects as the superuser
// for seeding/cleanup which must bypass RLS.
var (
	testPool  *pgxpool.Pool
	testAdmin *pgxpool.Pool
)

const (
	testAppURL   = "postgres://jughead_app:jughead_app@localhost:5432/jughead?sslmode=disable"
	testAdminURL = "postgres://jughead:jughead@localhost:5432/jughead?sslmode=disable"
)

// TestMain connects the integration test pools when DATABASE_URL is set and
// skips the integration suite otherwise (the package has no unit tests that
// can run without a live database).
func TestMain(m *testing.M) {
	if os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL unset — skipping db integration tests")
		os.Exit(0)
	}
	ctx := context.Background()
	var err error
	testPool, err = pgxpool.New(ctx, testAppURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect app pool: %v\n", err)
		os.Exit(1)
	}
	defer testPool.Close()
	testAdmin, err = pgxpool.New(ctx, testAdminURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect admin pool: %v\n", err)
		os.Exit(1)
	}
	defer testAdmin.Close()
	os.Exit(m.Run())
}

// resetDB wipes the tenant tables so each test starts clean. Runs as the
// superuser (bypasses RLS) via testAdmin.
func resetDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	for _, tbl := range []string{"invitations", "memberships", "profiles", "users", "organizations"} {
		if _, err := testAdmin.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tbl)); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
}

// seedOrgAndUser inserts an organization + owner user + owner membership via
// the superuser pool (bypassing RLS) and returns their IDs.
func seedOrgAndUser(t *testing.T, ctx context.Context, orgName, email string) (orgID, userID uuid.UUID) {
	t.Helper()
	if err := testAdmin.QueryRow(ctx, `
		INSERT INTO organizations (name) VALUES ($1) RETURNING id
	`, orgName).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if err := testAdmin.QueryRow(ctx, `
		INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := testAdmin.Exec(ctx, `
		INSERT INTO memberships (user_id, organization_id, role) VALUES ($1, $2, 'owner')
	`, userID, orgID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return orgID, userID
}

func randomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// TestOrganizationRepository_List_ScopedToMember exercises the RLS-gated List
// through the real repository methods with a pgx.Tx closure. Alice is a member
// of Acme only, so Globex must be invisible to her.
func TestOrganizationRepository_List_ScopedToMember(t *testing.T) {
	ctx := context.Background()
	resetDB(t)

	acmeID, aliceID := seedOrgAndUser(t, ctx, "Acme", "alice@acme.com")
	_, _ = seedOrgAndUser(t, ctx, "Globex", "bob@globex.com")

	orgRepo := NewOrganizationRepository()
	scope := Scope{UserID: aliceID, OrganizationID: acmeID}

	var got []Organization
	err := WithOrgContextTx(ctx, testPool, scope, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		got, err = orgRepo.List(ctx, tx)
		return err
	})
	if err != nil {
		t.Fatalf("list orgs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 org visible to alice, got %d", len(got))
	}
	if got[0].ID != acmeID {
		t.Fatalf("expected acme, got %s", got[0].ID)
	}
}

// TestMembershipRepository_ListByOrganization exercises RLS scoping of the
// memberships table to the active organization.
func TestMembershipRepository_ListByOrganization(t *testing.T) {
	ctx := context.Background()
	resetDB(t)

	acmeID, aliceID := seedOrgAndUser(t, ctx, "Acme", "alice@acme.com")
	globexID, _ := seedOrgAndUser(t, ctx, "Globex", "bob@globex.com")

	memRepo := NewMembershipRepository()

	// Scoped to Acme: only the acme membership is visible.
	var acmeMems []Membership
	err := WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID, OrganizationID: acmeID}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		acmeMems, err = memRepo.ListByOrganization(ctx, tx, acmeID)
		return err
	})
	if err != nil {
		t.Fatalf("list acme memberships: %v", err)
	}
	if len(acmeMems) != 1 || acmeMems[0].OrganizationID != acmeID {
		t.Fatalf("expected 1 acme membership, got %+v", acmeMems)
	}

	// Same scope but asking for Globex memberships: the WHERE clause requests
	// globexID, but RLS hides them because the active org is Acme.
	var leaked []Membership
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID, OrganizationID: acmeID}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		leaked, err = memRepo.ListByOrganization(ctx, tx, globexID)
		return err
	})
	if err != nil {
		t.Fatalf("list globex memberships as alice: %v", err)
	}
	if len(leaked) != 0 {
		t.Fatalf("RLS leaked globex memberships to alice: %+v", leaked)
	}
}

// TestUserRepository_FindByEmail verifies the SECURITY DEFINER login-path
// lookup works without an RLS scope set (pre-authentication), including CITEXT
// case-insensitivity.
func TestUserRepository_FindByEmail(t *testing.T) {
	ctx := context.Background()
	resetDB(t)

	_, aliceID := seedOrgAndUser(t, ctx, "Acme", "alice@acme.com")

	repo := NewUserRepository()
	u, err := repo.FindByEmail(ctx, testPool, "alice@acme.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if u.ID != aliceID {
		t.Fatalf("expected user %s, got %s", aliceID, u.ID)
	}

	// Case-insensitive (CITEXT).
	u, err = repo.FindByEmail(ctx, testPool, "ALICE@ACME.COM")
	if err != nil {
		t.Fatalf("FindByEmail case-insensitive: %v", err)
	}
	if u.ID != aliceID {
		t.Fatalf("case-insensitive lookup mismatch")
	}

	// Missing email.
	if _, err := repo.FindByEmail(ctx, testPool, "nobody@nowhere.com"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestUserRepository_RLS_SelfOnly verifies that the RLS-gated GetByID only
// returns the caller's own user row.
func TestUserRepository_RLS_SelfOnly(t *testing.T) {
	ctx := context.Background()
	resetDB(t)

	_, aliceID := seedOrgAndUser(t, ctx, "Acme", "alice@acme.com")
	_, bobID := seedOrgAndUser(t, ctx, "Globex", "bob@globex.com")

	repo := NewUserRepository()

	// Alice can read herself.
	var alice User
	err := WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		alice, err = repo.GetByID(ctx, tx, aliceID)
		return err
	})
	if err != nil {
		t.Fatalf("alice read self: %v", err)
	}
	if alice.ID != aliceID {
		t.Fatalf("expected alice, got %s", alice.ID)
	}

	// Alice cannot read Bob via the RLS-gated path (users_self policy).
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := repo.GetByID(ctx, tx, bobID)
		return err
	})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound reading bob as alice, got %v", err)
	}
}

// TestInvitationRepository_CreateAndList exercises the full invitation lifecycle
// under the organization RLS scope.
func TestInvitationRepository_CreateAndList(t *testing.T) {
	ctx := context.Background()
	resetDB(t)

	acmeID, aliceID := seedOrgAndUser(t, ctx, "Acme", "alice@acme.com")

	repo := NewInvitationRepository()
	token := randomToken()
	expires := time.Now().Add(48 * time.Hour)

	var inv Invitation
	err := WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID, OrganizationID: acmeID}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		inv, err = repo.Create(ctx, tx, acmeID, "carol@external.com", MembershipRoleMember, token, expires)
		return err
	})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if inv.Status != InvitationStatusPending {
		t.Fatalf("expected pending status, got %s", inv.Status)
	}

	// List should contain the new invitation.
	var list []Invitation
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID, OrganizationID: acmeID}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		list, err = repo.ListByOrganization(ctx, tx, acmeID)
		return err
	})
	if err != nil {
		t.Fatalf("list invitations: %v", err)
	}
	if len(list) != 1 || list[0].Email != "carol@external.com" {
		t.Fatalf("unexpected invitation list: %+v", list)
	}

	// Accept it.
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID, OrganizationID: acmeID}, func(ctx context.Context, tx pgx.Tx) error {
		return repo.UpdateStatus(ctx, tx, inv.ID, InvitationStatusAccepted)
	})
	if err != nil {
		t.Fatalf("accept invitation: %v", err)
	}
}

// TestProfileRepository_UpsertAndFetch exercises the 1:1 profile upsert and the
// profiles_self RLS policy.
func TestProfileRepository_UpsertAndFetch(t *testing.T) {
	ctx := context.Background()
	resetDB(t)

	_, aliceID := seedOrgAndUser(t, ctx, "Acme", "alice@acme.com")
	_, bobID := seedOrgAndUser(t, ctx, "Globex", "bob@globex.com")

	repo := NewProfileRepository()

	// Alice upserts her own profile.
	err := WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := repo.Upsert(ctx, tx, &Profile{
			UserID:    aliceID,
			FirstName: "Alice",
			LastName:  "Anderson",
			Timezone:  "America/Los_Angeles",
		})
		return err
	})
	if err != nil {
		t.Fatalf("upsert alice profile: %v", err)
	}

	// Alice can read her own profile.
	var prof Profile
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		prof, err = repo.GetByUserID(ctx, tx, aliceID)
		return err
	})
	if err != nil {
		t.Fatalf("get alice profile: %v", err)
	}
	if prof.FirstName != "Alice" {
		t.Fatalf("expected first name Alice, got %s", prof.FirstName)
	}

	// Alice cannot read Bob's profile (profiles_self policy).
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := repo.GetByUserID(ctx, tx, bobID)
		return err
	})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound reading bob's profile as alice, got %v", err)
	}

	// Upsert again to verify ON CONFLICT update path.
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := repo.Upsert(ctx, tx, &Profile{
			UserID:    aliceID,
			FirstName: "Alice",
			LastName:  "Smith",
			Timezone:  "UTC",
		})
		return err
	})
	if err != nil {
		t.Fatalf("upsert alice profile (update): %v", err)
	}
	err = WithOrgContextTx(ctx, testPool, Scope{UserID: aliceID}, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		prof, err = repo.GetByUserID(ctx, tx, aliceID)
		return err
	})
	if err != nil {
		t.Fatalf("get alice profile (after update): %v", err)
	}
	if prof.LastName != "Smith" {
		t.Fatalf("expected updated last name Smith, got %s", prof.LastName)
	}
}
