---
name: site-plugin
description: Develop jughead site plugins (.so) that ship templ/gini-rendered sites loaded at runtime by the host. Use when building a new site, scaffolding with `jughead init`, writing a Site's Template/Proxy/Routes, or referencing the sdk.Site contract across the .so boundary.
---

# jughead site plugins

A jughead **site plugin** is a Go shared object (`.so`) built against the
`github.com/geoffjay/jughead/sdk` package and loaded at startup by the host's
plugin loader. It contributes one or more sites to the host's SiteManager.

Use this skill whenever you are:

- Scaffolding a new site plugin (`jughead init --plugin site`)
- Writing a `var Plugin = sdk.Plugin{...}` that ships `Sites`
- Wiring a site's `Template`, `Proxy`, or `Routes` callback
- Resolving links between the site path and the FQDN reverse proxy
- Debugging "plugin was built with a different version of package ..."

## Hard constraint: type identity across the .so boundary

Go's `plugin` package requires the host and every plugin to be built with the
**same Go toolchain version** and **byte-identical dependency versions**.
A mismatch panics at load time. Always build via the Taskfile against the
pinned `go.mod`:

```
task plugin NAME=sites/<name>      # build one plugin
task plugins                       # build every plugin
task build:plugins                 # host binary that can load plugins
```

Never mix a host built with `task build` (static, `CGO_ENABLED=0`) with
plugins — that host cannot load any `.so`. Use `task build:plugins`.

## Scaffold

```
jughead init --plugin site --name foo path/to/foo
```

This renders `templates/init/site/*.tmpl` into the target, producing:

- `plugin.go` — declares `var Plugin = sdk.Plugin{...}` with one `sdk.Site`.
- `main_stub.go` — no-op `main()` so `go build` works without `-buildmode=plugin`.
- `go.mod` — pins the jughead version; keep it in sync with the host.
- `README.md` — build/run instructions.

## The contract

A plugin is a `package main` that exports a package-level `var Plugin` of type
`sdk.Plugin` (value type, not a pointer — `plugin.Lookup` returns a `*T`).

```go
package main

import (
    "github.com/geoffjay/jughead/sdk"
)

var Plugin = sdk.Plugin{
    Name: "foo",
    Sites: []sdk.Site{
        {
            Path:      "/sites/foo.example.com",
            Url:       "https://foo.example.com",
            Published: true,
            Theme:     "nord",
            Template:  placeholder(),  // templ.Component (fallback view)
            Routes:    Routes,         // func(*gin.RouterGroup, string)
        },
    },
}
```

`sdk.Site` fields and their host-side type assertions:

| Field      | SDK type | Host asserts to                              |
| ---------- | -------- | -------------------------------------------- |
| `Template` | `any`    | `templ.Component` — rendered when `Routes` is nil or auth is incomplete. |
| `Proxy`    | `any`    | `func(*gin.Context)` — reverse-proxy fallback. |
| `Routes`   | `any`    | `func(*gin.RouterGroup, string)` — registers site handlers; receives the site's theme. |
| `Auth`     | `*auth.AuthConfig` | Passed through; names a registered provider. |

An assertion failure for a populated field fails the whole site (logged and
skipped). A `nil` field is left zero, so a provider-only plugin ships an empty
`Sites` slice without issue.

## Routing

`Routes` is called once at startup with a `*gin.RouterGroup` whose base path
is the site's `Path` (`/sites/<host>`). Register sub-page handlers on it:

```go
func Routes(router *gin.RouterGroup, theme string) {
    router.GET("", indexHandler)
    router.GET("/", indexHandler)
    for _, p := range Pages() {
        p := p
        router.GET(p.Slug, func(c *gin.Context) { renderPage(c, p.Slug) })
    }
}
```

`Routes` takes precedence over `Template`: when `Routes` is non-nil the host
calls it and never renders `Template` directly. `Template` is the fallback
rendered by the host's `siteViewHandler` when `Routes` is nil or auth config is
incomplete — keep it a minimal placeholder so the site still responds.

## Link resolution (the dual-access trap)

A site is reachable two ways: directly under `/sites/<host>` (localhost dev)
and via its FQDN through the reverse proxy (browser sees root-relative URLs).
**Never hard-code the site path into hrefs.** Always resolve links through
`sites/links.LinkResolver`:

```go
import "github.com/geoffjay/jughead/sites/links"

func resolver(c *gin.Context) links.LinkResolver {
    return links.NewLinkResolver(c.Request, sitePath)
}
// resolver.Resolve("/install") -> "/sites/foo.example.com/install" (direct)
//                               -> "/install"                  (proxied)
```

For pre-rendered templates stored on `Site` (no request available), use
`links.NewDirectResolver(sitePath)`. Pass the resolver into your layout's
`Config.Resolver` so it prefixes site-relative hrefs correctly.

## Theme

`Site.Theme` selects a daisyUI 5 theme name (e.g. `"nord"`, `"kanagawa-dark"`,
`"coffee"`). When empty the host defaults to `"light"`. The theme string is
passed to `Routes` as the second argument so handlers can forward it to
`templates.Layout`. See the daisyUI skill for the full theme list and color
rules.

## Rendering pages

Use `htmx-go`'s `RenderTempl` to write a `templ.Component` to the response:

```go
import "github.com/angelofallars/htmx-go"

siteTemplate := doc.Layout(&cfg)
if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
    c.AbortWithStatus(http.StatusInternalServerError)
}
```

For fragments (HTMX swaps), render the component directly to `c.Writer`; no
layout wrapper.

## Build & run

From the plugin directory (or repo root for in-repo plugins):

```
CGO_ENABLED=1 go build -buildmode=plugin -o foo.so .
```

Iterate standalone without the database or admin UI:

```
jughead serve ./foo.so --port 8080
```

Run inside the full host: set `JUGHEAD_PLUGINS_DIR` to the directory
containing `foo.so` and start `jughead`. The site registers under its `Path`.
A plugin site loads **before** built-in `sites.Initialize()`, so a plugin and
a built-in registered under the same `Path` collide — the built-in wins
(unless you remove the in-repo entry in `sites/sites.go`). Pick a unique path.

## Strict mode

Set `JUGHEAD_PLUGINS_STRICT=1` to make the first per-plugin load error fatal —
useful in CI to catch toolchain/dependency drift early. Without it, a failed
plugin is logged and skipped.

## Reference plugin

`plugins/sites/docs/` is the canonical example: it ships the docs site with
`Template` (placeholder), `Proxy`, and `Routes` all wired, and renders pages
through `templates/layouts/documentation`. Read it before starting a new site
plugin.