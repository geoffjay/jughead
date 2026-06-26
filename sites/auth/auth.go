// Package auth defines the per-site authentication contract and a provider
// registry. A site declares which provider it wants via sites.Site.Auth; the
// SiteManager looks the provider up here at startup and wires its routes and
// middleware onto the router.
//
// Concrete providers (e.g. services/github) implement the Provider interface
// and register themselves with auth.Register during application startup. The
// sites package never imports a provider directly, keeping the dependency
// direction provider -> sites/auth (contract) only.
package auth

import (
	"sync"

	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

// AuthConfig is declared per-site on sites.Site.Auth. Provider-specific
// credentials are not carried here; each provider reads its own environment
// variables (e.g. GITHUB_CLIENT_ID) in Load. The fields below let a site
// override provider defaults or restrict access.
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

var (
	providersMu sync.RWMutex
	providers   = make(map[string]Provider)
)

// Register adds p to the provider registry under p.Name(). Registering a name
// twice replaces the prior provider. It is intended to be called during
// application startup (e.g. from package main) once per provider package.
func Register(p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.Name()] = p
}

// Get returns the registered provider for the given name. The boolean is false
// when no provider has been registered under name.
func Get(name string) (Provider, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}

// ResetForTest clears the registry; only used from tests in downstream
// packages (e.g. sites) so they can start from a clean provider set.
func ResetForTest() {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers = make(map[string]Provider)
}
