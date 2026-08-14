---
type: decision
date: 2026-08-14
title: Use pitchfork for process management instead of overmind + Procfile
status: accepted
---

# Decision: pitchfork for process management

## Context

The project used a `Procfile` + overmind for local development process
management. The Procfile defined five processes:

| Name     | Command                  |
|----------|--------------------------|
| jughead  | `air`                    |
| web      | `bun run watch`          |
| shiki    | `bun run watch:shiki`    |
| css      | `bun run css`            |
| templ    | `templ generate -watch`  |

Overmind wraps a Procfile but provides limited readiness checking, no
dependency ordering, no file-watching-driven restarts, and no built-in MCP
server for AI assistant integration.

## Decision

Replace the Procfile + overmind with `pitchfork.toml` (pitchfork v2.21.0).
Each Procfile process becomes a pitchfork daemon under the `jughead` namespace.

Key additions beyond a 1:1 port:

- `ready_port = 9000` on the `jughead` daemon — pitchfork polls the HTTP port
  instead of guessing readiness by delay.
- `auto = ["start", "stop"]` on all daemons — shell hook auto-starts on `cd`
  into the project and auto-stops on leave.
- `retry = 3` on `jughead` — air/plugin build failures get retried.
- `exec` prefix on every `run` command — pitchfork tracks the actual daemon
  PID instead of the wrapper shell.

The overmind MCP server entry in `.opencode/opencode.jsonc` is replaced with
pitchfork's built-in MCP server (`pitchfork mcp`), giving AI assistants
start/stop/restart/logs/status tools.

## Consequences

- `Procfile` removed; `pitchfork.toml` is the single source of truth.
- `.opencode/opencode.jsonc` MCP section now references `pitchfork mcp`.
- `.air.toml` is unchanged — air still handles Go rebuild + plugin pre-build;
  pitchfork manages the process lifecycle around it.
- Developers need pitchfork installed (`mise use -g pitchfork` or
  `cargo install pitchfork-cli`).