// Package doc renders a full-page documentation site layout. It is the first
// site-type layout in templates/layouts and is intended to be configured and
// extended for each documentation site (e.g. docs.jughead.geoffjay.com).
//
// The directory is named "documentation" to match the site-type concept; the
// package is named "doc" because Go's build tool excludes packages literally
// named "documentation" (a reserved name for the go doc tool's own output).
//
// The layout is composed of:
//   - a fixed top navbar with brand, primary nav, and an action region,
//   - a left sidebar with hierarchical navigation sections,
//   - a main content region rendering the page body,
//   - an optional right "on this page" table-of-contents region, and
//   - a footer.
//
// Sidebar and TOC visibility adapt to screen size: on small screens the
// sidebar collapses into a drawer (toggled from the navbar menu icon) and the
// TOC is hidden.
//
// The layout is configured entirely through Config, so sites can customize
// branding, navigation, theme, and content without forking the template. A
// site may also extend the layout by composing its own body components inside
// Content and passing them via Config.Content.
package doc

import (
	"github.com/geoffjay/jughead/templates/components/daisyui"
	"github.com/geoffjay/jughead/templates/layouts"

	"github.com/a-h/templ"
)

// Config configures the documentation layout for a single page render.
//
// Meta carries page-level metadata (title, description, keywords, theme).
// Brand is the navbar brand text and link. PrimaryNav is rendered as
// navbar-end menu items (e.g. "Docs", "API", "GitHub"). Actions, when set,
// is rendered in the navbar-end region before PrimaryNav (e.g. a theme toggle
// or version selector). Sections form the sidebar navigation. Content is the
// page body rendered in the main region. TOC, when set, renders the right-hand
// "on this page" region. Footer, when set, replaces the default footer.
// Resolver, when set, prefixes all site-relative hrefs with the site path so
// the layout works under both the site-path and FQDN access modes.
type Config struct {
	Meta       layouts.Meta
	Brand      layouts.NavItem
	PrimaryNav []layouts.NavItem
	Actions    templ.Component
	Sections   []layouts.NavSection
	Content    templ.Component
	TOC        templ.Component
	Footer     templ.Component
	Resolver   daisyui.URLResolver
}

// Layout renders a full HTML documentation page from cfg. It wraps the
// documentation chrome (navbar + sidebar + content + toc + footer) in the
// shared templates.Layout head/body shell so every docs page shares the same
// <head>, asset links, and CSP with the rest of the app.
//
// cfg is taken by pointer because Config is a large struct (it embeds
// templ.Component fields and navigation slices); passing it by value would
// copy ~232 bytes per page render.
func Layout(cfg *Config) templ.Component {
	return docsPage(cfg)
}
