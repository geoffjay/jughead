---
name: layouts
description: Use jughead's site-type layout system in templates/layouts to render full pages for a category of site (documentation, blog, admin, SaaS landing). Use when designing a new site, picking a layout, composing a page body into a layout Config, or adding a new site-type layout.
---

# jughead layout system

A **layout** in jughead is a reusable, configurable page shell for a category of
site — documentation, blog, admin, SaaS landing. Each concrete layout lives in
its own subpackage of `templates/layouts/` and exports a `Config` type plus a
`Layout(cfg)` function returning a full-page `templ.Component`.

Use this skill when:

- Designing a new site and deciding which layout to render through
- Composing a page body into a layout's `Config`
- Adding a new site-type layout
- Customizing branding, navigation, theme, or content for a single page render
- Understanding the shared vocabulary types layouts exchange

## Shared vocabulary (templates/layouts/layouts.go)

Every layout exchanges data through these shared types so layouts can be
swapped at the site-config level without callers depending on a concrete
layout package:

```go
// templates/layouts/layouts.go

type Meta struct {
    Title       string // <title> and brand text
    Description string // meta description
    Keywords    string // meta keywords
    Theme       string // daisyUI theme name applied to <html>
}

type NavItem struct {
    Label  string
    Href   string
    Active bool            // highlights the current page
    Icon   string          // phosphor icon name, e.g. "House", "BookOpen"
}

type NavSection struct {
    Title string
    Items []NavItem
}

type Layout interface {
    Render() templ.Component
}
```

`NavItem.Icon` is a **phosphor icon name** (e.g. `"House"`, `"BookOpen"`). The
concrete layout resolves it into a `templ.Component` via the
`github.com/iota-uz/icons/phosphor` registry at render time — callers reference
icons by name without importing the phosphor package. An unknown name is
silently ignored (renders label-only), which is what makes collapsed sidebars
usable when the label is hidden.

## The documentation layout (templates/layouts/documentation)

The first concrete layout. Package is named `doc` (not `documentation`,
which Go's build tool reserves for the `go doc` tool's output) but the
directory is `documentation` to match the site-type concept.

```go
import doc "github.com/geoffjay/jughead/templates/layouts/documentation"

type Config struct {
    Meta       layouts.Meta
    Brand      layouts.NavItem
    PrimaryNav []layouts.NavItem   // navbar-end menu items
    Actions    templ.Component     // navbar-end before nav (e.g. version selector)
    Sections   []layouts.NavSection // sidebar groups
    Content    templ.Component     // page body in the main region
    TOC        templ.Component     // optional right-hand "on this page"
    Footer     templ.Component     // rendered inside Content (the shell scrolls)
    Theme      ThemeConfig          // light/dark toggle
    Resolver   daisyui.URLResolver // prefixes site-relative hrefs
}

func Layout(cfg *Config) templ.Component
```

`Config` is taken **by pointer** because it embeds `templ.Component` fields and
navigation slices — passing by value would copy heavily per page render.

`Layout` is a thin wrapper around the shared `containers.AppShell` (the
application shell: fixed navbar, collapsible left sidebar, scrolling main
region, sidebar-state persistence). The documentation layout owns the
documentation-specific configuration and composes the AppShell; it then wraps
the result in `templates.Layout` so every page shares the same `<head>`, asset
links, and CSP.

### ThemeConfig

```go
type ThemeConfig struct {
    LightTheme string  // daisyUI theme name, e.g. "kanagawa-light"
    DarkTheme  string  // daisyUI theme name, e.g. "kanagawa-dark"
    StorageKey string  // localStorage key, unique per site (e.g. "docs.jughead.theme")
}
```

When both `LightTheme` and `DarkTheme` are empty the theme selector is omitted.
Otherwise a sun/moon swap toggle is rendered in the navbar and the choice is
persisted to `localStorage` under `StorageKey` (or a default key when empty).
Pick a unique `StorageKey` per site to avoid collisions on a shared origin.

### Resolver

`Config.Resolver` (a `daisyui.URLResolver`, satisfied by
`sites.LinkResolver`) prefixes all site-relative hrefs with the site path
when accessed directly, and drops the prefix when accessed via the FQDN
reverse proxy. Always pass the request's `LinkResolver` so links work in both
access modes. The layout pre-resolves all hrefs internally; the AppShell's
own `Resolver` is set to `nil` to avoid a second resolution pass that would
double-prefix site-relative links and corrupt absolute ones.

## Rendering a page through a layout

```go
import (
    "github.com/angelofallars/htmx-go"
    doc "github.com/geoffjay/jughead/templates/layouts/documentation"
    "github.com/geoffjay/jughead/sites/links"
)

func renderPage(c *gin.Context, slug string) {
    p, ok := pageBySlug(slug)
    if !ok {
        c.AbortWithStatus(http.StatusNotFound)
        return
    }
    r := links.NewLinkResolver(c.Request, sitePath)
    cfg := doc.Config{
        Meta:     layouts.Meta{Title: p.Title, Theme: LightTheme},
        Brand:    layouts.NavItem{Label: "jughead", Href: "/"},
        Sections: navSections(r, p.Slug),
        Content:  p.Body(),
        Theme:    doc.ThemeConfig{LightTheme: "kanagawa-light", DarkTheme: "kanagawa-dark", StorageKey: "docs.jughead.theme"},
        Resolver: r,
    }
    siteTemplate := doc.Layout(&cfg)
    if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
        c.AbortWithStatus(http.StatusInternalServerError)
    }
}
```

For HTMX fragment swaps, render the body component directly to `c.Writer` —
no layout wrapper — and let the client swap it into the main region.

## The AppShell container (templates/containers)

`containers.AppShell` is the shared shell the documentation layout wraps. It
is not a layout itself — it's the building block layouts compose. It owns:

- The fixed navbar header (brand, nav items, header-extra slot, theme toggle).
- The left sidebar with three states: `open` (full width, icons + text),
  `collapsed` (narrow, icons only), `closed` (hidden). State persists to
  `localStorage` under `appshell-sidebar-state`.
- The scrolling main content region.

```go
type AppShellConfig struct {
    Title        string
    TitleHref    string
    NavItems     []daisyui.NavItem     // navbar-end
    MenuItems    []daisyui.MenuItem    // sidebar flat menu
    NavSections  []NavSection          // sidebar titled groups
    Content      templ.Component
    SidebarState SidebarState
    HeaderExtra  templ.Component
    Theme        ThemeConfig
    Resolver     daisyui.URLResolver   // set to nil when the layout pre-resolves
}
```

A new site-type layout should compose `AppShell` the same way the
documentation layout does: own the vocabulary, pre-resolve hrefs, and pass
`Resolver: nil` to AppShell to avoid double-prefixing.

## Adding a new site-type layout

1. Create `templates/layouts/<type>/` with a package named after the type
   (avoid the literal name `documentation` — Go reserves it).
2. Export a `Config` struct embedding the shared `layouts.Meta` and any
   layout-specific fields (sections, content slots, theme, resolver).
3. Export `Layout(cfg *Config) templ.Component` that composes `AppShell` and
   wraps it in `templates.Layout`.
4. Use the shared `layouts.NavItem` / `layouts.NavSection` vocabulary so the
   site-config level can swap layouts without import churn.
5. Pre-resolve hrefs with `Config.Resolver` and pass `Resolver: nil` to
   `AppShell`.
6. Add a `*_test.go` rendering the layout to a `strings.Builder` and
   asserting on the structure.

## Choosing a layout

- **Documentation site** → `templates/layouts/documentation`. Sidebar of
  `NavSection`s, main content region, optional TOC, light/dark theme toggle.
- **Blog / marketing / admin / SaaS landing** → no layout yet. Add one under
  `templates/layouts/<type>/` following the documentation layout's shape, or
  compose a thin page directly through `templates.Layout` for a one-off site
  that doesn't warrant a reusable shell.