# Jughead — Project Review

_Date: 2026-06-26 · Scope: full repository review (architecture, code quality, testing, security)_

This document is advisory only. No source code was changed in producing it. Every finding cites a concrete `file:line` reference so it can be acted on directly.

---

## 1. Executive Summary

**Jughead** is a multi-tenant Go web server (Gin + [templ](https://github.com/a-h/templ) + HTMX + DaisyUI) that hosts multiple independent "sites" from a single binary. Each site is reachable two ways — directly under `/sites/<path>` and via its FQDN through a built-in reverse proxy — and may opt into per-site authentication through a pluggable provider registry (GitHub OAuth is the one implemented provider). It also ships an admin panel, an in-memory session store, and a GitHub PR-review site ("quux"). It deploys to Fly.io.

**Overall health: a well-architected demo/dogfood application with strong foundations, held back from production-readiness by pre-production auth, no CI quality gate, and uneven test coverage.** The design is genuinely good — clean separation of concerns, an acyclic provider/auth/sites dependency graph, a correct and careful GitHub OAuth flow, a comprehensive linter config, and structured logging in the core paths. The gaps are the kind you expect in something still under active construction.

**Top things to address, in order:**

1. **Hardcoded admin credentials** (`admin`/`password`) gate the entire `/admin` panel (Critical).
2. **No CI quality gate** — pushes to `main` deploy straight to Fly with no `go test`/`vet`/lint step, even though a thorough `.golangci.yml` already exists (High).
3. **In-memory sessions** lose all logins (and GitHub tokens) on restart and never reap expired entries (High).
4. **Open-redirect risk** on both login paths and **missing security headers / login CSRF token** (High).

---

## 2. Architecture Overview

Request pipeline:

```
HTTP request
  → middleware.ReverseProxy        (FQDN → /sites/<path> upstream; bypasses /static, /auth)
  → Gin router
      → public routes (/, /api/hello-world)
      → per-site routes             (sites.SiteManager.BuildSiteRoutes — optionally wrapped in a provider AuthMiddleware)
      → /auth/<provider>/...        (OAuth login/callback/logout, mounted once per provider)
      → /login, /logout             (admin password auth)
      → /admin/*                    (middleware.AuthRequired)
```

Key structural strengths worth preserving:

- **Provider registry as a contract layer** (`sites/auth/auth.go`). Sites declare `Auth.Provider = "github"`; the `SiteManager` looks the provider up at startup and wires its routes/middleware. The dependency direction is strictly `provider → auth contract → sites` (acyclic) — `sites` never imports a concrete provider. `services/github` was recently refactored to own its own config to reinforce this (commit `b85ce14`).
- **Dual access modes** handled centrally. `middleware/proxy.go` maps a site's FQDN to its localhost site-path upstream, and `sites/links` resolves URLs correctly for both proxied and direct access. This is the most non-obvious part of the system and is reasonably well-commented.
- **Singleton `SiteManager`** (`sites/manager.go`) registers sites based on `ShouldLoad()` (published sites always; unpublished only in dev) and de-dupes provider routes across sites.

---

## 3. Strengths

- **Clean separation of concerns with acyclic dependencies** — the provider registry pattern is well-documented at `sites/auth/auth.go:1-10` and faithfully followed.
- **Solid GitHub OAuth implementation** — CSRF `state` nonce stored in a short-lived secure cookie and validated on callback, `Secure`/`HttpOnly`/`SameSite=Lax` cookies, a 30s context timeout on the token exchange, `io.LimitReader` caps on response bodies, and an allowed-login restriction (`services/github/oauth.go`, `services/github/provider.go:104-164`).
- **Comprehensive, modern linter config** — `.golangci.yml` enables ~30 linters including `gosec`, `gocyclo` (complexity 15), `funlen` (120), `errcheck`, `errorlint`, `bodyclose`, and `mnd`, with pragmatic exclusions for generated/test/fixture code.
- **Structured logging** via `log/slog` in `main.go`, `server.go`, and `sites/manager.go`.
- **Good documentation of intent** — package-level godoc and inline comments explain non-obvious decisions (`services/github/client.go:1-13`, `sites/links/links.go:10-22`, `middleware/proxy.go:14-30`).
- **Reasonable test patterns where present** — table-driven tests, fake provider/instance for auth (`sites/manager_test.go:144-175`), and full-router integration tests built without `ListenAndServe` (`main_test.go`).
- **Lean, correct production image** — multi-stage build to `scratch`, CA certs copied for outbound TLS, `CGO_ENABLED=0`, stripped binary (`Dockerfile`).

---

## 4. Findings (prioritized)

### 🔴 Critical

**C1. Hardcoded admin credentials.**
`admin.go:18-20` defines `adminCredentials = gin.Accounts{"admin": "password"}`, compared in plaintext at `admin.go:39-41`. This single static credential gates the entire `/admin` panel (`server.go:113-121`). The comment acknowledges it as a placeholder "until a real database-backed accounts table replaces it," but as written it is a published credential in source control. **Recommendation:** move to hashed, DB- or env-backed credentials; at minimum read from an env var and compare with `subtle.ConstantTimeCompare` against a bcrypt hash.

### 🟠 High

**H1. Open-redirect risk on login flows.**
The `redirect` parameter is passed straight into `c.Redirect` without same-origin validation:
- Admin login: `admin.go:34-46` (`redirect` from the posted form).
- OAuth post-login target: `services/github/provider.go:124` via `pickPostLoginTarget` (`provider.go:190-195`), which only checks for emptiness.

A crafted `/login?redirect=https://attacker.example` (or the OAuth equivalent) will redirect the user off-site after authenticating. **Recommendation:** reject non-relative targets (must start with a single `/`, not `//` or a scheme) before redirecting.

**H2. In-memory session store: volatility + unbounded growth.**
`sessions/sessions.go` keeps all sessions in a `map` guarded by a mutex. Consequences:
- Every login (admin and GitHub, including stored `AccessToken`s) is **lost on restart** and **not shared across instances** — fine for a single Fly machine, a problem the moment it scales out.
- **Expired sessions are only evicted lazily on access** (`Get` deletes on read, `sessions.go:104-107`). Tokens for users who never return stay resident indefinitely — a slow memory leak and a larger window of plaintext tokens in memory. There is no reaper goroutine.

**Recommendation:** add a background sweep (ticker) to evict expired sessions; longer-term, back sessions with a persistent store (the repo already pulls in the MongoDB v2 driver as an indirect dep).

**H3. No CI quality gate.**
The only workflow, `.github/workflows/fly-deploy.yml`, runs `flyctl deploy` on every push to `main` with **no `go build` / `go test` / `go vet` / golangci-lint step**. The thorough `.golangci.yml` is never executed automatically, and broken or failing code deploys to production. **Recommendation:** add a `test`/`lint` job and make the deploy `needs:` it.

**H4. No CSRF token on the admin login form.**
`templates/pages/login.templ` posts username/password/redirect with no anti-CSRF token. The session cookie's `SameSite=Lax` is the only mitigation. Login CSRF is lower-severity than authenticated-action CSRF, but the admin panel has no CSRF protection on any state-changing route should those be added. **Recommendation:** add a per-session CSRF token pattern before the admin panel gains mutating actions.

### 🟡 Medium

**M1. Test-coverage gaps in critical, untested code.**
No tests cover the GitHub API surface or the quux site logic:
- `services/github/api.go` (436 lines), `oauth.go`, `client.go`, `config.go`.
- `sites/com/geoffjay/quux/{service,routes,helpers}.go`.

Roughly 70% of non-template packages have no tests. The OAuth helpers (`ExchangeCode`, `FetchViewer`, `AllowedLogin`) and the diff/PR logic are exactly the kind of code that benefits from unit tests against `httptest` servers. **Recommendation:** prioritize `services/github` (it's pure-ish and easy to fake) and the quux service.

**M2. Missing HTTP security headers.**
No `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, or `Strict-Transport-Security` are set anywhere. **Recommendation:** a single global middleware can set sane defaults for all routes; templ already auto-escapes, so CSP is the main XSS-defense gap.

**M3. No rate limiting on `/login`.**
Neither admin login (`admin.go`) nor the OAuth start endpoint is throttled. Low impact while credentials are hardcoded, but it should land alongside the real credential store (C1).

**M4. Mixed logging libraries.**
Core paths use `log/slog`, but stdlib `log` is used in three proxy files — `middleware/proxy.go`, `sites/tld/domain2/proxy.go`, `sites/com/geoffjay/quux/proxy.go` — including a debug `log.Println("host:", host)` in `middleware/proxy.go`. **Recommendation:** standardize on `slog` and drop/guard the debug line.

**M5. `Secure: true` hardcoded on all cookies.**
The session cookie (`sessions/sessions.go:130`) and the OAuth state cookie (`services/github/provider.go:106-114`) always set `Secure: true`. This is correct for production but means cookies are silently dropped over plain HTTP in local dev. **Recommendation:** derive `Secure` from environment (e.g. `JUGHEAD_ENV` / request scheme).

**M6. Site configuration is hardcoded.**
`sites/sites.go:43-72` defines the site map as a literal with `// TODO: Read from a configuration file` at line 43. Adding or changing a site requires a code change and redeploy. **Recommendation:** deliver the existing TODO with a file- or env-driven loader.

### 🟢 Low

- **L1. Non-constant-time OAuth state comparison** — `validStateCookie` uses `cookie == queryState` (`services/github/provider.go:212`). Low risk for a single-use nonce, but `subtle.ConstantTimeCompare` is a trivial hardening.
- **L2. Swallowed errors** — `_ = c.AbortWithError(...)` discards errors in `handlers.go:83-98`.
- **L3. Unchecked context value usage** — `github_token` is read from the gin context relying on a middleware guarantee rather than a checked type assertion (`services/github/provider.go`).
- **L4. Placeholder site rendering** — `siteViewHandler` renders hardcoded meta/body with an inline TODO describing the intended per-site lookup (`handlers.go:43-75`).
- **L5. Trusts `X-Forwarded-Host` for routing** (`middleware/proxy.go`). Acceptable behind a trusted proxy (and convenient for DNS-free testing), but a routing/spoofing risk if the app is ever exposed directly. Document the trust assumption.

---

## 5. Maintainability / Code-Health Opportunities

- **Duplicated DaisyUI component library.** The 66-file DaisyUI `.templ` component set exists in full in **both** `templates/components/daisyui/` and `sites/tld/domain1/components/` (identical filenames, verified). This doubles maintenance and invites drift. **Recommendation:** consolidate to one shared package and import it from the example site.
- **Go version mismatch.** `go.mod` declares `go 1.25.0`; `Dockerfile` builds with `golang:1.26-alpine`. Align them to avoid "works locally, differs in CI/prod" surprises.
- **Scaffolding vs. product.** Substantial placeholder content remains — `domain1`/`domain2`, the `/` index ("You're here because it worked out", `handlers.go:18-40`), and demo themes. Worth an explicit decision on what is demo vs. shippable, ideally gated behind dev-only loading.
- **No project documentation.** There is no `README.md`, `ARCHITECTURE.md`, or `CONTRIBUTING.md`. The dual-access-mode + reverse-proxy design is non-obvious and would benefit from a short written overview and a local-dev quickstart (Procfile uses `air`, `bun`, and `templ generate -watch`).
- **Observability.** No metrics, no request/trace IDs, and no audit logging for admin actions. A request-ID middleware and basic counters would pay off as soon as this runs anywhere real.

---

## 6. Opportunities (forward-looking)

- **Persistent, DB-backed accounts + sessions.** Resolves C1, H2, and M3 together. The MongoDB v2 driver is already an (indirect) dependency and the code comments anticipate a "database-backed accounts table."
- **File-based / env-driven site configuration.** Delivers the `sites/sites.go:43` TODO and removes the redeploy-to-add-a-site friction.
- **Flesh out the admin panel.** `/admin/{sites,users,settings,logs}` currently render static placeholder content (`admin.go:63-81`); they're scaffolding for real management UIs.
- **CI workflow as a merge gate.** A `test + vet + lint` job that the Fly deploy `needs:` — the single highest-leverage process change.

---

## 7. Quick-Wins

Cheapest high-value changes, roughly ordered by impact/effort:

| Fix | Addresses | Effort | Impact |
| --- | --- | --- | --- |
| Add CI `test + vet + lint` job gating the Fly deploy | H3 | Low | High |
| Validate `redirect` is a same-origin relative path | H1 | Low | High |
| Security-headers middleware (CSP, X-Frame-Options, nosniff, HSTS) | M2 | Low | High |
| Background reaper for expired sessions | H2 (leak) | Low | Medium |
| Align Go version (go.mod ↔ Dockerfile) | §5 | Trivial | Medium |
| Standardize on `slog`; remove debug `log.Println` | M4 | Low | Medium |
| Dev-aware `Secure` cookie flag | M5 | Low | Medium (dev UX) |
| Move admin credentials to env + bcrypt + constant-time compare | C1 | Medium | High |
| Consolidate duplicated DaisyUI component library | §5 | Medium | Medium |

---

_Evidence sources (read-only): `server.go`, `admin.go`, `handlers.go`, `main.go`, `sessions/sessions.go`, `middleware/{auth,proxy}.go`, `services/github/{config,oauth,provider,api,client}.go`, `sites/{sites,manager,env}.go`, `sites/auth/auth.go`, `.golangci.yml`, `.github/workflows/fly-deploy.yml`, `Dockerfile`, `go.mod`._
