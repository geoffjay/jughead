// Package main is a jughead plugin shipping the Google OAuth auth provider.
//
// Go `plugin` .so files must be built from a main package; this file and its
// siblings form that package. Built with `go build -buildmode=plugin` (see the
// Taskfile `plugin` target) and loaded at startup by the host's plugin loader.
// The plugin exports a package-level `Plugin` variable of type *sdk.Plugin
// whose Providers slice contains the Google provider. When loaded, the provider
// registers under "google" in the host's auth registry, supplanting any
// built-in fallback (the host skips its built-in registration when a plugin has
// already registered the same name).
//
// Build (from the repo root):
//
//	task plugin NAME=providers/google
//
// The resulting plugins/google.so is discovered when JUGHEAD_PLUGINS_DIR
// points at the plugins directory.
package main

import (
	"github.com/geoffjay/jughead/sdk"
	sdkauth "github.com/geoffjay/jughead/sdk/auth"
)

// Plugin is the well-known symbol the host loader looks up via plugin.Lookup.
// It ships only a provider (no sites): the Google auth provider, registered
// under Provider.Name() == "google".
//
// plugin.Lookup returns a *T pointing at the symbol, so this var must be of
// value type sdk.Plugin (not a pointer) for the loader's *sdk.Plugin type
// assertion to succeed. The host loader dereferences the pointer it receives.
var Plugin = sdk.Plugin{
	Name: "google",
	Providers: []sdkauth.Provider{
		NewProvider(),
	},
}
