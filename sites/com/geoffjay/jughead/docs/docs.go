// Package docs is the docs.jughead.geoffjay.com documentation site. It uses
// the templates/layouts/documentation layout to render a branded documentation
// experience with a sidebar of navigation sections and a main content region.
//
// The site is registered under the path "/sites/docs.jughead.geoffjay.com"
// and is reachable via the FQDN reverse proxy at https://docs.jughead.geoffjay.com.
//
// Routes wires the docs pages onto the site group; Proxy is the reverse-proxy
// fallback to the FQDN. Navigation sections and content pages live in nav.go
// and pages/*.templ respectively.
package docs

import (
	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/templates/layouts"
	doc "github.com/geoffjay/jughead/templates/layouts/documentation"

	"github.com/a-h/templ"
)

// sitePath is the configured path under which the docs site is served.
const sitePath = "/sites/docs.jughead.geoffjay.com"

// Theme names and the localStorage key used by the documentation layout's
// theme selector. The server renders pages with LightTheme by default; the
// navbar swap toggle overrides to DarkTheme and persists the choice across
// reloads. The storage key is namespaced per site so it doesn't collide with
// other sites on the same origin.
const (
	LightTheme   = "nord"
	DarkTheme    = "abyss"
	ThemeStorage = "docs.jughead.theme"
)

// Page describes a single documentation page rendered by the site. Slug is
// the site-relative URL path (e.g. "/intro"). Title is the page title and the
// label shown in the sidebar. Section is the navigation group the page
// belongs to. Body renders the page content injected into the layout's main
// region.
type Page struct {
	Slug    string
	Title   string
	Section string
	Body    templ.Component
}

// pageConfig builds the documentation.Config for a page render, deriving the
// sidebar sections from the page registry, marking the active page, and
// resolving site-relative links via the per-request LinkResolver.
func pageConfig(r links.LinkResolver, page Page) doc.Config {
	sections := navSections(page.Slug)
	return doc.Config{
		Meta: layouts.Meta{
			Title:       page.Title + " - jughead docs",
			Description: "jughead documentation: " + page.Title,
			Keywords:    "jughead, docs, documentation, go, templ, htmx, daisyui",
			Theme:       LightTheme,
		},
		Brand: layouts.NavItem{
			Label: "jughead",
			Href:  "",
		},
		PrimaryNav: primaryNav(),
		Sections:   sections,
		Content:    page.Body,
		Theme: doc.ThemeConfig{
			LightTheme: LightTheme,
			DarkTheme:  DarkTheme,
			StorageKey: ThemeStorage,
		},
		Resolver: r,
	}
}

// primaryNav returns the navbar-end links shown on every docs page. Links are
// site-relative where applicable; the documentation layout resolves them via
// the per-request LinkResolver.
func primaryNav() []layouts.NavItem {
	return []layouts.NavItem{
		{Label: "Docs", Href: "/"},
		{Label: "GitHub", Href: "https://github.com/geoffjay/jughead"},
	}
}
