// Package auth holds the canonical auth-provider contract shared by the host
// and plugins.
//
// The interfaces and the AuthConfig type previously lived in
// github.com/geoffjay/jughead/sites/auth. They moved here so that plugins
// (built against the SDK) and the host share the exact same type identities,
// which the Go `plugin` package requires across the .so boundary. The host's
// sites/auth package now re-exports these via type aliases and retains the
// registry (Register/Get), which is host-owned state.
//
// A site declares which provider it wants via Site.Auth; the host's
// SiteManager looks the provider up in the registry at startup and wires its
// routes and middleware onto the router. Concrete providers (e.g. the built-in
// GitHub provider, or a provider shipped as a plugin) implement Provider and
// register themselves via auth.Register during application startup.
package auth

import (
	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

// AuthConfig is declared per-site on Site.Auth. Provider-specific credentials
// are not carried here; each provider reads its own environment variables
// (e.g. GITHUB_CLIENT_ID) in Load. The fields below let a site override
// provider defaults or restrict access.
type AuthConfig struct {
	// Provider names the auth provider to use, e.g. "github". It must match a
	// provider registered via Register.
	Provider string

	// RedirectURL overrides the provider's default OAuth callback path. When
	// empty the provider supplies its own default (e.g. "/auth/github/callback").
	RedirectURL string

	// AllowedLogin, when non-empty, restricts access to a single login. Its
	// meaning is provider-specific (for GitHub it is a GitHub login name).
	AllowedLogin string
}

// ProviderInstance is a configured, ready-to-serve auth provider. A new
// instance is produced by Provider.Load for each site that requests the
// provider; the instance carries the loaded credentials and renders its routes
// and middleware from them.
type ProviderInstance interface {
	// RegisterAuthRoutes wires the provider's OAuth web-flow routes (login,
	// callback, logout) onto the given router. Routes are namespaced under
	// /auth/<provider>/... so multiple providers can coexist.
	RegisterAuthRoutes(router gin.IRouter, store *sessions.Store)

	// AuthMiddleware returns the gin handler that protects a site's route
	// group, redirecting unauthenticated requests to the provider's login
	// start endpoint.
	AuthMiddleware(store *sessions.Store) gin.HandlerFunc
}

// Provider is the registry-facing contract a provider package implements.
type Provider interface {
	// Name is the provider identifier sites reference in AuthConfig.Provider.
	Name() string

	// Load reads the provider's credentials (typically from the environment)
	// and returns a ready-to-serve instance. A non-nil error signals that the
	// configuration is incomplete (e.g. missing client id); the SiteManager
	// treats this as "auth disabled" and falls back to the site's default view.
	Load(cfg AuthConfig) (ProviderInstance, error)
}
