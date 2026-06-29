package docs

import "github.com/geoffjay/jughead/templates/layouts"

// navSections returns the documentation sidebar navigation sections for the
// current render, marking the page with the given slug as Active so the
// sidebar highlights it. Hrefs are site-relative; the layout resolves them via
// the per-request LinkResolver.
//
// Sections mirror the page registry in pages.go so adding a page there
// automatically extends the sidebar.
func navSections(activeSlug string) []layouts.NavSection {
	return []layouts.NavSection{
		{Title: SectionGettingStarted, Items: sectionItems(SectionGettingStarted, activeSlug)},
		{Title: SectionGuides, Items: sectionItems(SectionGuides, activeSlug)},
		{Title: SectionReference, Items: sectionItems(SectionReference, activeSlug)},
	}
}

// sectionItems returns the nav items for a named section, marking the item
// matching activeSlug as Active.
func sectionItems(section, activeSlug string) []layouts.NavItem {
	var items []layouts.NavItem
	for _, p := range Pages() {
		if p.Section != section {
			continue
		}
		items = append(items, layouts.NavItem{
			Label:  p.Title,
			Href:   p.Slug,
			Active: p.Slug == activeSlug,
		})
	}
	return items
}
