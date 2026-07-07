---
name: jughead-components
description: Use jughead's templ-based daisyUI component library in templates/components/daisyui before writing raw daisyUI class markup. MANDATORY when rendering UI in a jughead site or plugin. Use when building pages, layouts, nav, cards, code mockups, buttons, or any daisyUI element within a jughead site.
---

# jughead daisyUI component library

jughead ships a typed templ wrapper around daisyUI 5 in
`templates/components/daisyui`. Every daisyUI component is wrapped as a
`templ` function taking a `<Name>Config` struct, so you get compile-time
validation and IDE autocomplete instead of hand-writing class strings.

**MANDATORY:** before writing any `class="btn ..."` or `class="card ..."`
markup in a jughead site, check whether a typed wrapper already exists and use
it. Only fall back to raw class strings when no wrapper fits.

## When to use this skill

- Rendering any daisyUI component inside a jughead site or plugin
- Building page bodies, navbars, sidebars, cards, code blocks, buttons
- Deciding between two daisyUI components for a UI need
- Authoring a new wrapper that follows the established conventions

This skill complements the upstream `daisyui` skill (class-name reference).
Use the upstream skill to learn daisyUI's class taxonomy; use this skill to
prefer jughead's typed wrappers over raw classes.

## The convention

Every wrapper follows the same shape, established in
`templates/components/daisyui/`:

```templ
package daisyui

// <Name>Config configures a daisyUI <Name>. <field docs...>
type <Name>Config struct {
    Label    string
    Href     string
    Color    string         // documented suffix: "primary", "secondary", ...
    Style    string         // documented suffix: "outline", "soft", ...
    Size     string         // "xs" | "sm" | "md"(default, omitted) | "lg" | "xl"
    Modifier string         // documented modifier
    Class    string         // extra utility classes
    Children templ.Component // when the component composes arbitrary content
    Resolver URLResolver    // resolves site-relative hrefs
}

// <Name> renders a daisyUI <name> using the provided configuration.
templ <Name>(cfg <Name>Config) {
    <div class={ <name>Class(cfg) }>
        if cfg.Children != nil {
            @cfg.Children
        }
        { cfg.Label }
    </div>
}

func <name>Class(cfg <Name>Config) string {
    return joinClasses(
        "<name>",
        classFor("<name>", cfg.Color),
        <name>StyleClass(cfg.Style),
        <name>SizeClass(cfg.Size),
        classFor("<name>", cfg.Modifier),
        cfg.Class,
    )
}
```

Conventions to follow when adding or modifying a wrapper:

- **Config struct, value receiver.** Components take `<Name>Config` by value
  (never a pointer) and are named `templ <Name>(cfg <Name>Config)`.
- **Field docs in a comment above the struct.** Each field's accepted values
  and the class it produces are documented in the doc comment — read the
  comment before using a component.
- **`Size` uses documented suffixes.** `"md"` is the default and is omitted
  (no class emitted); `"xs"`, `"sm"`, `"lg"`, `"xl"` emit `btn-sm` etc.
- **`Color`, `Style`, `Modifier` use a `switch` helper** that only accepts the
  documented suffixes; anything else emits `""`. Never concatenate raw user
  strings into a class.
- **`Class` is the escape hatch** for utility classes the wrapper doesn't
  model (`"w-96 shadow-sm"`). It's appended last so callers can override.
- **`Children templ.Component`** for arbitrary composed content; render with
  `@cfg.Children`. `Label` is rendered after children when both are set.
- **`Resolver URLResolver`** on anything with an `href`. Never hard-code the
  site path — pass the site's `LinkResolver` so links work both directly
  (`/sites/<host>/...`) and via the FQDN reverse proxy (`/...`).
- **`glassClass(true)`** emits `"glass"` for the daisyUI glass modifier.
- **`joinClasses(...)`** joins non-empty strings with spaces; use it instead
  of manual concatenation.
- **One component per `.templ` file** named after the daisyUI class
  (`card.templ`, `button.templ`), plus the generated `*_templ.go` and a
  `*_test.go` that renders the component to a string and asserts on the
  output. Run `templ fmt` and `go test ./templates/...` after changes.

## Available wrappers

These live in `templates/components/daisyui/`. Each has a `<Name>Config`
struct documented in its `.templ` file — read that file's top comment before
using the component.

accordion, alert, avatar, badge, breadcrumbs, browser, button, calendar,
card, carousel, chat-bubble, checkbox, code, collapse, countdown, diff,
diff-view, divider, dock, drawer-sidebar, dropdown, fieldset, file-input,
filter, floating-action-button, footer, hero, hover-3d-card, hover-gallery,
indicator, input-field, join, kbd, label, link, list, loading, mask, menu,
modal, navbar, pagination, phone, progress, radial-progress, radio, range,
rating, select, skeleton, stack, stat, status, steps, swap, table, tabs,
text-rotate, textarea, theme-controller, timeline, toast, toggle, tooltip,
validator, window.

Plus shared helpers: `util.go` (`joinClasses`, `classFor`, `glassClass`,
`resolveHref`).

## Discovery protocol

Before writing daisyUI markup in a jughead template:

1. Read the request intent, behavior, and shape — match on meaning, not
   literal words. (Semantic matching is required even when wording differs
   from component names — a "loading spinner" maps to `loading`, a "tag" to
   `badge`.)
2. Shortlist candidate daisyUI components using the upstream `daisyui` skill's
   component list; read at least 3 candidate docs when there's ambiguity.
3. For each candidate, check whether a jughead wrapper exists in
   `templates/components/daisyui/<candidate>.templ`. If it does, read its
   `<Name>Config` doc comment.
4. Prefer the jughead wrapper over raw classes. Only fall back to raw
   `class="..."` markup when no wrapper exists or the wrapper can't model the
   required variation — and in that case, add the missing wrapper rather
   than scattering raw classes through the site template.
5. State which wrapper you chose and why it matches the request.

## Usage example (docs site)

The docs plugin (`plugins/sites/docs/pages.templ`) renders code blocks through
the `Code` wrapper instead of hand-writing `<div class="mockup-code">`:

```templ
import "github.com/geoffjay/jughead/templates/components/daisyui"

@daisyui.Code(daisyui.CodeConfig{
    Class: "w-full",
    Lines: []daisyui.CodeLine{
        {Prefix: "$", Text: "git clone https://github.com/geoffjay/jughead.git"},
        {Prefix: "$", Text: "cd jughead"},
        {Prefix: "$", Text: "go run ."},
    },
})
```

A button with link resolution:

```templ
@daisyui.Button(daisyui.ButtonConfig{
    Label:    "Go to introduction",
    Href:     "/",
    Color:    "primary",
    Resolver: resolver,
})
```

`resolver` is a `daisyui.URLResolver` — pass the site's `links.LinkResolver`,
which satisfies the interface.

## Adding a new wrapper

1. Create `templates/components/daisyui/<name>.templ` with package `daisyui`,
   a `<Name>Config` struct (documented fields), the `templ <Name>(cfg)` and a
   `func <name>Class(cfg)` helper using `joinClasses`/`classFor`.
2. Run `templ generate` (or `task` via the air watcher) to emit `<name>_templ.go`.
3. Add `<name>_test.go` rendering the component to a `strings.Builder` and
   asserting on key classes/attributes.
4. Run `templ fmt` then `go test ./templates/components/daisyui/...`.

A wrapper that composes other wrappers (e.g. `navbar` uses `menu`-style
markup) should accept `templ.Component` slots (`StartExtra`, `EndExtra`,
`Children`) so callers can inject without forking the template.