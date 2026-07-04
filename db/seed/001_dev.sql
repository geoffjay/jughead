-- Development seed data.
--
-- Creates two users (admin@jughead, jughead@jughead) sharing one organization
-- (jughead), with owner and member memberships respectively. Both passwords
-- are "jughead" — this file is for local development ONLY and must never run
-- against a production database.
--
-- Run as the superuser (jughead) so RLS is bypassed. The script is idempotent:
-- re-running it updates the password hashes in place and leaves the
-- organization/membership rows untouched.
--
-- Usage:
--   task db:seed
--   # or directly:
--   docker compose exec -T postgres psql -U jughead -d jughead -f /dev/stdin < db/seed/001_dev.sql

-- Only allow seeding when the database name is "jughead" to prevent accidental
-- use against a non-dev database.
DO $$
BEGIN
    IF current_database() <> 'jughead' THEN
        RAISE EXCEPTION 'Refusing to seed: current_database() is %, expected "jughead"', current_database();
    END IF;
END $$;

-- ----------------------------------------------------------------------------
-- Organization
-- ----------------------------------------------------------------------------

-- The dev organization. INSERT ... ON CONFLICT keeps the row stable across
-- re-seeds so foreign keys (memberships, invitations) survive.
INSERT INTO organizations (id, name, subscription_tier, billing_status)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'jughead',
    'free',
    'active'
)
ON CONFLICT (id) DO UPDATE
    SET name = EXCLUDED.name,
        subscription_tier = EXCLUDED.subscription_tier,
        billing_status = EXCLUDED.billing_status;

-- ----------------------------------------------------------------------------
-- Users
-- ----------------------------------------------------------------------------
--
-- pgcrypto's crypt() with gen_salt('bf', 8) produces a bcrypt hash that is
-- compatible with Go's bcrypt.CompareHashAndPassword (the prefix is $2a$).
-- The cost factor 8 keeps seeding fast while remaining bcrypt-standard.

-- admin@jughead — the owner of the jughead organization.
INSERT INTO users (id, email, password_hash, is_active)
VALUES (
    '00000000-0000-0000-0000-000000000010',
    'admin@jughead',
    crypt('jughead', gen_salt('bf', 8)),
    TRUE
)
ON CONFLICT (id) DO UPDATE
    SET password_hash = EXCLUDED.password_hash,
        is_active = TRUE;

-- Restore the email conflict behavior: if a user with this email already
-- exists under a different id, update the existing row instead of erroring.
INSERT INTO users (id, email, password_hash, is_active)
VALUES (
    '00000000-0000-0000-0000-000000000011',
    'jughead@jughead',
    crypt('jughead', gen_salt('bf', 8)),
    TRUE
)
ON CONFLICT (id) DO UPDATE
    SET password_hash = EXCLUDED.password_hash,
        is_active = TRUE;

-- ----------------------------------------------------------------------------
-- Memberships
-- ----------------------------------------------------------------------------

-- admin is the owner of the jughead organization.
INSERT INTO memberships (user_id, organization_id, role)
VALUES (
    '00000000-0000-0000-0000-000000000010',
    '00000000-0000-0000-0000-000000000001',
    'owner'
)
ON CONFLICT (user_id, organization_id) DO UPDATE
    SET role = EXCLUDED.role;

-- jughead is a regular member of the jughead organization.
INSERT INTO memberships (user_id, organization_id, role)
VALUES (
    '00000000-0000-0000-0000-000000000011',
    '00000000-0000-0000-0000-000000000001',
    'member'
)
ON CONFLICT (user_id, organization_id) DO UPDATE
    SET role = EXCLUDED.role;

-- ----------------------------------------------------------------------------
-- Profiles
-- ----------------------------------------------------------------------------

INSERT INTO profiles (user_id, first_name, last_name, timezone)
VALUES (
    '00000000-0000-0000-0000-000000000010',
    'Admin',
    'User',
    'UTC'
)
ON CONFLICT (user_id) DO UPDATE
    SET first_name = EXCLUDED.first_name,
        last_name = EXCLUDED.last_name,
        timezone = EXCLUDED.timezone;

INSERT INTO profiles (user_id, first_name, last_name, timezone)
VALUES (
    '00000000-0000-0000-0000-000000000011',
    'Jughead',
    'User',
    'UTC'
)
ON CONFLICT (user_id) DO UPDATE
    SET first_name = EXCLUDED.first_name,
        last_name = EXCLUDED.last_name,
        timezone = EXCLUDED.timezone;

-- ----------------------------------------------------------------------------
-- Verify
-- ----------------------------------------------------------------------------

-- Sanity check: print a summary of what was seeded.
SELECT
    u.email,
    m.role,
    o.name AS organization
FROM users u
JOIN memberships m ON m.user_id = u.id
JOIN organizations o ON o.id = m.organization_id
WHERE o.name = 'jughead'
ORDER BY u.email;