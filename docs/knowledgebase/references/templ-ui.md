---
type: reference
title: templ-ui component library
date: 2026-09-23
---

# templ-ui component library

- Repository: <https://github.com/geoffjay/templ-ui> (public)
- Module: `github.com/geoffjay/templ-ui`
- Contents: `daisyui` (66 daisyUI 5 components), `shiki` (code highlighting,
  `shiki.Head` setup helper, configurable `shiki.ScriptSrc`), `containers`
  (`AppShell`), and a registry-driven component gallery with live server +
  static GitHub Pages export.
- Consumer guide (Tailwind v4 + daisyUI CSS build, shiki bundle vendoring,
  head template): `docs/USAGE.md` in the templ-ui repo.
- Local gallery development uses pitchfork daemons (`pitchfork start` in the
  templ-ui repo) — the same pattern jughead uses overmind for.
- Version floor: `go 1.25.0`, matching jughead's `go.mod` floor; the two
  must be bumped together.
- CSS safelist: Tailwind never scans the Go module cache and many daisyUI
  modifiers are built at runtime, so component classes must come from
  templ-ui's safelist. `bun run safelist` (first step of `bun run css`) runs
  `go run github.com/geoffjay/templ-ui/cmd/templ-ui safelist` at the go.mod
  version into `assets/vendor/templ-ui/safelist.css` (gitignored), which
  `assets/styles.css` imports. Bumping templ-ui needs no manual copy.