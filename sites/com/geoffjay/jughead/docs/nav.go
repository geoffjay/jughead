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
// matching activeSlug as Active. Each item's Icon is a phosphor icon name
// resolved by the documentation layout into the icon shown in the sidebar; it
// is especially important for collapsed sidebar state, where the label is
// hidden and only the icon identifies the entry. Icons are sourced from
// pageIcon, which maps each page slug to a phosphor icon name.
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
			Icon:   pageIcon(p.Slug),
		})
	}
	return items
}

// pageIcon maps a documentation page slug to the phosphor icon name shown
// beside it in the sidebar. Add an entry here when adding a page to
// pageRegistry; pages without an entry render label-only (no icon), which is
// fine in the open sidebar but leaves the collapsed sidebar without a glyph
// for that entry.
func pageIcon(slug string) string {
	switch slug {
	case "/":
		return "House"
	case "/install":
		return "Download"
	case "/architecture":
		return "CompassTool"
	case "/routes":
		return "Path"
	case "/plugins":
		return "PlugsConnected"
	}
	return ""
}
