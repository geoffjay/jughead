# Subprocess Plugin Implementation Plan

_Date: 2026-07-04 · Scope: detailed implementation plan for subprocess-based site plugins_

This document expands on Section 5 of `docs/plugin-research.md`. It specifies the
file-level changes, data structures, interfaces, and sequencing needed to
implement the subprocess + HTTP reverse-proxy plugin architecture alongside the
existing `.so` loader.

## 1. Design Principles

1. **Coexistence, not replacement.** The `.so` loader stays for in-repo plugins.
   The subprocess loader is additive — both register into the same
   `SiteManager`, both coexist in the same host process.
2. **Jughead manages the lifecycle.** The host starts plugin subprocesses,
   health-checks them, restarts them on crash, and shuts them down gracefully.
   Plugin developers don't run anything separately.
3. **HTTP is the boundary.** The host and plugin communicate over HTTP. No Go
   types cross the process boundary. The plugin can be built with any toolchain
   or language.
4. **The `jughead serve` CLI is the shared code path.** The standalone serve
   command and the host's subprocess launcher use the same SDK to start the
   plugin's HTTP server. One codebase, two modes (standalone and host-managed).
5. **Auth stays host-owned.** The host runs OAuth flows and passes authenticated
   identity to plugins via signed headers. Plugins don't implement their own
   auth — they trust the host's signed context.

## 2. Architecture Overview

```
                          ┌──────────────────────────────────┐
                          │           Jughead Host            │
                          │                                  │
   HTTP request ────────► │  gin router                       │
                          │    ├─ /admin/*  (admin UI)        │
                          │    ├─ /login, /logout             │
                          │    ├─ /auth/<provider>/* (OAuth) │
                          │    ├─ /sites/<in-repo>/* (.so)    │
                          │    └─ /sites/<subprocess>/*      │
                          │           │                      │
                          │           ▼                      │
                          │  middleware.ReverseProxy          │
                          │           │                      │
                          └───────────┼──────────────────────┘
                                      │  HTTP proxy
                          ┌───────────▼──────────────────────┐
                          │     Plugin Subprocess             │
                          │  (e.g. docs-plugin binary)        │
                          │                                  │
                          │  gin router (own port, e.g. 10001)│
                          │    ├─ /health                    │
                          │    ├─ /static/* (go:embed)        │
                          │    └─ /sites/docs.../* (routes)   │
                          └──────────────────────────────────┘
```

The host's `middleware.ReverseProxy` already proxies FQDN-based requests to
localhost upstreams (`middleware/proxy.go:31`). For subprocess plugins, it
additionally proxies path-based requests (`/sites/<plugin-path>/*`) to the
plugin's localhost port. The existing reverse-proxy logic for bypassing
`/static` and `/auth` paths is reused.

## 3. Component Specification

### 3.1 Plugin Manifest (`plugin/manifest.go`)

A YAML file placed in the plugins directory alongside plugin binaries. The
loader discovers both `.so` files (existing path) and `.yml` / `.yaml` manifest
files (new path) in the same directory scan.

```go
// Package plugin — plugin/manifest.go

// Manifest declares a subprocess plugin to the host. It replaces the
// sdk.Plugin Go struct for external plugins: the host reads this file instead
// of looking up a Go symbol, so no Go type identity is required across the
// process boundary.
type Manifest struct {
    // Name is a human-readable identifier for the plugin (used in logs).
    Name string `yaml:"name"`

    // Binary is the path to the plugin executable, relative to the manifest
    // file's directory.
    Binary string `yaml:"binary"`

    // Sites declares the sites this plugin serves. A plugin may serve
    // multiple sites; each gets its own route group on the host's router and
    // shares the single subprocess.
    Sites []ManifestSite `yaml:"sites"`

    // Env is an optional map of environment variables to pass to the
    // subprocess. Merged with the host's injected vars (JUGHEAD_PORT, etc.).
    Env map[string]string `yaml:"env,omitempty"`
}

// ManifestSite declares a single site served by a subprocess plugin.
type ManifestSite struct {
    // Path is the URL path prefix the site mounts under, e.g.
    // "/sites/docs.jughead.geoffjay.com". Must be unique across all loaded
    // sites (plugins and built-ins).
    Path string `yaml:"path"`

    // Url is the public-facing URL for FQDN-based reverse proxying. When
    // non-empty, the host adds {host: localhost:port} to the proxy targets so
    // requests to this FQDN are forwarded to the plugin.
    Url string `yaml:"url"`

    // Theme is the daisyUI theme name applied to the site (e.g. "nord",
    // "kanagawa-dark"). Passed to the plugin via JUGHEAD_SITE_THEME.
    Theme string `yaml:"theme"`

    // Published controls whether the site loads in production. When false,
    // the site only loads when JUGHEAD_ENV is "dev" or "test" (same
    // ShouldLoad semantics as built-in sites).
    Published bool `yaml:"published"`

    // Auth, when non-nil, declares an auth provider for the site. The host
    // runs the OAuth flow and passes authenticated identity to the plugin via
    // signed headers. The plugin does not implement auth itself.
    Auth *AuthConfig `yaml:"auth,omitempty"`
}

// AuthConfig mirrors sdk/auth.AuthConfig for YAML deserialization.
type AuthConfig struct {
    Provider     string `yaml:"provider"`
    RedirectURL  string `yaml:"redirectURL,omitempty"`
    AllowedLogin string `yaml:"allowedLogin,omitempty"`
}
```

**Example manifest file** (`plugins/docs/plugin.yml`):

```yaml
name: docs
binary: ./docs-plugin
sites:
  - path: /sites/docs.jughead.geoffjay.com
    url: https://docs.jughead.geoffjay.com
    theme: nord
    published: true
```

### 3.2 Subprocess Loader (`plugin/subprocess.go`)

Responsible for reading a manifest, starting the plugin binary as a subprocess,
health-checking it, and registering it with the `SiteManager`.

```go
// Package plugin — plugin/subprocess.go

// SubprocessPlugin is a loaded subprocess plugin: the manifest, the running
// process handle, and the port it's listening on. The host holds these in a
// registry for lifecycle management (restart, shutdown).
type SubprocessPlugin struct {
    Manifest Manifest
    Cmd      *exec.Cmd
    Port     int
    Sites    []*sites.Site
}

// loadSubprocess reads a manifest file, starts the plugin binary, health-checks
// it, and returns the SubprocessPlugin. Does not register with SiteManager —
// the caller does that so registration order is explicit.
func loadSubprocess(ctx context.Context, manifestPath string) (*SubprocessPlugin, error)

// HealthCheck polls the plugin's /health endpoint until it responds 200 or
// the timeout expires. Returns an error if the plugin doesn't become healthy
// in time.
func (sp *SubprocessPlugin) HealthCheck(ctx context.Context, timeout time.Duration) error

// Stop sends SIGTERM to the subprocess, waits up to gracefulTimeout for it to
// exit, then sends SIGKILL if it's still running.
func (sp *SubprocessPlugin) Stop(gracefulTimeout time.Duration) error
```

**Startup sequence** (`loadSubprocess`):

1. Read and parse the manifest YAML.
2. Resolve the binary path relative to the manifest file.
3. Allocate a port: scan from `basePort` (default 10000, configurable via
   `JUGHEAD_PLUGIN_BASE_PORT`) upward until a free port is found.
4. Construct the environment:
   - `JUGHEAD_PORT=<allocated port>`
   - `JUGHEAD_SITE_PATH=<site path>` (first site; multi-site plugins would
     receive a comma-separated list)
   - `JUGHEAD_SITE_THEME=<theme>`
   - Any env vars from `Manifest.Env`.
5. Start the binary via `exec.CommandContext`, inheriting the host's stdout/
   stderr (or capturing them into a per-plugin logger — see Open Questions in
   the research doc).
6. Wait for `/health` to return 200 (with a configurable timeout, default 10s).
7. Build `[]*sites.Site` entries from the manifest's site declarations, with
   `Proxy` set to a function that delegates to `httputil.NewSingleHostReverseProxy`
   pointing at `localhost:<port>`. The site's `Routes` is nil (the plugin owns
   its own routes); `Template` is nil (the plugin renders HTML).
8. Return the `SubprocessPlugin`.

### 3.3 Proxy Integration with SiteManager

The `SiteManager` currently has two access modes for sites:

1. **Direct routing**: `BuildSiteRoutes` mounts each site's `Routes` callback onto
   a gin router group under the site's `Path`.
2. **FQDN reverse proxy**: `BuildProxyTargets` maps `{FQDN-host: localhost:port/sites/path}`
   for `middleware.ReverseProxy`.

Subprocess plugins use **neither** of these directly. Their routes live in the
subprocess, not the host. The host needs a third access mode: **path-based
reverse proxy** — requests to `/sites/<plugin-path>/*` are proxied to the
subprocess's port.

**Change to `sites/manager.go`:**

Add a `SubprocessTargets` field to `SiteManager` and a `BuildSubprocessProxyMiddleware`
method:

```go
// sites/manager.go

type SiteManager struct {
    sites             map[string]*Site
    subprocessTargets map[string]string // site-path → "http://localhost:<port>"
}

// RegisterSubprocessTarget registers a proxy mapping for a subprocess plugin
// site. Requests to sitePath and its sub-paths are proxied to upstream.
func (sm *SiteManager) RegisterSubprocessTarget(sitePath, upstream string)

// SubprocessTargets returns the {site-path: upstream} map for the path-based
// proxy middleware.
func (sm *SiteManager) SubprocessTargets() map[string]string
```

**New middleware** (`middleware/path_proxy.go`):

A path-based reverse proxy that fires **before** the gin router's site groups. It
checks whether the request path starts with any registered subprocess site path;
if so, it proxies to the subprocess upstream. If not, it falls through to the
router (so `.so`-backed sites and built-in routes work as before).

```go
// middleware/path_proxy.go

// PathReverseProxy proxies requests whose path starts with a registered
// subprocess site path to the corresponding upstream. Unlike ReverseProxy
// (which matches on host), this matches on URL path prefix. Installed before
// the gin router's site groups so subprocess sites never hit the router.
func PathReverseProxy(targets map[string]string) gin.HandlerFunc
```

**Request flow:**

```
HTTP request → middleware.ReverseProxy (FQDN-based; existing)
             → middleware.PathReverseProxy (path-based; new — subprocess plugins)
             → gin router (in-repo .so sites, admin, login, auth, etc.)
```

The `PathReverseProxy` middleware:
1. Checks if `c.Request.URL.Path` starts with any registered subprocess site
   path.
2. If yes: strips the site path prefix, creates a `httputil.NewSingleHostReverseProxy`
   targeting the subprocess upstream, proxies the request, and calls `c.Abort()`.
3. If no: calls `c.Next()` to let the gin router handle it.

The path prefix stripping is necessary because the subprocess plugin mounts its
routes under the site path (e.g. `/sites/docs.jughead.geoffjay.com/install`), but
the proxy should forward the full path so the plugin's router sees the same path.
Actually, **no stripping is needed** — the subprocess plugin listens on the same
site path prefix (it receives `JUGHEAD_SITE_PATH` via env), so the proxy forwards
the full path as-is. This matches how the existing `ReverseProxy` works (it
forwards to `localhost:port/sites/<path>/...`).

### 3.4 Plugin SDK for Subprocess Mode (`sdk/subprocess/`)

Gives plugin developers a clean entry point. The plugin binary calls
`subprocess.Run` with its site definition; the SDK handles the HTTP server
lifecycle, health endpoint, and graceful shutdown.

```go
// sdk/subprocess/subprocess.go

// Site is the subprocess plugin's site declaration. Unlike sdk.Site (which
// uses `any` fields for .so type-assertion), this uses concrete Go types
// because the plugin binary owns them and doesn't cross a process boundary.
type Site struct {
    Path      string
    Theme     string
    Template  templ.Component           // fallback when Routes is nil
    Routes    func(*gin.RouterGroup, string) // site routes
}

// Run starts the plugin's HTTP server and blocks until ctx is cancelled or the
// server receives a shutdown signal. It reads JUGHEAD_PORT for the listen port,
// JUGHEAD_SITE_PATH and JUGHEAD_SITE_THEME for the site config, and registers:
//
//   - GET /health → 200 OK (for host health checks)
//   - <site.Path>/* → site.Routes or site.Template fallback
//   - /static/* → embedded assets if provided
//
// The server uses the same TemplRender wrapper as the host so templ
// components render identically.
func Run(ctx context.Context, site Site) error
```

**Plugin binary entry point:**

```go
// plugins/docs/main.go
package main

import (
    "context"
    "github.com/geoffjay/jughead/sdk/subprocess"
)

func main() {
    site := subprocess.Site{
        Path:  "/sites/docs.jughead.geoffjay.com",
        Theme: "nord",
        Routes: routes.Routes, // func(*gin.RouterGroup, string)
    }
    if err := subprocess.Run(context.Background(), site); err != nil {
        log.Fatal(err)
    }
}
```

The `sdk/subprocess.Site` struct intentionally mirrors `sites.Site` (the host's
internal type) but with concrete Go types instead of `any` fields. The plugin
binary links against `sdk/subprocess`, `templ`, and `gin` directly — it doesn't
need to match the host's versions because no types cross the process boundary.

### 3.5 Process Supervisor (`plugin/supervisor.go`)

Tracks running subprocess plugins, restarts them on crash, and shuts them down
gracefully.

```go
// plugin/supervisor.go

// Supervisor manages the lifecycle of all loaded subprocess plugins. It
// tracks running processes, restarts them on unexpected exit, and shuts them
// down on host shutdown.
type Supervisor struct {
    mu       sync.Mutex
    plugins  []*SubprocessPlugin
    basePort  int
    backoff   time.Duration // initial restart backoff
    maxBackoff time.Duration // max backoff between restarts
}

// NewSupervisor creates a Supervisor with the given base port for subprocess
// allocation.
func NewSupervisor(basePort int) *Supervisor

// Start loads and starts a subprocess plugin from a manifest, health-checks
// it, registers its sites with the SiteManager, and begins monitoring it.
func (s *Supervisor) Start(ctx context.Context, manifestPath string, sm *sites.SiteManager) error

// Watch monitors all managed subprocesses and restarts them on unexpected
// exit. Blocks until ctx is cancelled.
func (s *Supervisor) Watch(ctx context.Context)

// Shutdown gracefully stops all managed subprocesses: sends SIGTERM, waits up
// to gracefulTimeout, then SIGKILL any that haven't exited.
func (s *Supervisor) Shutdown(gracefulTimeout time.Duration) error
```

**Restart logic:**

When a subprocess exits unexpectedly (non-zero exit code, or the process was
killed), the supervisor:
1. Logs the exit with the plugin name and exit error.
2. Waits `backoff` (starting at 1s, doubling up to 30s max).
3. Re-runs `loadSubprocess` (new port allocation, new process).
4. Re-registers the site targets with the `SiteManager`.
5. Resets backoff on successful health check.

**Graceful shutdown:**

On host shutdown (SIGINT/SIGTERM), the supervisor:
1. Sends SIGTERM to each subprocess.
2. Waits up to `gracefulTimeout` (default 10s) for each to exit.
3. Sends SIGKILL to any still running.
4. Logs a summary.

### 3.6 Loader Integration (`plugin/loader.go`)

The existing `LoadAll` function scans the plugins directory for `.so` files. It
is extended to also discover manifest files (`.yml`, `.yaml`) and dispatch to the
subprocess loader.

```go
// plugin/loader.go — updated LoadAll

func loadAll(dir, suffix string, strict bool) error {
    // ... existing directory scan ...

    for _, e := range entries {
        name := e.Name()
        path := filepath.Join(dir, name)

        if strings.HasSuffix(name, ".so") {
            // Existing .so path
            if err := loadOne(path); err != nil { ... }
        } else if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
            // New subprocess path
            if err := supervisor.Start(ctx, path, sm); err != nil { ... }
        }
    }
    // ...
}
```

The `Supervisor` is constructed in `server.go` and passed to `LoadAll` (or
`LoadAll` constructs it internally and returns it for shutdown). The supervisor
needs the `SiteManager` to register subprocess site targets — both are available
in `runServer`.

### 3.7 Host Wiring (`server.go`)

```go
// server.go — updated runServer

func runServer() error {
    // ... existing plugin loading, sites.Initialize(), etc. ...

    // Construct the subprocess supervisor. basePort is configurable via env.
    basePort, _ := strconv.Atoi(gowebly.Getenv("JUGHEAD_PLUGIN_BASE_PORT", "10000"))
    supervisor := plugin.NewSupervisor(basePort)

    // LoadAll now handles both .so and manifest files. The supervisor is
    // passed so manifest-discovered plugins are started as subprocesses.
    if err := plugin.LoadAll(pluginsDir, strict, supervisor, sm); err != nil {
        return fmt.Errorf("load plugins: %w", err)
    }

    // Start the supervisor's watch goroutine (restarts crashed plugins).
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go supervisor.Watch(ctx)

    // Install the path-based proxy middleware BEFORE the gin router's site
    // groups so subprocess plugin requests are proxied before reaching the
    // router.
    router.Use(middleware.PathReverseProxy(sm.SubprocessTargets()))

    // ... existing router setup ...

    // Graceful shutdown: stop all subprocesses on SIGINT/SIGTERM.
    defer func() {
        if err := supervisor.Shutdown(10 * time.Second); err != nil {
            slog.Error("subprocess shutdown error", "error", err)
        }
    }()

    // ... server.ListenAndServe() ...
}
```

### 3.8 `jughead serve` CLI Update (`cmd/jughead/serve/`)

The existing `serve` command loads a `.so` file. It is extended to also support
plugin binaries (executables) directly. When the argument is a `.so`, it uses
the existing `plugin.Open` path. When the argument is a directory containing a
`plugin.yml` manifest, or a manifest file directly, it uses `sdk/subprocess.Run`
to start the plugin in standalone mode.

This unifies the developer experience:
- `jughead serve ./plugins/docs.so` — standalone `.so` (existing).
- `jughead serve ./plugins/docs/plugin.yml` — standalone subprocess plugin
  (new; reads the manifest and starts the binary with `sdk/subprocess.Run`).
- A plugin developer iterating on a subprocess plugin can also just `go run`
  their plugin's `main.go` directly, since `sdk/subprocess.Run` reads
  `JUGHEAD_PORT` and starts the server.

The `cmd/jughead/serve/serve.go` file is refactored so that `serveSite` delegates
to `sdk/subprocess.Run` when the input is a manifest or binary, and to the
existing `loadSitePlugin` + `serveSite` path when it's a `.so`. The
`templRender` and `defaultViewHandler` helpers move into `sdk/subprocess` so
they're shared.

### 3.9 Signed Context Headers (`middleware/plugin_context.go`)

The host passes authenticated user context to subprocess plugins via signed
headers, so plugins can trust the identity without implementing their own auth.

```go
// middleware/plugin_context.go

// SignContext sets signed headers on the proxied request carrying the
// authenticated user's identity. The plugin validates the signature using a
// shared secret (JUGHEAD_PLUGIN_SECRET) and extracts the user/org IDs.
//
// Headers:
//   X-Jughead-User-ID: <uuid>
//   X-Jughead-Org-ID: <uuid>
//   X-Jughead-Timestamp: <unix>
//   X-Jughead-Signature: <hmac-sha256(secret, "user-id|org-id|timestamp")>
func SignContext(secret string) gin.HandlerFunc

// VerifyContext is used by the plugin's SDK to validate the signed headers.
// Returns the user ID and org ID, or an error if the signature is invalid or
// expired.
func VerifyContext(secret string, r *http.Request) (userID, orgID uuid.UUID, err error)
```

The signing middleware runs **after** `AuthRequired` (so the session is already
validated) and **before** `PathReverseProxy` (so the headers are set on the
proxied request). The shared secret is generated by the host on startup and
passed to subprocesses via the `JUGHEAD_PLUGIN_SECRET` env var.

The `sdk/subprocess` package exposes a helper that plugins call to extract the
context:

```go
// sdk/subprocess/context.go

// RequestContext extracts the signed user/org identity from the request
// headers. Returns zero values and no error when the headers are absent
// (the request is unauthenticated — the plugin decides how to handle that).
func RequestContext(secret string, c *gin.Context) (userID, orgID uuid.UUID, err error)
```

## 4. Implementation Sequence

The work is split into six steps, each independently testable and mergeable.

### Step 1: Manifest and subprocess loader (MVP)

**Files created:**
- `plugin/manifest.go` — `Manifest`, `ManifestSite`, YAML parsing.
- `plugin/subprocess.go` — `SubprocessPlugin`, `loadSubprocess`, `HealthCheck`,
  `Stop`.
- `plugin/supervisor.go` — `Supervisor`, `Start`, `Watch`, `Shutdown`.

**Files modified:**
- `plugin/loader.go` — `LoadAll` dispatches `.yml`/`.yaml` to the supervisor.
- `sites/manager.go` — `RegisterSubprocessTarget`, `SubprocessTargets`.
- `middleware/path_proxy.go` — new file, `PathReverseProxy` middleware.
- `server.go` — construct supervisor, install `PathReverseProxy`, wire
  shutdown.

**Test:** a manifest pointing at a simple binary that returns 200 on `/health`
and renders a static HTML page. The host starts it, health-checks it, and proxies
a request through to it.

### Step 2: Plugin SDK (`sdk/subprocess/`)

**Files created:**
- `sdk/subprocess/subprocess.go` — `Site` struct, `Run` function.
- `sdk/subprocess/context.go` — `RequestContext` helper.
- `sdk/subprocess/render.go` — `templRender`, `defaultViewHandler` (moved from
  `cmd/jughead/serve/serve.go`).

**Files modified:**
- `cmd/jughead/serve/serve.go` — delegate to `sdk/subprocess.Run` for binary/
  manifest inputs; keep `.so` path.

**Test:** a sample plugin (`plugins/template/`) using the SDK, started via
`jughead serve ./plugins/template/plugin.yml`.

### Step 3: Process supervision and restart

**Files modified:**
- `plugin/supervisor.go` — implement `Watch` (restart loop with backoff),
  `Shutdown` (graceful SIGTERM → SIGKILL).
- `server.go` — start `Watch` goroutine, wire shutdown into signal handler.

**Test:** kill a subprocess plugin's process mid-request; verify the supervisor
restarts it and the site comes back. Send SIGTERM to the host; verify
subprocesses are stopped gracefully.

### Step 4: Signed context passing

**Files created:**
- `middleware/plugin_context.go` — `SignContext`, `VerifyContext`.

**Files modified:**
- `server.go` — install `SignContext` after `AuthRequired`, generate
  `JUGHEAD_PLUGIN_SECRET`.
- `sdk/subprocess/context.go` — `RequestContext` using `VerifyContext`.

**Test:** login on the host, request a subprocess plugin page, verify the plugin
receives the signed `X-Jughead-User-ID` header and can read the user's identity.

### Step 5: Auth delegation

**Files modified:**
- `sites/manager.go` — when a subprocess site has `Auth`, wrap the proxy
  middleware in the auth provider's `AuthMiddleware` (the host runs the OAuth
  flow; the plugin just receives signed context).
- `plugin/subprocess.go` — pass auth config to the subprocess via env vars
  (`JUGHEAD_AUTH_PROVIDER`, `JUGHEAD_AUTH_REDIRECT_URL`).

**Test:** a subprocess site with `auth: { provider: github }`; verify the host
redirects to GitHub OAuth, and on callback the plugin receives the signed
identity headers.

### Step 6: Documentation and template plugin

**Files created:**
- `plugins/template/` — a scaffolding plugin (main.go, plugin.yml, routes.go,
  go.mod) demonstrating the subprocess SDK.
- `docs/plugin-development.md` — developer guide for writing subprocess plugins.

**Files modified:**
- `docs/plugin-research.md` — link to this plan and mark the subprocess approach
  as "implemented".
- `README.md` — add a "Plugin Development" section linking to the guide.

## 5. Port Allocation Strategy

The supervisor allocates ports sequentially starting from `basePort` (default
10000, configurable via `JUGHEAD_PLUGIN_BASE_PORT`). For each plugin:

1. Try `basePort + offset` (offset starts at 0).
2. Check if the port is free via `net.Listen("tcp", ":<port>")`.
3. If free, close the listener and assign the port.
4. If in use, increment offset and retry.

The allocated port is passed to the plugin via `JUGHEAD_PORT`. The host records
it in the `SubprocessPlugin` struct and uses it for proxy target construction.

On restart, a new port is allocated (the old one may still be in TIME_WAIT). The
`SiteManager`'s subprocess targets are updated with the new port.

## 6. Static Asset Strategy

Subprocess plugins serve their own static assets via `go:embed`. The plugin
binary embeds its `static/` directory and serves it under
`<site.Path>/static/*`. The host's `PathReverseProxy` forwards these requests
transparently.

The host's shared `/static` directory (CSS, favicons, manifest.json) remains
served by the host. The `PathReverseProxy` passes through `/static` requests to
the host (not the plugin), matching the existing `ReverseProxy` behavior
(`middleware/proxy.go:56-59`).

Plugins that want to use the host's shared styles (`/static/styles.css`) simply
reference them by path — the proxy passes them through.

## 7. Logging

The host captures each subprocess's stdout/stderr and logs it under the plugin's
name using `slog`. This gives unified, structured logging without requiring
plugins to use `slog` themselves.

Implementation: the `exec.Cmd`'s `Stdout` and `Stderr` are set to a custom
`io.Writer` that prefixes each line with the plugin name and delegates to
`slog.Info`. A line-buffered scanner reads from the subprocess pipes on a
goroutine.

## 8. Backward Compatibility

- **Existing `.so` plugins are unaffected.** The `loadOne` path is unchanged.
  The `LoadAll` function gains manifest dispatch as an additional branch.
- **Existing built-in sites are unaffected.** `sites.Initialize()` runs as before.
  Subprocess plugins register alongside, not instead of, built-in sites.
- **The `jughead serve` command still works for `.so` files.** The manifest
  path is an additional input format.
- **No breaking changes to `sdk.Plugin` or `sdk.Site`.** The `.so` contract is
  frozen. Subprocess plugins use a separate type (`sdk/subprocess.Site`).
- **The `SiteManager` API gains methods but changes none.** `RegisterSite`,
  `GetSite`, `BuildSiteRoutes`, `BuildProxyTargets` all work as before.
  `RegisterSubprocessTarget` and `SubprocessTargets` are additive.

## 9. Testing Strategy

**Unit tests:**
- `plugin/manifest_test.go` — YAML parsing, validation, missing fields.
- `plugin/subprocess_test.go` — `loadSubprocess` with a mock binary (a tiny Go
  program that starts an HTTP server on a given port and exits on SIGTERM).
- `plugin/supervisor_test.go` — restart on crash, graceful shutdown, port
  allocation.
- `middleware/path_proxy_test.go` — path matching, proxy forwarding, fallthrough.
- `middleware/plugin_context_test.go` — sign/verify roundtrip, expired
  timestamp, invalid signature.
- `sdk/subprocess/subprocess_test.go` — `Run` starts server, `/health` returns
  200, graceful shutdown on context cancellation.

**Integration tests:**
- Start host with a subprocess plugin manifest, request the site through the
  host's proxy, verify rendered HTML.
- Kill the subprocess mid-request, verify 502 then recovery after restart.
- Login on host, request authenticated subprocess site, verify signed headers.
- Send SIGTERM to host, verify subprocesses exit cleanly.

## 10. Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Subprocess crash loses in-flight requests | Proxy returns 502; supervisor restarts; retry budget prevents crash loops |
| Port exhaustion with many plugins | Base port configurable; OS-assigned ports as a fallback (plugin reports port back via health check) |
| Plugin binary missing or wrong architecture | `loadSubprocess` validates binary exists and is executable before starting; health check timeout surfaces startup failures |
| Secret leakage in logs | Signed headers use HMAC, not raw secrets; `JUGHEAD_PLUGIN_SECRET` is never logged |
| Plugin hangs without exiting | Health check timeout (10s default) surfaces hangs; supervisor kills and restarts |
| Path collision between `.so` and subprocess sites | `SiteManager.RegisterSite` checks for existing path; logs a warning and skips the duplicate (same as existing built-in override behavior) |