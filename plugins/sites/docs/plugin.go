// Package main is a jughead plugin shipping the docs.jughead.geoffjay.com
// documentation site.
//
// It is the Phase 4 pilot for site plugins: it validates that a site's
// templ.Component (the Site.Template), its reverse-proxy fallback (Site.Proxy),
// and its route registrar (Site.Routes, which receives a *gin.RouterGroup) all
// cross the .so boundary correctly and render through the host's TemplRender
// wrapper. This is the harder case flagged in the plugin plan: unlike a
// provider (which only exchanges narrow interfaces), a site shares the full
// templ runtime, the gin router group, and the shared templates/layouts surface
// with the host.
//
// The plugin vendors the docs site source (sites/com/geoffjay/jughead/docs) as
// a package main and imports the shared templates/ and sites/links packages
// from the jughead module — this is fine because the plugin is built against
// the same module, which is how the type-identity requirement is satisfied.
//
// Build from the repo root:
//
//	task plugin NAME=sites/docs
//
// When JUGHEAD_PLUGINS_DIR points at the built docs.so, the loader registers
// the site under Path "/sites/docs.jughead.geoffjay.com", supplanting the
// in-repo built-in docs site (both register under the same Path; the plugin
// loads before sites.Initialize() so the plugin's registration wins, then the
// in-repo Initialize() overwrites it — see the note in sites.Initialize). To
// run the docs site purely from the plugin, remove or comment out the in-repo
// entry in sites/sites.go.
package main

import (
	"github.com/geoffjay/jughead/sdk"
)

// Plugin is the well-known symbol the host loader looks up. It ships a single
// site (no providers). The site's Template, Proxy, and Routes are typed as any
// on sdk.Site; the host loader type-asserts them against templ.Component,
// func(*gin.Context), and func(*gin.RouterGroup, string) respectively.
//
// Routes is non-nil, so the host's SiteManager calls it to register the docs
// page handlers onto the site's route group (the same wiring the in-repo site
// uses). Template (IntroPlaceholder) is the fallback rendered by
// siteViewHandler when Routes is nil or auth is incomplete; it's carried here
// for parity but Routes takes precedence.
var Plugin = sdk.Plugin{
	Name: "docs",
	Sites: []sdk.Site{
		{
			Path:      sitePath,
			Url:       "https://docs.jughead.geoffjay.com",
			Published: true,
			Theme:     LightTheme,
			Template:  IntroPlaceholder(),
			Proxy:     Proxy,
			Routes:    Routes,
		},
	},
}
