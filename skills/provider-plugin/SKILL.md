---
name: provider-plugin
description: Develop jughead auth-provider plugins (.so) that ship OAuth/credential flows loaded at runtime by the host. Use when building a new provider, scaffolding with `jughead init --plugin provider`, implementing sdk/auth.Provider/ProviderInstance, or wiring AuthConfig onto a site.
---

# jughead auth-provider plugins

A jughead **auth-provider plugin** is a Go shared object (`.so`) built against
`github.com/geoffjay/jughead/sdk` and `github.com/geoffjay/jughead/sdk/auth`.
It contributes one or more providers to the host's auth registry
(`sites/auth`). Sites reference a provider by name via `Site.Auth.Provider`.

Use this skill whenever you are:

- Scaffolding a new provider plugin (`jughead init --plugin provider`)
- Implementing `sdkauth.Provider` (`Name`, `Load`) and `ProviderInstance`
- Wiring OAuth web-flow routes and session middleware
- Reading credentials from the environment
- Debugging "auth disabled for this site" fallbacks

## Hard constraint: type identity across the .so boundary

The host and every plugin must be built with the **same Go toolchain** and
**byte-identical dependency versions**. The canonical contract types
(`Provider`, `ProviderInstance`, `AuthConfig`) live in `sdk/auth` so the host
and plugins share one definition of each type — a type defined in two packages
is not assignable across the `.so` boundary. Build via the Taskfile:

```
task plugin NAME=providers/<name>
task build:plugins
```

## Scaffold

```
jughead init --plugin provider --name bar path/to/bar
```

Renders `templates/init/provider/*.tmpl` into the target, producing a
package main with a placeholder `Provider`, `Instance`, `Load`, routes, and
middleware. Replace the placeholder logic with your real auth flow.

## The contract

```go
import (
    "github.com/geoffjay/jughead/sdk"
    sdkauth "github.com/geoffjay/jughead/sdk/auth"
)

var Plugin = sdk.Plugin{
    Name: "bar",
    Providers: []sdkauth.Provider{
        BarProvider{},
    },
}

type BarProvider struct{}

func (BarProvider) Name() string { return "bar" }

func (BarProvider) Load(cfg sdkauth.AuthConfig) (sdkauth.ProviderInstance, error) {
    // Read credentials from the environment; return an error when incomplete.
    return &BarInstance{...}, nil
}
```

`sdkauth.Provider` (registry-facing):

| Method  | Purpose |
| ------- | ------- |
| `Name()` | Registry key a site references via `AuthConfig.Provider`. |
| `Load(cfg AuthConfig) (ProviderInstance, error)` | Reads credentials (typically from env), returns a ready instance. A non-nil error means "auth disabled for this site" — the host falls back to the site's default view. |

`sdkauth.ProviderInstance` (configured, per-site):

| Method | Purpose |
| ------ | ------- |
| `RegisterAuthRoutes(router gin.IRouter, store *sessions.Store)` | Mounts the OAuth web-flow routes (`/auth/<provider>/login`, `/callback`, `/logout`). Routes are registered on the **root router**, not the site group, so they're reachable via both localhost and the FQDN reverse proxy. |
| `AuthMiddleware(store *sessions.Store) gin.HandlerFunc` | Protects a site's route group; redirects unauthenticated requests to the login start endpoint. |

`AuthConfig` is declared per-site on `Site.Auth`. Provider-specific credentials
are **not** carried here — each provider reads its own environment variables in
`Load`. The fields let a site override provider defaults:

```go
type AuthConfig struct {
    Provider     string // names the provider, e.g. "github"
    RedirectURL  string // overrides the provider's default OAuth callback path
    AllowedLogin string // when non-empty, restricts access to a single login
}
```

## Load: read credentials, return an instance or an error

`Load` is called once per site that requests the provider. It must:

1. Read credentials from the environment (e.g. `BAR_CLIENT_ID`,
   `BAR_CLIENT_SECRET`). Do **not** put credentials in `AuthConfig`.
2. Honor `cfg.RedirectURL` and `cfg.AllowedLogin` overrides when non-empty;
   fall back to env defaults otherwise.
3. Return a configured `ProviderInstance` on success.
4. Return a non-nil error when configuration is incomplete. The host treats
   this as "auth disabled for this site" and renders the site's default view
   instead of mounting protected routes — so the site still responds.

```go
func (BarProvider) Load(cfg sdkauth.AuthConfig) (sdkauth.ProviderInstance, error) {
    clientID := os.Getenv("BAR_CLIENT_ID")
    clientSecret := os.Getenv("BAR_CLIENT_SECRET")
    if clientID == "" || clientSecret == "" {
        return nil, errors.New("bar: BAR_CLIENT_ID/BAR_CLIENT_SECRET not set")
    }
    redirect := cfg.RedirectURL
    if redirect == "" {
        redirect = os.Getenv("BAR_REDIRECT_URL")
    }
    return &BarInstance{clientID: clientID, clientSecret: clientSecret,
        redirect: redirect, allowed: cfg.AllowedLogin}, nil
}
```

## RegisterAuthRoutes: mount the web flow

Routes mount on the root router under `/auth/<provider>/...` so multiple
providers coexist and the routes are reachable via both localhost and the
FQDN proxy. The host dedupes by provider name: if two sites request the same
provider, `RegisterAuthRoutes` is called only once.

A typical OAuth authorization-code flow:

| Route                    | Action |
| ------------------------ | ------ |
| `GET /auth/<p>/login`    | Generate a CSRF state nonce, stash it in a short-lived cookie, redirect to the provider's authorize URL. |
| `GET /auth/<p>/callback` | Validate state, exchange the code for an access token, fetch the viewer profile, enforce `AllowedLogin`, create a session, redirect to the original target. |
| `GET /auth/<p>/logout`   | Destroy the session, redirect to login. |

OAuth CSRF: generate 16 random bytes hex-encoded, store in a `MaxAge: 600`
`SameSite=Lax` `HttpOnly` cookie, and verify it matches the `state` query
parameter in the callback. Clear the cookie after the exchange.

Sessions: use `*sessions.Store` (passed to `RegisterAuthRoutes` and
`AuthMiddleware`). `store.Get(r)` returns the session; check `AccessToken` to
authenticate. `store.CreateWithToken(w, login, token, avatar)` logs a user in;
`store.Destroy(w, r)` logs out.

## AuthMiddleware: protect the site group

The middleware runs on every request to the site's route group. When the
session is valid, set context values the site handlers need (e.g. `username`,
`<provider>_token`) and call `c.Next()`. When not, redirect to the login start
endpoint with a `redirect` query param so the flow can return the user to the
original target:

```go
func (i *BarInstance) AuthMiddleware(store *sessions.Store) gin.HandlerFunc {
    return func(c *gin.Context) {
        sess := store.Get(c.Request)
        if sess != nil && sess.AccessToken != "" {
            c.Set("username", sess.Login)
            c.Next()
            return
        }
        target := browserRedirectTarget(c)
        c.Redirect(http.StatusFound, "/auth/bar/login?redirect="+target)
        c.Abort()
    }
}
```

`browserRedirectTarget` must account for the reverse proxy: when the request
arrived via the proxy (`X-Site-Base` header present), the browser sees
root-relative URLs, so `/` is correct; otherwise use the request's full path.

## Provider precedence and overrides

`auth.Register` replaces any existing provider registered under the same
`Name()`, so a plugin can intentionally override a built-in. Plugins load
**before** built-in providers register, so a plugin provider with the same
name wins and the host skips its built-in registration for that name.

## Wiring a site to use the provider

In the site's `sdk.Site`, set `Auth`:

```go
{
    Path: "/sites/app.example.com",
    // ...
    Auth: &sdkauth.AuthConfig{Provider: "bar"},
}
```

When `Auth` is nil the site is public. When `Auth` is non-nil the host looks up
the provider, calls `Load`, and on success wraps the site group in
`AuthMiddleware` and mounts the provider's OAuth routes once (deduped). On
failure (unknown provider or `Load` error) the site falls back to its default
view so it renders its `Template` (e.g. a sign-in prompt) instead of mounting
protected routes.

## Reference plugins

- `plugins/providers/example/` — minimal starter; the scaffold's target.
- `plugins/providers/github/` — real OAuth provider vendored from
  `services/github`, with the authorization-code flow, `/user` profile fetch,
  `AllowedLogin` restriction, and session creation. Read it before building a
  real provider.