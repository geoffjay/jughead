---
type: decision
title: UI components extracted to the templ-ui module
date: 2026-09-23
status: accepted
---

# UI components extracted to the templ-ui module

## Context

The templ UI components under `templates/components/` (daisyui, shiki) and
`templates/containers/` (AppShell) were being copied into other projects by
hand. jughead is a private repository, so consuming the components from CI
in other projects required awkward vendoring. The component showcase lived
inside jughead as the `sites/tld/domain1` site, coupled to
`sites/links.LinkResolver`.

## Decision

The components moved to a new **public** repository,
[geoffjay/templ-ui](https://github.com/geoffjay/templ-ui):

- `daisyui` — 66 daisyUI 5 components + render tests (unchanged API).
- `shiki` — `shiki.Code` plus a new `shiki.Head` layout helper; the bundle
  URL is now configurable via `shiki.ScriptSrc` (default `/static/shiki.js`,
  which matches jughead's existing asset path).
- `containers` — `AppShell` (imports `github.com/iota-uz/icons`).
- A component gallery (`examples/gallery`): a registry-driven live server
  plus a static export for GitHub Pages, replacing the domain1 showcase
  pattern. Local development uses [pitchfork](https://pitchfork.jdx.dev)
  daemons (`pitchfork.toml`), not overmind.

jughead consumes the components via `go.mod`
(`github.com/geoffjay/templ-ui`) instead of owning them. The local copies
are deleted.

## Consequences

- **Public availability**: any project can `go get
  github.com/geoffjay/templ-ui`; CI no longer needs private-repo access.
- **Version floor**: templ-ui declares `go 1.25.0` (jughead's floor) so
  jughead can always consume it. Bump both together.
- **Plugin type identity**: any change to the dependency graph (this one
  included) requires rebuilding all `.so` plugins (`task plugins`); stale
  artifacts fail `plugin.Open`. Leftover `google.so` binaries (source
  never committed) were removed.
- **Breaking-change policy**: the daisyui/shiki/containers public surface
  must not break without recording the decision in the templ-ui repo
  (this KB records the jughead-side impact only).
- The domain1 showcase site still exists in jughead but now renders the
  templ-ui components; the authoritative demo surface is templ-ui's
  gallery.
- Tailwind/daisyUI stylesheet requirements (`.glass`,
  `.component-preview`, theme lists) are documented in templ-ui's
  `docs/USAGE.md`; jughead's `assets/styles.css` remains the app-side
  build entry.