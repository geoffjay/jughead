# jughead agent skills

Reusable instruction sets for coding agents working on jughead sites and
plugins. Each skill is a `SKILL.md` with YAML frontmatter (`name`,
`description`) that an agent loads when the task matches the description.

## Install

Install all skills into your project (recommended: committed with the project
so your team shares them):

```
npx skills add geoffjay/jughead
```

Install to your user directory instead of the project:

```
npx skills add geoffjay/jughead -g
```

Install specific skills only:

```
npx skills add geoffjay/jughead --skill site-plugin --skill provider-plugin
```

List available skills without installing:

```
npx skills add geoffjay/jughead --list
```

The CLI auto-detects which coding agents you have installed (OpenCode, Claude
Code, Cursor, Codex, and 60+ others) and installs to the right paths. To
target a specific agent:

```
npx skills add geoffjay/jughead -a opencode -a claude-code
```

See `npx skills add --help` for all options.

## Skills

| Skill | Use when |
| ----- | -------- |
| [`site-plugin`](site-plugin/SKILL.md) | Building a jughead site plugin (`.so`): scaffolding, the `sdk.Plugin`/`sdk.Site` contract, `Template`/`Proxy`/`Routes`, link resolution, build & run. |
| [`provider-plugin`](provider-plugin/SKILL.md) | Building a jughead auth-provider plugin: `sdk/auth.Provider`/`ProviderInstance`, `Load`, OAuth web-flow routes, session middleware, `AuthConfig` wiring. |
| [`jughead-components`](jughead-components/SKILL.md) | Rendering daisyUI in a jughead site — prefer the typed templ wrappers in `templates/components/daisyui/` over raw class strings. MANDATORY before writing daisyUI markup in a site. |
| [`layouts`](layouts/SKILL.md) | Using jughead's site-type layout system (`templates/layouts/`) to render full pages, composing a `Config`, adding a new layout, the shared `AppShell` container. |
| [`site-design`](site-design/SKILL.md) | Design conventions for jughead sites: templ-first rendering, daisyUI via wrappers, htmx for interactivity, link resolution, theming, where things belong. |

## Companion upstream skills

These repo skills assume the upstream daisyUI and templ skills are also
installed (they ship with many agents or can be added separately):

- `daisyui` — daisyUI 5 class-name reference, color rules, component docs.
- `templ-syntax`, `templ-components`, `templ-htmx` — templ language,
  component composition, and htmx integration.

Install them with:

```
npx skills add saadeghi/daisyui
```

## Layout

```
skills/
  site-plugin/SKILL.md
  provider-plugin/SKILL.md
  jughead-components/SKILL.md
  layouts/SKILL.md
  site-design/SKILL.md
  README.md
```

Each skill is self-contained and can be installed independently via
`--skill <name>`.