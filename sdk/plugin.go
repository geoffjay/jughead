// Package sdk is the versioned contract that jughead plugins are built against.
//
// A plugin is a Go shared object (.so) built with `go build -buildmode=plugin`
// against this package. It exports a package-level variable named `Plugin` of
// type *sdk.Plugin, which the host loader looks up via plugin.Lookup at
// startup and uses to register the plugin's providers and sites.
//
// The host and plugin MUST be built with the same Go toolchain version and the
// same versions of every shared dependency (gin, templ, sessions, this
// package). Because the `plugin` package requires byte-identical type
// identities across the .so boundary, the canonical contract types live here
// and are re-exported by the host's internal packages (e.g. sites/auth) so
// in-repo code and plugins share one definition of each type.
//
// Bump the SDK's conceptual version (documented in the release notes) on any
// breaking change to the types in this package and its subpackages.
package sdk

import (
	"github.com/geoffjay/jughead/sdk/auth"
)

// Plugin is the well-known symbol a plugin .so exports. Plugin authors declare
// a package-level variable:
//
//	var Plugin = sdk.Plugin{
//	    Name: "github",
//	    Providers: []auth.Provider{ github.NewProvider() },
//	}
//
// The host loader looks up the symbol "Plugin" (pointer to this value). Name
// is informational only; providers identify themselves via Provider.Name() and
// sites via Site.Path.
type Plugin struct {
	// Name is a human-readable identifier for the plugin (used in logs). It is
	// not used for routing or registry keys.
	Name string

	// Providers are auth providers the plugin contributes to the host's auth
	// registry. Each is registered via auth.Register under its Provider.Name().
	// Registering a name that already exists replaces the prior provider, so a
	// plugin can intentionally override a built-in.
	Providers []auth.Provider

	// Sites are sites the plugin contributes to the host's SiteManager. Each
	// is registered under its Site.Path. A plugin site may reference a
	// provider shipped by the same plugin (the provider is registered first).
	Sites []Site
}

// Site is the plugin-facing site descriptor. It mirrors the host's internal
// sites.Site but is the type plugins construct, so the host and plugin share
// the same type instance (required by the `plugin` package: a type defined in
// two different packages is not assignable across the .so boundary).
//
// Template, Proxy, and Routes are typed as any so the SDK does not need to
// re-export templ/gin types at the top level; the host loader type-asserts
// them against the host's sites.Site field types (templ.Component,
// func(*gin.Context), func(*gin.RouterGroup, string)) when registering.
//
// Fields mirror sites.Site; see that package for field semantics.
type Site struct {
	Path      string
	Url       string
	Published bool
	Theme     string
	Template  any // host asserts templ.Component
	Proxy     any // host asserts func(*gin.Context)
	Routes    any // host asserts func(*gin.RouterGroup, string)
	Auth      *auth.AuthConfig
}
