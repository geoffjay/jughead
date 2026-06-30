// Package auth owns the in-process auth-provider registry and re-exports the
// canonical provider contract from github.com/geoffjay/jughead/sdk/auth.
//
// The Provider, ProviderInstance, and AuthConfig types live in the SDK package
// so that plugins (built against the SDK) and the host share identical type
// identities — a hard requirement of the Go `plugin` package across the .so
// boundary. This package re-exports them via type aliases so existing in-repo
// call sites (auth.Register, auth.Get, auth.AuthConfig, etc.) keep working
// without touching their imports.
//
// The registry itself (Register/Get/ResetForTest) is host-owned process state
// and stays here; the SDK deliberately does not own it so plugins register
// through the host's loader rather than mutating a registry they can't see.
package auth

import (
	"sync"

	sdkauth "github.com/geoffjay/jughead/sdk/auth"

	"github.com/gin-gonic/gin"
)

// Canonical contract types re-exported from the SDK. These are type aliases,
// not new types, so a value constructed in a plugin (against sdk/auth) is
// directly assignable to these names in the host.
type (
	// AuthConfig is the per-site auth declaration. See sdk/auth.AuthConfig.
	AuthConfig = sdkauth.AuthConfig

	// ProviderInstance is a configured, ready-to-serve auth provider. See
	// sdk/auth.ProviderInstance.
	ProviderInstance = sdkauth.ProviderInstance

	// Provider is the registry-facing contract a provider implements. See
	// sdk/auth.Provider.
	Provider = sdkauth.Provider
)

var (
	providersMu sync.RWMutex
	providers   = make(map[string]Provider)
)

// Register adds p to the provider registry under p.Name(). Registering a name
// twice replaces the prior provider. It is intended to be called during
// application startup (e.g. from package main or the plugin loader) once per
// provider.
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

// Compile-time guard: the SDK's ProviderInstance interface references
// gin.IRouter and *sessions.Store. These imports keep the host's view of
// those packages linked so the alias types resolve to the same symbols a
// plugin sees.
var (
	_ gin.IRouter     = (*gin.Engine)(nil)
	_ gin.HandlerFunc = func(*gin.Context) {}
)
