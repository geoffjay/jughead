# AGENTS.md

Guidance for coding agents working in **jughead** — a multi-tenant site platform
(Go) with a `.so` plugin architecture. A host server runs an admin UI, Postgres,
auth, built-in sites, and plugins; the `jughead` CLI runs the host and scaffolds
plugins.

## Knowledge base

This project keeps an OKF knowledge base at [`docs/knowledgebase/`](docs/knowledgebase/index.md).
**Consult it before acting** and **update it after acting** — the policy and the
full map live in its `index.md`.

## Layout

| Path | What lives here |
| --- | --- |
| `main.go`, `server.go`, `handlers.go`, `admin.go` | Host server (Gin) + admin UI |
| `cmd/jughead/{init,serve,version}` | Cobra CLI subcommands |
| `db/` | Postgres access (pgx), `migrations/`, `seed/`, RLS |
| `services/` | Domain services (org, user, membership, invitation) + `github/` auth provider |
| `middleware/` | Gin middleware (auth, proxy) |
| `plugin/` | `.so` plugin loader |
| `sdk/` | Public plugin SDK (`sdk.Plugin`, `sdk/auth`) — plugins build against this |
| `sites/` | Built-in sites (`auth`, `com`, `links`, `tld`) + manager |
| `templates/init/{site,provider}` | Embedded `jughead init` scaffold templates |
| `assets/`, `static/` | Frontend assets (templ + htmx + daisyUI, built with bun/parcel) |

## Common tasks

Run via [Task](https://taskfile.dev) (`task <name>`):

- `task build` — build the host binary (static, no CGO).
- `task build:plugins` — build the host with CGO enabled (required to load `.so` plugins).
- `task plugins` / `task plugin NAME=sites/docs` — build all / one plugin.
- `task test` — all Go tests (starts Postgres, sets `DATABASE_URL`).
- `task test:short` — tests excluding db integration tests. `task test:db` — db tests only.
- `task db:up` / `db:create` / `db:seed` / `db:recreate` — manage the dev Postgres.

## Conventions

- **Go 1.25**, module `github.com/geoffjay/jughead`. Format with `gofmt`; lint config in `.golangci.yml`.
- **Plugins**: host and plugin must be built with the same Go toolchain and jughead
  module version (Go `plugin` type-identity constraint). Never break `sdk/`'s public
  surface without recording the decision in the KB.
- **Database**: schema changes are SQL migrations under `db/migrations/` (numbered,
  idempotent). RLS is active — see `db/rls.go`.
- Prefer editing existing files over adding new ones; match surrounding style.
