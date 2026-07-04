# Plugin Architecture Research

_Date: 2026-07-04 · Scope: alternatives to Go's `plugin` package for external site development_

## 1. Problem Statement

Jughead's current plugin system uses Go's `plugin` package to load `.so` files at
runtime. This imposes a hard constraint: **the host binary and every plugin must be
built with the same Go toolchain version and identical dependency versions** (same
`go.mod`). A mismatch causes `plugin.Open` to panic with "plugin was built with a
different version of package ...".

This is a fundamental limitation of Go's `plugin` package, not something the loader
can mitigate. It creates several practical problems:

- **External plugin developers** must pin their `go.mod` to exact versions matching
  the host. A host update silently breaks all existing plugins until they recompile.
- **Distribution friction**: plugins cannot be pre-compiled and distributed as
  portable artifacts. They must be compiled against the host's exact module graph.
- **No language choice**: plugins must be written in Go. Sites that are purely
  content-driven (documentation, marketing) pay the full Go compilation cost.
- **Crash isolation**: a panicking plugin takes down the entire host process.

The goal of this research is to identify an alternative plugin architecture that
preserves the current system's capabilities (sites registering routes, rendering
templ components, using auth providers) while eliminating the toolchain lock-in.

## 2. Current Architecture Summary

The existing system works as follows:

```
Host startup (server.go)
  → plugin.LoadAll(JUGHEAD_PLUGINS_DIR)        (plugin/loader.go)
      → for each *.so:
          → plugin.Open(path)                   (Go runtime .so loader)
          → plugin.Lookup("Plugin")            (well-known symbol)
          → type-assert to *sdk.Plugin
          → registerProviders(pl.Providers)     (into sites/auth registry)
          → registerSites(pl.Sites)             (into sites.SiteManager)
  → sites.Initialize()                          (built-in sites, may be overridden by plugins)
  → SiteManager.BuildSiteRoutes(router, ...)   (mounts each site's routes on gin)
```

The `sdk.Plugin` struct (`sdk/plugin.go`) is the contract:

```go
type Plugin struct {
    Name      string
    Providers []auth.Provider
    Sites     []Site
}

type Site struct {
    Path      string
    Url       string
    Published bool
    Theme     string
    Template  any // host asserts → templ.Component
    Proxy     any // host asserts → func(*gin.Context)
    Routes    any // host asserts → func(*gin.RouterGroup, string)
    Auth      *auth.AuthConfig
}
```

The `any`-typed fields exist because Go's `plugin` package requires byte-identical
type identities across the `.so` boundary — the SDK avoids importing templ/gin at
the top level so plugin authors don't need to match exact versions, but the host
still type-asserts against its own concrete types at load time.

A `jughead serve <plugin.so>` CLI command was recently added (`cmd/jughead/serve/`)
that loads a single plugin and runs it standalone — useful for development, but the
same toolchain constraint applies.

## 3. Alternatives Evaluated

### 3.1 Subprocess + HTTP Reverse Proxy

**Mechanism:** Each plugin is a standalone HTTP server binary. The host starts
each plugin as a managed subprocess on a local port and reverse-proxies matching
requests to it.

**Lifecycle (jughead-managed):**

```
Host startup
  → for each plugin in plugins dir:
      → start plugin binary as subprocess (cmd/exec)
      → wait for plugin to bind its port (health check)
      → add {site-path → localhost:port} to proxy targets
  → middleware.ReverseProxy forwards matching requests to plugin subprocesses
```

**Fit for jughead: Very High.** The infrastructure already exists:

- `middleware.ReverseProxy` (`middleware/proxy.go`) already proxies FQDN-based
  requests to localhost upstreams. The same mechanism proxies path-based requests
  to plugin subprocesses.
- `SiteManager.BuildProxyTargets` (`sites/manager.go:57`) already builds
  `{host: localhost:port/sites/path}` maps. Extending this to include subprocess
  plugins is a natural fit.
- The `jughead serve` command (`cmd/jughead/serve/serve.go`) already demonstrates
  that a plugin site can run as a standalone HTTP server with gin + templ — the
  standalone server is 90% of what a subprocess plugin would be.

**How sites work under this model:**

- The plugin binary starts its own gin router, registers its own routes, renders
  its own templ components, and serves its own HTML.
- The host proxies requests to the plugin; the plugin renders and returns HTML.
- Auth: the host can wrap the proxy with auth middleware (session cookie check)
  before forwarding, or the plugin can handle auth itself. Provider configuration
  (OAuth client IDs, etc.) can be passed via environment variables to the
  subprocess.
- Static assets: the host serves `/static` centrally; plugins reference
  `/static/...` paths that resolve to the host. Alternatively, plugins can serve
  their own assets under their path prefix.

**Pros:**
- **No toolchain lock-in.** Plugin binaries can be built with any Go version, any
  dependencies, even different languages (the boundary is HTTP, not Go types).
- **Crash isolation.** A panicking plugin kills only its subprocess; the host
  detects the exit and can restart it.
- **Language-agnostic.** A site plugin could be written in Rust, Python, or Node —
  the host doesn't care, it just proxies HTTP.
- **Leverages existing infrastructure.** The reverse-proxy middleware and
  `jughead serve` already work. The transition is incremental.
- **Natural development workflow.** Plugin developers run `jughead serve` (or
  `go run .`) to test locally; in production, the host manages the subprocess.
  Same code, different lifecycle.

**Cons:**
- **No shared in-memory state.** Session data, auth context, and request-scoped
  state must be passed via HTTP headers or cookies, not Go context values. The
  host would set a signed session header that the plugin validates.
- **Process management.** The host must start, monitor, and restart subprocesses.
  Need a process supervisor (health checks, graceful shutdown, restart-on-crash,
  port allocation).
- **Port allocation.** Each subprocess needs a port. The host allocates ports
  (e.g., starting at 10000) and passes them to the plugin via env var or arg.
- **Static asset strategy.** Plugins that ship their own CSS/JS need either
  embedded assets (go:embed) or a convention for the host to serve them.

**Migration path:**

1. Define a plugin manifest (YAML or Go struct) declaring the site's path, URL,
   port, and binary path.
2. The loader gains a `loadSubprocess(manifest)` path alongside `loadOne(.so)`.
3. The host starts the subprocess, health-checks it, and registers it as a proxy
  target.
4. Keep the `.so` loader for in-repo plugins (where toolchain lock-in is a
  non-issue) and use subprocess plugins for external/out-of-repo distribution.
5. The `sdk.Site` struct gains a `Serve(ctx context.Context, port int) error`
  method that starts the HTTP server — the same code path used by `jughead serve`
  and by the host's subprocess launcher.

### 3.2 Hashicorp go-plugin (gRPC over subprocess pipes)

**Mechanism:** Each plugin is a separate binary communicating via gRPC over
stdin/stdout pipes. The host starts the subprocess; the plugin speaks the gRPC
protocol over its stdin/stdout; the host calls RPC methods to render pages,
register routes, etc. Used by Terraform, Packer, Vault, and Nomad.

**Fit for jughead: Medium-High.** The `sdk.Plugin` struct would become a gRPC
service definition. Providers map cleanly (they're already interface-based).
Sites are harder: `templ.Component` can't cross a gRPC boundary, so the plugin
would render to HTML bytes and stream them back. The host's `siteViewHandler`
would call the plugin's `Render` RPC and write the returned HTML.

**Pros:**
- **Battle-tested.** Used by HashiCorp's entire product line. Handles subprocess
  lifecycle, health checks, graceful shutdown, version negotiation.
- **No toolchain lock-in.** Plugin binaries can use any Go version or any
  language with a gRPC implementation.
- **Structured protocol.** gRPC gives typed request/response, streaming, and
  bidirectional communication. Richer than raw HTTP proxying.
- **Built-in process management.** The go-plugin library handles subprocess
  lifecycle, restart, and log capture.

**Cons:**
- **gRPC adds complexity.** Need to define a proto schema, generate Go stubs,
  maintain the proto as the contract (instead of the current Go struct).
- **Sites lose direct gin access.** The plugin can't hold a `*gin.RouterGroup`
  — route registration becomes declarative (plugin declares routes via gRPC,
  host registers them) rather than imperative.
- **No shared templ runtime.** The plugin must render its own templ components
  to HTML and return bytes. The host can't pass `templ.Component` over gRPC.
- **Heavier SDK.** Plugin authors must learn the go-plugin library, gRPC
  basics, and the proto schema. More ceremony than a Go struct.

### 3.3 WebAssembly (wazero / Extism)

**Mechanism:** Plugins compile to WASM modules loaded by a pure-Go WASM runtime
(wazero). Plugins can be written in Go, Rust, C, or any language targeting WASM.
The host calls exported WASM functions; data crosses the boundary as bytes.

**Fit for jughead: Medium.** The WASM boundary is bytes-only — no Go types cross
over. A `render(request_json) -> html_bytes` function signature would replace
the current `templ.Component` + `func(*gin.RouterGroup, string)` contract.

**Pros:**
- **Sandboxed.** Memory and CPU limits are enforced by the runtime. A
  misbehaving plugin can't crash or exhaust the host.
- **Truly portable.** WASM modules are architecture-independent. A plugin
  compiled once runs on any platform with a WASM runtime.
- **Fast.** Near-native execution speed (wazero is a JIT-less interpreter;
  performance is good but below native Go).
- **No process management.** Modules load in-process; no subprocess lifecycle.

**Cons:**
- **No shared libraries.** Each plugin statically links everything it needs
  (templ runtime, gin, phosphor icons, daisyUI components). Plugin sizes grow
  significantly.
- **templ doesn't target WASM cleanly.** templ generates Go code that calls
  into the Go `text/template` runtime; the generated code needs the full Go
  runtime available in the WASM module. This works but produces large modules.
- **Limited ecosystem maturity.** Go-to-WASM is functional but not as polished
  as Go-to-native or Go-to-subprocess. Debugging, profiling, and error
  messages are less ergonomic.
- **No shared types.** The current SDK's `sdk.Site` struct, `auth.Provider`
  interface, and `templ.Component` type can't cross the WASM boundary. The
  plugin contract becomes a JSON schema, not a Go API.

### 3.4 Embedded Scripting (yaegi / goja / tamarin)

**Mechanism:** Plugins are scripts interpreted at runtime by an embedded
interpreter. Yaegi interprets Go source; goja interprets JavaScript; tamarin
is a Go-flavored scripting language.

**Fit for jughead: Low.** Scripting engines can't run templ components (templ
generates Go code needing compilation) or gin handlers (Go closures). This
would require replacing the plugin model with a declarative config + template
format, losing the power of the current Go-based SDK.

**Pros:**
- **Zero compilation.** Plugins are text files loaded at runtime.
- **Hot-reloadable.** Edit a script and reload without recompiling.
- **Non-developers can write plugins.** Lower barrier to entry.

**Cons:**
- **Can't reuse templ/gin stack.** The core value proposition (Go-typed site
  definitions with templ components) is lost. Plugins become config files,
  not code.
- **Performance overhead.** Interpretation is slower than native execution.
- **Security.** Arbitrary code execution in the host process is risky without
  sandboxing (which scripting engines don't provide well).
- **Expressiveness.** Scripting languages can't express the full range of Go
  patterns the current SDK uses (interfaces, struct embedding, closures over
  request context).

### 3.5 Docker Build Container

**Mechanism:** Ship a versioned Docker image containing the exact Go toolchain
and jughead module version. Plugin developers mount their plugin source into the
container and build a `.so` that is guaranteed-compatible with the corresponding
jughead release. The `.so` loader and runtime mechanism are unchanged; the
container only standardizes the build environment.

**Usage model:**

```bash
# Jughead releases a builder image per version
docker run --rm -v ./my-plugin:/src jughead/plugin-builder:v1.2.0
# Produces /src/my-plugin.so, guaranteed to load on jughead v1.2.0
```

**Fit for jughead: High as a bridge, not as the long-term answer.** The container
makes the existing `.so` approach far more manageable for in-repo and
closely-coupled plugins, but it does not change the underlying mechanism — the
host and plugin are still coupled at the type-identity level.

**What it solves:**
- **Build reproducibility.** The container pins the Go toolchain, the jughead
  module, and all transitive dependencies. Plugin developers get a
  guaranteed-compatible build environment without manually matching versions.
- **Developer onboarding.** A plugin developer runs one `docker run` command and
  gets a compatible `.so` out. No reading go.mod files, no guessing toolchain
  versions, no "which Go version does jughead use?" question.
- **Versioned releases.** Each jughead release ships a corresponding builder
  image tagged with the version. Plugins built against
  `jughead/plugin-builder:v1.2.0` are guaranteed to load on jughead `v1.2.0`.
  The tag becomes the compatibility contract.
- **CI integration.** A plugin's CI pipeline can pull the builder image, compile
  the `.so`, and publish it as an artifact — fully reproducible across
  developer machines and CI runners.

**What it doesn't solve:**
- **Every host update still breaks all plugins.** The container makes rebuilding
  trivial, but the coupling remains — a host upgrade requires every plugin to be
  rebuilt in the new container. The subprocess approach (3.1) removes this coupling
  entirely; a plugin binary built once keeps working across host upgrades.
- **Platform lock-in for `.so` files.** Go's `plugin` package produces
  platform-specific binaries. A `.so` built in a Linux container only loads on a
  Linux host. If a developer runs jughead natively on macOS for local
  development, container-built `.so` files won't load — the host itself would
  need to run in Docker, or per-platform builder images (linux/amd64,
  linux/arm64, darwin/arm64) would need to be shipped. This is a real constraint
  for macOS-based development.
- **Runtime fragility persists.** The Go `plugin` package's type-identity check
  is a runtime verification, not a build-time one. A transitive dependency that
  updates between the container build and the host build (even with the same major
  version) still panics at `plugin.Open`. The container reduces the probability
  to near-zero for a given release, but the failure mode is still a panic that
  kills the host process.
- **Distribution is still coupled.** The container solves the build problem but
  not the distribution problem — a `.so` built for `v1.2.0` won't load on
  `v1.3.0`. Plugin authors must rebuild and re-release for every host version.
  The subprocess approach decouples this: a plugin binary built once keeps
  working because the boundary is HTTP, not Go type identity.

**Role in the overall strategy:**
- **In-repo plugins** (docs site, github provider): a builder image is
  immediately useful. Toolchain lock-in is a non-issue when you control both sides,
  and a container makes CI and contributor onboarding trivial. Low effort, high
  value.
- **External plugins**: the builder is a practical bridge that improves the
  current experience while the subprocess architecture is being designed and
  implemented. It doesn't replace the need for subprocess plugins — it makes the
  interim period manageable.

**Pros:**
- **Immediate.** A Dockerfile + a build script can be shipped in a single PR. No
  architectural changes, no new SDK, no protocol design.
- **Versioned.** The image tag becomes the compatibility contract — clear and
  auditable.
- **Zero code changes.** The `.so` loader, the SDK, and the host are all
  unchanged. The container is purely a build-time tool.
- **Familiar.** Docker is already in the stack (docker-compose.yml exists for
  postgres). A builder image is a natural extension.

**Cons:**
- **Doesn't address runtime fragility.** The fundamental limitation (Go
  `plugin`'s type-identity check) is unchanged. A panic in `plugin.Open` still
  crashes the host.
- **Platform-specific `.so` output.** Linux container produces Linux `.so`
  files; macOS developers need a separate path or must run the host in Docker.
- **Still requires per-version rebuilds.** The coupling is managed, not removed.
- **Not a long-term answer for external distribution.** The subprocess approach
  is still needed for plugins that should survive host upgrades without
  recompilation.

## 4. Recommendation

**Adopt Option 3.1 (subprocess + HTTP reverse proxy)** as the primary external
plugin architecture, retaining the `.so` loader for in-repo plugins. **Ship
Option 3.5 (Docker build container) immediately** as a practical bridge that
improves the `.so` experience while the subprocess architecture is being
implemented.

Rationale:

1. **Aligns with existing architecture.** The reverse-proxy middleware
   (`middleware/proxy.go`), the `SiteManager.BuildProxyTargets` method, and the
   `jughead serve` CLI command already implement 90% of what's needed. The
   transition is incremental, not a rewrite.

2. **Jughead manages the lifecycle.** The host starts plugin subprocesses,
   health-checks them, and restarts them on crash. Plugin developers don't run
   anything separately — jughead is the environment, as intended.

3. **No toolchain lock-in.** Plugin binaries can be built with any Go version,
   any dependencies, or even other languages. This is the primary problem we're
   solving.

4. **Crash isolation.** A failing plugin is a dead subprocess, not a panicked
   host. The host can detect the exit, log it, and restart.

5. **Developer experience.** The `jughead serve <binary>` command already lets
   plugin developers test standalone. In production, the same binary is managed
   by the host. One codebase, two modes.

6. **Lowest disruption to existing plugins.** In-repo plugins (docs, github
   provider) keep using the `.so` loader where toolchain lock-in is a non-issue.
   External plugins use the subprocess path. Both coexist in the same host.

## 5. Implementation Plan

### Phase 0: Docker build container (immediate bridge)

**Goal:** Ship a versioned Docker image that plugin developers use to build
`.so` files guaranteed-compatible with a given jughead release. No
architectural changes — purely a build-time tool that makes the existing `.so`
approach manageable.

1. **Create `Dockerfile.plugin-builder`** that pins:
   - The Go toolchain version matching the jughead release.
   - The jughead module source at the release tag (vendored or via `go mod
     download`).
   - Build tooling (`templ generate`, `task`).
2. **Publish `jughead/plugin-builder:<version>`** to a registry (GitHub Container
   Registry) on each release. The version tag is the compatibility contract: a
   `.so` built with `:v1.2.0` loads on jughead `v1.2.0`.
3. **Add a `task plugin:build` target** that runs the builder container:
   ```bash
   docker run --rm -v $(pwd)/my-plugin:/src jughead/plugin-builder:$(JUGHEAD_VERSION)
   ```
4. **Document the workflow** for external plugin developers: clone the plugin
   template, write the site, build with the container image matching the target
   jughead version, ship the `.so`.

This phase is independent of and does not block the subprocess architecture
below. It improves the current experience immediately.

### Phase 1: Plugin manifest and subprocess loader (MVP)

**Goal:** Load an external site plugin binary as a subprocess and proxy to it.

1. **Define a plugin manifest format.** A small YAML or JSON file alongside the
   plugin binary declaring:
   ```yaml
   name: docs
   type: subprocess
   binary: ./docs-plugin
   sites:
     - path: /sites/docs.jughead.geoffjay.com
       url: https://docs.jughead.geoffjay.com
       theme: nord
       published: true
   ```
   The manifest replaces the `sdk.Plugin` Go struct for external plugins. It
   declares what the host needs to know (path, URL, theme) without requiring the
   host to load Go types.

2. **Add a subprocess loader** (`plugin/subprocess.go`) that:
   - Reads the manifest.
   - Allocates a port (starting from a configurable base, e.g. 10000).
   - Starts the binary as a subprocess with `exec.Command`, passing the port and
     site config via env vars (`JUGHEAD_PORT`, `JUGHEAD_SITE_PATH`,
     `JUGHEAD_SITE_THEME`).
   - Health-checks the subprocess (HTTP GET `/health` or TCP connect) with a
     timeout.
   - On success, registers the site as a proxy target in the `SiteManager`.

3. **Extend `SiteManager`** to accept subprocess-backed sites. A
   `SubprocessSite` type wraps the manifest data and the subprocess handle. It
   satisfies the `*sites.Site` interface for routing purposes but delegates
   rendering to the proxy.

4. **Update `plugin.LoadAll`** to dispatch on manifest type: `.so` files use the
   existing `loadOne` path; manifest files (`.yml` / `.json`) use the new
   subprocess loader. Both register into the same `SiteManager`.

5. **Add process supervision** to the host:
   - Track subprocess handles in a registry.
   - On shutdown, send SIGTERM to each and wait for exit.
   - On crash, log the error and restart (with backoff).

### Phase 2: Plugin SDK for subprocess mode

**Goal:** Give plugin developers a clean SDK for writing subprocess plugins.

1. **Create `sdk/subprocess` package** with:
   - A `Run(ctx context.Context, site Site) error` function that starts a gin
     server on the port from `JUGHEAD_PORT`, registers the site's routes, and
     blocks until the context is cancelled.
   - A `Site` struct mirroring `sdk.Site` but with concrete Go types
     (`templ.Component`, `func(*gin.RouterGroup, string)`) since the plugin
     binary owns these types and doesn't need to cross a `.so` boundary.
   - Helper for health-check endpoint (`GET /health` returns 200).

2. **Plugin binary entry point** becomes:
   ```go
   package main

   import "github.com/geoffjay/jughead/sdk/subprocess"

   func main() {
       subprocess.Run(ctx, subprocess.Site{
           Path: "/sites/docs.jughead.geoffjay.com",
           Theme: "nord",
           Routes: routes.Routes,
       })
   }
   ```

3. **The `jughead serve` CLI** is updated to use `sdk/subprocess.Run` internally
   so the standalone and host-managed code paths are identical.

### Phase 3: Shared context passing

**Goal:** Pass session/auth context from host to plugin subprocesses.

1. **Signed context header.** The host sets a signed header (e.g.
   `X-Jughead-User-ID: <uuid>`, `X-Jughead-Org-ID: <uuid>`) on proxied requests,
   signed with a shared secret. The plugin validates the signature and extracts
   the user/org context.

2. **Auth provider delegation.** When a site declares `Auth.Provider = "github"`,
   the host runs the OAuth flow (as it does now) and passes the authenticated
   user identity to the plugin via the signed header. The plugin doesn't need
   its own OAuth implementation — the host handles auth and the plugin trusts the
   host's signed context.

3. **Static asset serving.** Plugins that ship their own CSS/JS use `go:embed`
   and serve assets under their path prefix (e.g.
   `/sites/docs.jughead.geoffjay.com/static/...`). The host's reverse proxy
   forwards these requests; the plugin serves embedded assets directly.

### Phase 4: Migration and documentation

1. **Update the plugin docs** to describe both modes:
   - In-repo `.so` plugins: for built-in sites and providers.
   - Subprocess plugins: for external/third-party site development.
2. **Add a plugin template** (`plugins/template/`) that scaffolds a subprocess
   plugin with the SDK, manifest, and a sample route.
3. **Deprecation path.** The `.so` loader remains supported indefinitely for
   in-repo use. External plugins are directed to the subprocess path. No forced
   migration.

## 6. Open Questions

- **Port allocation strategy:** sequential, OS-assigned, or configured per
  plugin in the manifest? Sequential is simplest but risks collisions;
  OS-assigned requires the plugin to report its port back to the host.
- **Log capture:** should plugin stdout/stderr be captured by the host and
  logged under the plugin's name, or should plugins log directly? Capturing
  gives unified logging; direct logging is simpler for development.
- **Health check protocol:** HTTP `/health`, TCP connect, or a gRPC health
  check? HTTP is simplest and aligns with the HTTP-proxy model.
- **Hot reload:** should the host watch the plugins dir for new manifests and
  start/stop subprocesses without restarting? Useful but adds complexity.
- **Graceful shutdown:** SIGTERM with a timeout, then SIGKILL? How long to wait
  for in-flight requests?
- **Multi-site plugins:** a single binary serving multiple sites (multiple
  ports, or one port with path-based routing)? The current `.so` model supports
  `[]Site`; the subprocess model could too if the binary registers multiple
  path prefixes on one port.