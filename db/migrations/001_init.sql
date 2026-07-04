-- Multi-tenancy schema for jughead.
--
-- Implements the User / Organization / Membership / Profile / Invitation data
-- model. Row-Level Security (RLS) is enabled on every tenant-scoped table so a
-- single connection can be gated by `SET LOCAL app.current_organization_id`.
--
-- The "accounts" entity from the source model is named "organizations" here,
-- which better represents the company paying for the service.
--
-- Roles:
--   jughead      — superuser/migration owner (POSTGRES_USER). Bypasses RLS.
--                  Used ONLY for schema migrations.
--   jughead_app  — the role the application connects as at runtime. Non-superuser,
--                  no BYPASSRLS. Subject to all RLS policies.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";

-- ============================================================================
-- Application role
-- ============================================================================

-- Non-superuser, non-BYPASSRLS role so RLS actually applies at runtime.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'jughead_app') THEN
        CREATE ROLE jughead_app LOGIN PASSWORD 'jughead_app';
    END IF;
END
$$;

ALTER ROLE jughead_app NOSUPERUSER NOBYPASSRLS;

-- ============================================================================
-- Enumerations
-- ============================================================================

-- Enums lack IF NOT EXISTS, so each is guarded by a DO block that catches the
-- duplicate_object error. This makes the migration idempotent for `psql -f`.
DO $$ BEGIN CREATE TYPE subscription_tier AS ENUM ('free', 'pro', 'enterprise'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE billing_status AS ENUM ('active', 'past_due', 'canceled', 'trialing'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE membership_role AS ENUM ('owner', 'admin', 'member'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN CREATE TYPE invitation_status AS ENUM ('pending', 'accepted', 'expired', 'revoked'); EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ============================================================================
-- users — authentication & identity
-- ============================================================================

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lightweight audit trigger for updated_at.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_set_updated_at ON users;
CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- organizations — the business/billing entity
-- ============================================================================

CREATE TABLE IF NOT EXISTS organizations (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name               TEXT NOT NULL,
    subscription_tier  subscription_tier NOT NULL DEFAULT 'free',
    billing_status     billing_status NOT NULL DEFAULT 'active',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS organizations_set_updated_at ON organizations;
CREATE TRIGGER organizations_set_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- memberships — junction linking users to organizations
-- ============================================================================

CREATE TABLE IF NOT EXISTS memberships (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    role            membership_role NOT NULL DEFAULT 'member',
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT memberships_user_org_unique UNIQUE (user_id, organization_id)
);

CREATE INDEX IF NOT EXISTS idx_memberships_organization_id ON memberships(organization_id);
CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON memberships(user_id);

DROP TRIGGER IF EXISTS memberships_set_updated_at ON memberships;
CREATE TRIGGER memberships_set_updated_at
    BEFORE UPDATE ON memberships
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- profiles — personal demographics (1:1 with users, kept out of auth path)
-- ============================================================================

CREATE TABLE IF NOT EXISTS profiles (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    first_name  TEXT,
    last_name   TEXT,
    avatar_url  TEXT,
    timezone    TEXT DEFAULT 'UTC',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS profiles_set_updated_at ON profiles;
CREATE TRIGGER profiles_set_updated_at
    BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- invitations — pending memberships
-- ============================================================================

CREATE TABLE IF NOT EXISTS invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           CITEXT NOT NULL,
    role            membership_role NOT NULL DEFAULT 'member',
    token           TEXT UNIQUE NOT NULL,
    status          invitation_status NOT NULL DEFAULT 'pending',
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invitations_organization_id ON invitations(organization_id);
CREATE INDEX IF NOT EXISTS idx_invitations_email ON invitations(email);
CREATE INDEX IF NOT EXISTS idx_invitations_token ON invitations(token);

DROP TRIGGER IF EXISTS invitations_set_updated_at ON invitations;
CREATE TRIGGER invitations_set_updated_at
    BEFORE UPDATE ON invitations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================================
-- Row-Level Security
--
-- Tenant isolation works as follows:
--   1. The application sets `SET LOCAL app.current_organization_id` (and
--      `app.current_user_id`) on each transaction/pooled connection.
--   2. RLS policies restrict tenant-scoped rows to the active organization.
--   3. `users` and `profiles` are global identity tables — a user can log in
--      without an organization context, so they are gated by `app.current_user_id`
--      only, not by organization.
--
-- The `jughead_app` role is non-superuser and has no BYPASSRLS, so policies
-- apply. The `jughead` superuser/owner is for migrations only.
-- ============================================================================

ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE memberships   ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations   ENABLE ROW LEVEL SECURITY;
ALTER TABLE users         ENABLE ROW LEVEL SECURITY;
ALTER TABLE profiles      ENABLE ROW LEVEL SECURITY;

-- FORCE so even the table owner is subject to RLS (defense in depth).
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
ALTER TABLE memberships   FORCE ROW LEVEL SECURITY;
ALTER TABLE invitations   FORCE ROW LEVEL SECURITY;
ALTER TABLE users         FORCE ROW LEVEL SECURITY;
ALTER TABLE profiles      FORCE ROW LEVEL SECURITY;

-- Grant DML on tenant tables to the app role. Schema changes stay with the
-- migration owner.
GRANT SELECT, INSERT, UPDATE, DELETE ON users, organizations, memberships, profiles, invitations TO jughead_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO jughead_app;

-- Helper to read the active organization GUC safely.
CREATE OR REPLACE FUNCTION current_organization_id() RETURNS UUID AS $$
    SELECT NULLIF(current_setting('app.current_organization_id', TRUE), '')::UUID;
$$ LANGUAGE SQL STABLE;

CREATE OR REPLACE FUNCTION current_user_id() RETURNS UUID AS $$
    SELECT NULLIF(current_setting('app.current_user_id', TRUE), '')::UUID;
$$ LANGUAGE SQL STABLE;

-- organizations: a user may only see orgs they belong to.
DROP POLICY IF EXISTS org_is_visible_to_member ON organizations;
CREATE POLICY org_is_visible_to_member ON organizations
    FOR ALL
    USING (
        id IN (
            SELECT m.organization_id
            FROM memberships m
            WHERE m.user_id = current_user_id()
        )
    )
    WITH CHECK (
        id IN (
            SELECT m.organization_id
            FROM memberships m
            WHERE m.user_id = current_user_id()
        )
    );

-- memberships: visible within the active organization, or to the user
-- themselves (so a user can list their own memberships across orgs).
DROP POLICY IF EXISTS memberships_visible ON memberships;
CREATE POLICY memberships_visible ON memberships
    FOR ALL
    USING (
        organization_id = current_organization_id()
        OR user_id = current_user_id()
    )
    WITH CHECK (
        organization_id = current_organization_id()
        OR user_id = current_user_id()
    );

-- invitations: scoped to the active organization.
DROP POLICY IF EXISTS invitations_visible ON invitations;
CREATE POLICY invitations_visible ON invitations
    FOR ALL
    USING (organization_id = current_organization_id())
    WITH CHECK (organization_id = current_organization_id());

-- users: a user only sees their own identity row.
DROP POLICY IF EXISTS users_self ON users;
CREATE POLICY users_self ON users
    FOR ALL
    USING (id = current_user_id())
    WITH CHECK (id = current_user_id());

-- profiles: a user only sees their own profile.
DROP POLICY IF EXISTS profiles_self ON profiles;
CREATE POLICY profiles_self ON profiles
    FOR ALL
    USING (user_id = current_user_id())
    WITH CHECK (user_id = current_user_id());