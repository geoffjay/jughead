# jughead

Jughead is a multi-tenant site platform with a plugin architecture for external
site development. The host server runs an admin UI, database, auth, built-in
sites, and plugins loaded from a directory. A CLI (`jughead`) is provided for
running the host and for scaffolding new plugins.

## Install the CLI

Install the `jughead` CLI with `go install`:

```
go install github.com/geoffjay/jughead@v0.1.0
```

This builds the `jughead` binary and places it in your `$GOBIN` directory
(usually `$(go env GOPATH)/bin`, which should be on your `PATH`). Go 1.25 or
newer is required.

Verify the install:

```
jughead version
```

## Usage

Without a subcommand, `jughead` starts the full application server (admin UI,
database, auth, built-in sites, and plugins from `JUGHEAD_PLUGINS_DIR`):

```
jughead
```

### `jughead serve`

Load a single site plugin (`.so`) and run it as a standalone HTTP server
without the full system — useful for iterating on a site without the database
or admin UI:

```
jughead serve ./plugins/docs.so --port 8080
```

The plugin `.so` must export a package-level `var Plugin = sdk.Plugin{...}`
symbol, and the host and plugin must be built with the same Go toolchain and
dependency versions (see the Go `plugin` package's type-identity constraint).

### `jughead init`

Bootstrap a minimal plugin scaffold (a site or an auth-provider) from a
template. Without flags it is interactive and prompts for the plugin name,
kind, and target directory:

```
jughead init
```

Non-interactive usage:

```
jughead init --plugin site --name foo path/to/foo
jughead init --plugin provider --name bar path/to/bar
```

A custom template directory can be supplied via `--template`. It must contain
subdirectories named `site` and `provider` mirroring the default templates; the
selected kind's subdirectory is rendered into the target path:

```
jughead init --plugin site --template path/to/custom/template/ path/to/foo
```

The default templates ship inside the `jughead` binary (under
`templates/init/{site,provider}` in the source tree) and are rendered with the
Go `text/template` syntax. The following fields are available:

| Field            | Description                                              |
| ---------------- | -------------------------------------------------------- |
| `.Name`          | Plugin name (e.g. `foo`).                                |
| `.SitePath`      | URL path the site is served under (`/sites/<name>`).     |
| `.TitleName`     | PascalCase form of `.Name` (e.g. `foo-bar` → `FooBar`).  |
| `.GoVersion`     | Go toolchain version written into the generated go.mod.  |
| `.JugheadVersion`| jughead module version written into the generated go.mod.|

After scaffolding, edit the generated `go.mod` to pin the jughead version you
built the host against, then build the plugin:

```
CGO_ENABLED=1 go build -buildmode=plugin -o foo.so .
```

Run it standalone with `jughead serve ./foo.so`, or place it in the directory
named by `JUGHEAD_PLUGINS_DIR` and start the full host.

## License

See the project repository for license information.