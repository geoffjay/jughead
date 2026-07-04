// Package main is a minimal example jughead plugin showing the contract a
// plugin .so exports. Copy this directory as a starting point for a new
// provider plugin.
//
// A plugin is a main package built with `go build -buildmode=plugin`. It
// exports a package-level var named `Plugin` of type sdk.Plugin. The host
// loader (plugin.LoadAll) opens the .so, looks up "Plugin", and registers
// the plugin's Providers into the auth registry and its Sites into the
// SiteManager.
//
// Build from the repo root:
//
//	task plugin NAME=providers/example
//
// Then set JUGHEAD_PLUGINS_DIR to the directory containing example.so and
// start jughead. The provider registers under Name() == "example"; a site
// can reference it via Auth.Provider: "example".
package main

import (
	"errors"
	"net/http"

	"github.com/geoffjay/jughead/sdk"
	sdkauth "github.com/geoffjay/jughead/sdk/auth"
	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

// Plugin is the well-known symbol the host loader looks up. This example
// ships a single trivial provider that demonstrates the contract; replace its
// Load/instance logic with your real auth flow.
var Plugin = sdk.Plugin{
	Name: "example",
	Providers: []sdkauth.Provider{
		exampleProvider{},
	},
}

// exampleProvider implements sdkauth.Provider. Name() is the registry key a
// site references via AuthConfig.Provider. Load returns a configured
// instance or an error (e.g. missing env config); the SiteManager treats an
// error as "auth disabled for this site" and falls back to its default view.
type exampleProvider struct{}

func (exampleProvider) Name() string { return "example" }

// Load here always succeeds; a real provider reads its credentials from the
// environment and returns an error when they're incomplete.
func (exampleProvider) Load(cfg sdkauth.AuthConfig) (sdkauth.ProviderInstance, error) {
	if cfg.Provider == "" {
		return nil, errors.New("example: empty provider name")
	}
	return &exampleInstance{}, nil
}

// exampleInstance implements sdkauth.ProviderInstance. RegisterAuthRoutes
// mounts the OAuth web-flow routes; AuthMiddleware protects a site's group.
type exampleInstance struct{}

func (e *exampleInstance) RegisterAuthRoutes(router gin.IRouter, _ *sessions.Store) {
	router.GET("/auth/example/login", func(c *gin.Context) {
		c.String(http.StatusOK, "example provider login (not implemented)")
	})
}

func (e *exampleInstance) AuthMiddleware(_ *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// A real middleware checks the session and redirects to the login
		// start endpoint when unauthenticated. This example always allows.
		c.Next()
	}
}
