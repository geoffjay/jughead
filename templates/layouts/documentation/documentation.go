// Package doc renders a full-page documentation site layout. It is the first
// site-type layout in templates/layouts and is intended to be configured and
// extended for each documentation site (e.g. docs.jughead.geoffjay.com).
//
// The layout is a thin wrapper around the shared containers.AppShell — the
// application shell provides the fixed navbar, the collapsible left sidebar,
// the scrolling main content region, and the sidebar-state persistence. This
// package contributes the documentation-specific configuration: a sidebar
// grouped by navigation sections, a light/dark theme selector wired into the
// navbar, and the shared <html>/<head>/<body> shell from templates.Layout.
//
// The directory is named "documentation" to match the site-type concept; the
// package is named "doc" because Go's build tool excludes packages literally
// named "documentation" (a reserved name for the go doc tool's own output).
//
// The layout is configured entirely through Config, so sites can customize
// branding, navigation, theme, and content without forking the template. A
// site may also extend the layout by composing its own body components inside
// Content and passing them via Config.Content.
package doc

import (
	"github.com/geoffjay/jughead/templates/components/daisyui"
	"github.com/geoffjay/jughead/templates/containers"
	"github.com/geoffjay/jughead/templates/layouts"

	"github.com/a-h/templ"
)

// Config configures the documentation layout for a single page render.
//
// Meta carries page-level metadata (title, description, keywords, theme).
// Brand is the navbar brand text and link. PrimaryNav is rendered as
// navbar-end menu items (e.g. "Docs", "API", "GitHub"). Actions, when set,
// is rendered in the navbar-end region before the theme toggle and PrimaryNav
// (e.g. a version selector). Sections form the sidebar navigation, rendered
// as titled groups. Content is the page body rendered in the main region.
// TOC, when set, renders a right-hand "on this page" region inside Content
// (the layout does not reserve a separate column; the caller composes it).
// Footer, when set, is rendered inside Content (the AppShell main region
// scrolls, so a footer belongs to the content rather than the shell).
// Theme, when set with both LightTheme and DarkTheme, renders a sun/moon swap
// toggle in the navbar and persists the choice to localStorage. Resolver, when
// set, prefixes all site-relative hrefs with the site path so the layout works
// under both the site-path and FQDN access modes.
type Config struct {
	Meta       layouts.Meta
	Brand      layouts.NavItem
	PrimaryNav []layouts.NavItem
	Actions    templ.Component
	Sections   []layouts.NavSection
	Content    templ.Component
	TOC        templ.Component
	Footer     templ.Component
	Theme      ThemeConfig
	Resolver   daisyui.URLResolver
}

// ThemeConfig configures the documentation layout's theme selector. It maps
// directly onto containers.ThemeConfig. LightTheme and DarkTheme are the
// daisyUI theme names switched between by the navbar toggle (e.g.
// "kanagawa-light" / "kanagawa-dark"). StorageKey is the localStorage key used
// to persist the user's choice across reloads; it should be unique per site
// (e.g. "docs.jughead.theme"). When both LightTheme and DarkTheme are empty
// the theme selector is omitted.
type ThemeConfig = containers.ThemeConfig

// Layout renders a full HTML documentation page from cfg. It wraps the
// documentation configuration around the shared containers.AppShell and then
// wraps the result in templates.Layout so every docs page shares the same
// <head>, asset links, and CSP with the rest of the app.
//
// cfg is taken by pointer because Config is a large struct (it embeds
// templ.Component fields and navigation slices); passing it by value would
// copy heavily per page render.
func Layout(cfg *Config) templ.Component {
	return docsPage(cfg)
}

// toAppShellConfig maps the documentation Config onto the AppShellConfig
// consumed by containers.AppShell. The documentation layout owns the
// vocabulary for nav sections (layouts.NavSection, which carries Active state
// for sidebar highlighting); the AppShell renders them via containers.NavSection
// wrapping daisyui.MenuItem. All hrefs are pre-resolved here (via resolveNavItem,
// which preserves absolute URLs and prefixes site-relative ones), so the
// AppShell's Resolver is set to nil to avoid a second resolution pass that
// would double-prefix site-relative links and corrupt absolute ones.
func toAppShellConfig(cfg *Config) containers.AppShellConfig {
	shell := containers.AppShellConfig{
		Title:       cfg.Brand.Label,
		TitleHref:   resolveNavItem(cfg.Resolver, cfg.Brand.Href),
		NavItems:    toDaisyNavItems(cfg.Resolver, cfg.PrimaryNav),
		NavSections: toAppShellSections(cfg.Resolver, cfg.Sections),
		Content:     docsMain(cfg),
		Theme:       cfg.Theme,
		Resolver:    nil,
	}
	if cfg.Actions != nil {
		shell.HeaderExtra = cfg.Actions
	}
	return shell
}

// toAppShellSections converts the layout-agnostic []layouts.NavSection into
// the []containers.NavSection expected by AppShell, marking the active item
// per section. Hrefs are resolved via the resolver with absolute URLs passed
// through unchanged.
func toAppShellSections(r daisyui.URLResolver, sections []layouts.NavSection) []containers.NavSection {
	out := make([]containers.NavSection, len(sections))
	for i, section := range sections {
		items := make([]daisyui.MenuItem, len(section.Items))
		for j, item := range section.Items {
			items[j] = daisyui.MenuItem{
				Label:  item.Label,
				Href:   resolveNavItem(r, item.Href),
				Active: item.Active,
			}
		}
		out[i] = containers.NavSection{Title: section.Title, Items: items}
	}
	return out
}
