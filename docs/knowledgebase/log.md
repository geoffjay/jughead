# Knowledge Base Update Log

## 2026-08-14
* **Scaffold**: Created the jughead knowledge base at `docs/knowledgebase/` using the `okf-ify` skill. Initial structure: `index.md`, `log.md`, and concept directories (concepts, decisions, patterns, references, plans). OKF v0.2 conformant.
* **Process management**: Switched from Procfile + overmind to `pitchfork.toml`. Removed `Procfile`; replaced overmind MCP in `.opencode/opencode.jsonc` with `pitchfork mcp`. Added decision `decisions/pitchfork-process-management.md`.
