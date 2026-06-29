// Package layouts establishes the site-type layout concept. A layout is a
// reusable, configurable page shell for a category of site — documentation,
// blog, admin, SaaS landing, etc. Each concrete layout lives in its own
// subpackage (e.g. templates/layouts/documentation) and exports a Config type
// plus a Layout function returning a full-page templ.Component.
//
// This parent package holds only the shared vocabulary types used across
// layouts, so layouts can be composed and referenced uniformly without
// depending on each other.
package layouts

import "github.com/a-h/templ"

// Meta holds page-level metadata shared by every site-type layout. Title is
// rendered as the <title> and brand text; Description and Keywords feed the
// page's meta tags; Theme selects the daisyUI theme applied to <html>.
type Meta struct {
	Title       string
	Description string
	Keywords    string
	Theme       string
}

// NavItem describes a single navigation link rendered in a layout's navbar or
// sidebar. Active highlights the current page so layouts can reflect the
// request's location. Icon, when set to a non-empty phosphor icon name (e.g.
// "House", "BookOpen"), is resolved by the rendering layout into a phosphor
// icon component and shown before the label — this keeps collapsed sidebars
// usable when the label is hidden. An unknown name is silently ignored.
type NavItem struct {
	Label  string
	Href   string
	Active bool
	Icon   string
}

// NavSection is a titled group of NavItems. Layouts render sections as
// collapsible groups in the sidebar (e.g. "Getting started", "Components").
type NavSection struct {
	Title string
	Items []NavItem
}

// Layout is the contract implemented by each site-type layout. A layout
// renders a full HTML page from its own package-specific Config, which
// satisfies this interface via a Render method.
//
// Sites may hold a Layout value to render pages without knowing the concrete
// layout package, enabling layout swaps (e.g. documentation → blog) at the
// site-config level.
type Layout interface {
	Render() templ.Component
}
