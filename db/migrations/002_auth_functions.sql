-- Auth helpers: SECURITY DEFINER functions for the login path.
--
-- The users table has FORCE RLS with a users_self policy gating on
-- app.current_user_id. Before authentication, that GUC is unset, so a plain
-- SELECT on users returns nothing for the non-superuser jughead_app role.
--
-- SECURITY DEFINER functions run with the owner's privileges (bypassing RLS)
-- and provide the narrow, read-only lookups the login flow needs. They are
-- granted to jughead_app so the app can call them without holding bypass rights.

-- find_user_by_email returns the login record for a given email. Used by the
-- password-login flow to verify credentials before a session is created.
CREATE OR REPLACE FUNCTION find_user_by_email(p_email CITEXT)
RETURNS TABLE (
    id            UUID,
    email         CITEXT,
    password_hash TEXT,
    is_active     BOOLEAN,
    created_at    TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ
)
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT id, email, password_hash, is_active, created_at, updated_at
    FROM users
    WHERE email = p_email
$$;

-- find_user_by_id returns the login record for a given id. Used to re-hydrate
-- the authenticated identity on subsequent requests without requiring the
-- app.current_user_id GUC to be set first.
CREATE OR REPLACE FUNCTION find_user_by_id(p_id UUID)
RETURNS TABLE (
    id            UUID,
    email         CITEXT,
    password_hash TEXT,
    is_active     BOOLEAN,
    created_at    TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ
)
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT id, email, password_hash, is_active, created_at, updated_at
    FROM users
    WHERE id = p_id
$$;

GRANT EXECUTE ON FUNCTION find_user_by_email(CITEXT) TO jughead_app;
GRANT EXECUTE ON FUNCTION find_user_by_id(UUID) TO jughead_app;