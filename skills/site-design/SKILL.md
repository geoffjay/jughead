---
name: site-design
description: Design preferences and conventions for building jughead sites — templ-first rendering, daisyUI 5 via jughead's typed component wrappers, htmx for interactivity, link resolution that survives the reverse proxy, and per-site theming. Use when designing a new site, choosing components, theming, or making UI/UX decisions within jughead.
---

# jughead site design

jughead sites are server-rendered with templ, styled with daisyUI 5 (via
jughead's typed component wrappers), and made interactive with htmx. This
skill captures the conventions that keep sites consistent, type-safe, and
working across both access modes (localhost site path and FQDN reverse proxy).

Use when:

- Designing a new site or page
- Choosing how to render a UI element (wrapper vs. raw classes vs. layout)
- Theming a site
- Making UX decisions (navigation, interactivity, progressive enhancement)
- Deciding where a piece of UI belongs (site, layout, component, container)

## Principles

1. **Server-render with templ; enhance with htmx.** Default to rendering a
   full page from a `templ.Component`. Add htmx attributes only when a region
   needs to swap without a full reload. Never build a client-side SPA — there
   is no JS framework in the stack by design.
2. **Prefer jughead's typed daisyUI wrappers over raw classes.** Every daisyUI
   component in `templates/components/daisyui/` is a `templ` function taking a
   typed `<Name>Config`. Use the wrapper; fall back to raw `class="..."` only
   when no wrapper models the variation — and then add the wrapper. See the
   `jughead-components` skill.
3. **Always resolve links through `sites/links.LinkResolver`.** A site is
   reachable two ways: directly under `/sites/<host>` and via its FQDN through
   the reverse proxy (browser sees root-relative URLs). Hard-coding the site
   path into an href breaks one of the two modes. See Link resolution below.
4. **One theme per site, declared on `Site.Theme`.** daisyUI 5 ships 35
   themes; pick one (or a light/dark pair for layouts that support a toggle).
   Theme names are strings (`"nord"`, `"kanagawa-dark"`, `"coffee"`); when
   empty the host defaults to `"light"`.
5. **Compose through layouts, not ad-hoc shells.** Render pages through a
   site-type layout (`templates/layouts/<type>`) so the navbar, sidebar,
   `<head>`, asset links, and CSP are consistent. See the `layouts` skill.
6. **No custom CSS unless daisyUI + Tailwind utilities can't express it.**
   daisyUI class names and Tailwind utility classes are the only allowed
   styling. Reach for custom CSS only as a last resort, and prefer adding a
   utility class to a wrapper's `Class` field first. (From the daisyUI usage
   rules.) Don't add a custom font unless necessary. Don't add
   `bg-base-100 text-base-content` to `<body>` unless necessary.
7. **Use the default daisyUI variant unless the user asks otherwise.** Don't
   reach for a color/size/modifier variation unprompted; the default is the
   design baseline.
8. **For design decisions, follow *Refactoring UI* best practices** (spacing,
   hierarchy, color, typography) — this is the daisyUI skill's stated rule and
   applies here too.

## Rendering pipeline

A request hits the site's route group → the `Routes` callback's handler
builds a `Config` for the chosen layout → `Layout(&cfg)` returns a
`templ.Component` → `htmx.NewResponse().RenderTempl(ctx, w, component)` writes
it. For HTMX fragment swaps, render the body component directly to `c.Writer`
without the layout, and let the client swap it into the main region.

```go
import (
    "github.com/angelofallars/htmx-go"
    doc "github.com/geoffjay/jughead/templates/layouts/documentation"
)

siteTemplate := doc.Layout(&cfg)
if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
    c.AbortWithStatus(http.StatusInternalServerError)
}
```

## Link resolution (the dual-access trap)

Every href in a site must go through a `LinkResolver` so it works in both
access modes. Construct one per request:

```go
import "github.com/geoffjay/jughead/sites/links"

r := links.NewLinkResolver(c.Request, sitePath)
r.Resolve("/install")
// direct:   "/sites/foo.example.com/install"
// proxied:  "/install"
```

Pass `r` (a `daisyui.URLResolver`) into:

- Layout `Config.Resolver` — the layout pre-resolves all nav/content hrefs.
- Component `Config.Resolver` fields (Button, Navbar, etc.) — for hrefs the
  layout doesn't know about.
- `r.SafeURL(href)` when you need a `templ.SafeURL` directly in markup.

For pre-rendered templates stored on `Site` (no request available), use
`links.NewDirectResolver(sitePath)`. Never hard-code `sitePath` into a string
concatenation — it changes between access modes.

## Theming

`Site.Theme` is a daisyUI 5 theme name string. The host applies it to `<html>`
via the layout's `Meta.Theme`. When empty, defaults to `"light"`. For
layouts that support a light/dark toggle (the documentation layout's
`ThemeConfig`), pass both `LightTheme` and `DarkTheme` and a unique
`StorageKey` so the user's choice persists per site without colliding on a
shared origin.

Pick from the daisyUI 5 theme list (see the `daisyui`/`daisyui-colors`
skills): `light`, `dark`, `cupcake`, `nord`, `dracula`, `coffee`,
`kanagawa-light`, `kanagawa-dark`, etc. Use semantic color classes
(`text-base-content`, `bg-base-100`, `text-primary`) so a theme swap recolors
the whole site — never hard-code hex colors.

## Navigation

- **Top-level nav** lives in the layout's `Config.PrimaryNav`
  (`[]layouts.NavItem`) — rendered in the navbar-end region.
- **Sidebar nav** for documentation-style sites lives in `Config.Sections`
  (`[]layouts.NavSection`), titled groups of `NavItem`s. Set `Active: true` on
  the item matching the current request so the sidebar highlights location.
- **Icons** are phosphor icon names on `NavItem.Icon` (e.g. `"House"`,
  `"BookOpen"`, `"Gear"`). The layout resolves them at render time via
  `phosphor.Icons`. Unknown names are ignored, so collapsed sidebars still
  show icons for items that have them and labels-only for items that don't.
- **Brand** is `Config.Brand` (a `layouts.NavItem` with `Label` and `Href`),
  rendered as the navbar title.

## Interactivity (htmx)

Use htmx for partial updates, not as a general-purpose JS runtime:

- `hx-get`/`hx-post` with `hx-target` and `hx-swap` for fragment swaps.
- `hx-trigger="keyup changed delay:500ms"` for live search.
- `hx-trigger="revealed"` for infinite scroll.
- `hx-confirm` for destructive actions; `hx-target="closest tr"` +
  `hx-swap="outerHTML"` for row removal.
- Out-of-band updates (`hx-swap-oob="true"`) when one action updates multiple
  regions (e.g. a cart count in the navbar).
- Always provide a non-JS fallback: a `<form action method>` that works
  without htmx, enhanced by `hx-post` for progressive enhancement.

Render fragments as bare `templ.Component`s (no layout) to the response
writer; the client swaps them in. See the `templ-htmx` skill for the full
attribute reference and patterns.

## Where things belong

| Concern                          | Location                                              |
| -------------------------------- | ----------------------------------------------------- |
| daisyUI class wrapper            | `templates/components/daisyui/<name>.templ`           |
| Shared shell (navbar/sidebar/main) | `templates/containers/appshell.templ`              |
| Site-type page shell             | `templates/layouts/<type>/` (e.g. `documentation/`)   |
| Page body for one page           | inside the site plugin (e.g. `plugins/sites/docs/pages.templ`) |
| Site registration & routes       | the site plugin's `plugin.go` + `routes.go`           |
| Auth flow                        | a provider plugin (`plugins/providers/<name>/`)      |
| Link prefix logic                | `sites/links/links.go`                                |

When a UI element is reused across sites, promote it to a wrapper or a
container; when it's specific to one site, keep it in the site plugin.

## Reference

- `plugins/sites/docs/` — the canonical site plugin: `Routes` wired, pages
  rendered through the documentation layout, `LinkResolver` used throughout,
  `daisyui.Code` wrapper for code blocks, per-page sidebar active state.
- `templates/layouts/documentation/documentation.go` — the canonical layout:
  composes `AppShell`, pre-resolves hrefs, configures the theme toggle.
- `templates/components/daisyui/card.templ`, `button.templ`, `code.templ`,
  `navbar.templ` — the wrapper conventions in miniature.