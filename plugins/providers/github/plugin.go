// Package main is a jughead plugin shipping the GitHub OAuth auth provider.
//
// Go `plugin` .so files must be built from a main package; this file and its
// siblings in this directory form that main package. The package is built with
// `go build -buildmode=plugin` (see the Taskfile `plugin` target) and loaded
// at startup by the host's plugin loader. The plugin exports a package-level
// `Plugin` variable of type *sdk.Plugin whose Providers slice contains the
// GitHub provider. When loaded, the provider registers under the name
// "github" in the host's auth registry, supplanting the built-in fallback
// (the host skips its built-in registration when a plugin has already
// registered the same name).
//
// This plugin vendors a trimmed copy of the GitHub OAuth logic
// (services/github) — just the authorization-code web flow and the /user
// profile fetch — because the full services/github package also contains the
// quux review UI's GitHub API client, which is site-specific and not needed
// here. Keeping the plugin self-contained avoids pulling the quux data layer
// into every deployment that wants GitHub auth.
//
// Build (from the repo root):
//
//	task plugin NAME=providers/github
//
// The resulting plugins/github.so is discovered when JUGHEAD_PLUGINS_DIR
// points at the plugins directory.
package main

import (
	"github.com/geoffjay/jughead/sdk"
	sdkauth "github.com/geoffjay/jughead/sdk/auth"
)

// Plugin is the well-known symbol the host loader looks up via plugin.Lookup.
// It ships only a provider (no sites): the GitHub auth provider, registered
// under Provider.Name() == "github".
//
// plugin.Lookup returns a *T pointing at the symbol, so this var must be of
// value type sdk.Plugin (not a pointer) for the loader's *sdk.Plugin type
// assertion to succeed. The host loader dereferences the pointer it receives.
var Plugin = sdk.Plugin{
	Name: "github",
	Providers: []sdkauth.Provider{
		NewProvider(),
	},
}
