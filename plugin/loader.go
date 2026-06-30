//go:build linux || darwin

// Package plugin loads runtime .so plugins built against the jughead SDK.
//
// At startup the host calls LoadAll with a directory (JUGHEAD_PLUGINS_DIR).
// Each *.so file is opened with the Go `plugin` package and a symbol named
// "Plugin" of type *sdk.Plugin is looked up. Its Providers are registered into
// the host's auth provider registry (sites/auth) and its Sites are converted
// to sites.Site and registered with the SiteManager. Providers register before
// sites so a plugin site can reference a provider shipped by the same plugin.
//
// A failure on one .so is logged and skipped unless strict mode is on
// (JUGHEAD_PLUGINS_STRICT=1), in which case the first error is returned and
// the server refuses to start.
//
// The host and every plugin must be built with the same Go toolchain version
// and identical dependency versions; a mismatch causes plugin.Open to panic
// with "plugin was built with a different version of package ...". This is a
// hard requirement of the Go `plugin` package, not something the loader can
// mitigate. Build plugins via the Makefile (`make plugins`) against the
// pinned go.mod to keep them in lockstep with the host.
package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffjay/jughead/sdk"
	sdkauth "github.com/geoffjay/jughead/sdk/auth"
	"github.com/geoffjay/jughead/sites"
	"github.com/geoffjay/jughead/sites/auth"

	goplugin "plugin"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

// pluginSuffix is the shared-object extension plugins use on every platform.
// Go's `plugin` package accepts .so files on both Linux and macOS; we
// standardize on .so so the Makefile target and the loader agree regardless
// of host OS. (macOS's conventional .dylib extension is not required by the
// `plugin` package; a file built with `go build -buildmode=plugin -o x.so`
// loads fine on darwin.)
const pluginSuffix = ".so"

// LoadAll opens every plugin file in dir and registers its providers and
// sites. An empty or missing dir is a no-op (logged at debug). When strict is
// true the first per-plugin error is returned immediately; otherwise errors
// are logged and the loader continues to the next file.
//
// Files ending in .so are treated as plugins. (See the pluginSuffix const:
// .so is used on every platform; the Makefile `plugin` target produces .so
// files.)
func LoadAll(dir string, strict bool) error {
	return loadAll(dir, pluginSuffix, strict)
}

func loadAll(dir, suffix string, strict bool) error {
	if dir == "" {
		slog.Debug("plugin loader: no plugins dir configured; skipping")
		return nil
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("plugin loader: plugins dir does not exist; skipping", "dir", dir)
			return nil
		}
		return fmt.Errorf("stat plugins dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("plugins dir %q is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read plugins dir %q: %w", dir, err)
	}

	loaded := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		path := filepath.Join(dir, name)

		if err := loadOne(path); err != nil {
			if strict {
				return fmt.Errorf("plugin %s: %w", name, err)
			}
			slog.Error("plugin loader: failed to load plugin; skipping",
				"file", name, "error", err.Error())
			continue
		}
		loaded++
		slog.Info("plugin loader: loaded plugin", "file", name)
	}

	if loaded > 0 {
		slog.Info("plugin loader: registered plugins", "count", loaded, "dir", dir)
	}
	return nil
}

// loadOne opens a single .so, looks up the Plugin symbol, and registers its
// providers and sites. A panic during plugin.Open or symbol lookup is
// recovered so one bad .so can't crash the loader.
func loadOne(path string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic opening plugin: %v", r)
		}
	}()

	p, err := goplugin.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	sym, err := p.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("lookup Plugin: %w", err)
	}

	pluginPtr, ok := sym.(*sdk.Plugin)
	if !ok {
		return fmt.Errorf("plugin symbol is %T, want *sdk.Plugin", sym)
	}
	if pluginPtr == nil {
		return fmt.Errorf("plugin symbol is nil")
	}
	pl := *pluginPtr

	registerProviders(pl.Name, pl.Providers)
	registerSites(pl.Sites)
	return nil
}

// registerProviders adds each provider to the host's auth registry. Last
// registration wins (auth.Register replaces existing names), so a plugin can
// intentionally override a built-in provider with the same Name().
func registerProviders(pluginName string, providers []sdkauth.Provider) {
	for _, pr := range providers {
		auth.Register(pr)
		slog.Info("plugin loader: registered auth provider",
			"plugin", pluginName, "provider", pr.Name())
	}
}

// registerSites converts each sdk.Site to the host's sites.Site and registers
// it with the SiteManager under its Path. The Template/Proxy/Routes any fields
// are type-asserted against the host's concrete field types; a field that
// doesn't match is left zero (and logged) rather than failing the whole site,
// so a provider-only plugin can ship an empty Sites slice without issue.
func registerSites(sitesIn []sdk.Site) {
	if len(sitesIn) == 0 {
		return
	}
	sm := sites.GetSiteManager()
	for _, s := range sitesIn {
		hs, err := toHostSite(s)
		if err != nil {
			slog.Error("plugin loader: skipping site with invalid field types",
				"path", s.Path, "error", err.Error())
			continue
		}
		sm.RegisterSite(s.Path, hs)
		slog.Info("plugin loader: registered site",
			"path", s.Path, "url", s.Url)
	}
}

// toHostSite converts a plugin's sdk.Site into the host's sites.Site. The
// any-typed fields are asserted against their concrete host types; an
// assertion failure for a populated field is reported so the plugin author
// can fix the mismatch.
func toHostSite(s sdk.Site) (*sites.Site, error) { //nolint:gocritic
	hs := &sites.Site{
		Path:      s.Path,
		Url:       s.Url,
		Published: s.Published,
		Theme:     s.Theme,
		Auth:      s.Auth,
	}

	if s.Template != nil {
		tmpl, ok := s.Template.(templ.Component)
		if !ok {
			return nil, fmt.Errorf("template is %T, want templ.Component", s.Template)
		}
		hs.Template = tmpl
	}
	if s.Proxy != nil {
		proxy, ok := s.Proxy.(func(*gin.Context))
		if !ok {
			return nil, fmt.Errorf("proxy is %T, want func(*gin.Context)", s.Proxy)
		}
		hs.Proxy = proxy
	}
	if s.Routes != nil {
		routes, ok := s.Routes.(func(*gin.RouterGroup, string))
		if !ok {
			return nil, fmt.Errorf("routes is %T, want func(*gin.RouterGroup, string)", s.Routes)
		}
		hs.Routes = routes
	}
	return hs, nil
}
