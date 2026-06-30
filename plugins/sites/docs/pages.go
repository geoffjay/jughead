package main

import "github.com/a-h/templ"

// PageBuilder renders the body content for a documentation page. It is the
// page-specific templ component returned by page factories in pages.go.
type PageBuilder = templ.Component

// pageDef describes a documentation page registered on the site. Slug is the
// site-relative URL path. Title is the page title. Section is the sidebar
// group the page belongs to. Body builds the page content injected into the
// documentation layout's main region.
type pageDef struct {
	Slug    string
	Title   string
	Section string
	Body    func() templ.Component
}

// Section names used in the page registry and the sidebar navigation. Shared
// as constants so the nav builder and page registry reference the same label.
const (
	SectionGettingStarted = "Getting started"
	SectionGuides         = "Guides"
	SectionReference      = "Reference"
)

// pageRegistry is the set of documentation pages rendered by the site. Order
// within a section is preserved in the sidebar.
var pageRegistry = []pageDef{
	{Slug: "/", Title: "Introduction", Section: SectionGettingStarted, Body: introPage},
	{Slug: "/install", Title: "Installation", Section: SectionGettingStarted, Body: installPage},
	{Slug: "/architecture", Title: "Architecture", Section: SectionGuides, Body: architecturePage},
	{Slug: "/routes", Title: "Site routes", Section: SectionReference, Body: routesPage},
	{Slug: "/plugins", Title: "Plugins", Section: SectionReference, Body: pluginsPage},
}

// Pages returns the page definitions in registry order. The docs.nav and
// docs.routes packages use this to build the sidebar and wire URL handlers.
func Pages() []pageDef {
	return pageRegistry
}

// pageBySlug returns the page definition for the given site-relative slug, or
// ok=false when no page matches.
func pageBySlug(slug string) (pageDef, bool) {
	for _, p := range pageRegistry {
		if p.Slug == slug {
			return p, true
		}
	}
	return pageDef{}, false
}
