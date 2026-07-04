// Package serve implements the `jughead serve` subcommand: it loads a single
// site plugin (.so) and runs it as a standalone HTTP server without the full
// jughead application (no database, no admin UI, no built-in sites, no auth
// provider registry). This is designed for site developers who want to iterate
// on a site plugin without running the entire system.
//
// The .so must export a `Plugin` symbol of type *sdk.Plugin (the same contract
// the full host's loader expects). Only the first site in the plugin's
// Sites slice is served; if the plugin ships multiple sites, use the full
// jughead server instead.
//
// The host and plugin MUST be built with the same Go toolchain and dependency
// versions (same go.mod), because the Go `plugin` package requires
// byte-identical type identities across the .so boundary. This is a
// fundamental constraint of Go's plugin system, not a limitation of the CLI.
package serve

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	goplugin "plugin"

	"github.com/geoffjay/jughead/sdk"
	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"github.com/spf13/cobra"
)

const (
	defaultPort  = 9000
	readTimeout  = 5 * time.Second
	writeTimeout = 10 * time.Second
)

// Cmd is the cobra command for `jughead serve`.
var Cmd = &cobra.Command{
	Use:   "serve <plugin.so>",
	Short: "Run a single site plugin standalone (no database or admin UI)",
	Long: `Load a site plugin (.so) and serve it as a standalone HTTP server.

This is useful for site developers who want to iterate on a site without
running the full jughead system (database, admin UI, built-in sites, auth).

The plugin .so must export a package-level ` + "`var Plugin = sdk.Plugin{...}`" + `
symbol, the same contract the full host's plugin loader expects. Only the
first site in the plugin's Sites slice is served.

The host and plugin must be built with the same Go toolchain and dependency
versions (same go.mod), because Go's ` + "`plugin`" + ` package requires
byte-identical type identities across the .so boundary.

Example:
  jughead serve ./plugins/docs.so --port 8080`,
	Args: cobra.ExactArgs(1),
	RunE: run,
}

var port int

// SetupFlags registers the serve command's flags. Called by main before
// Execute so the flags are available for parsing. This avoids an init()
// function (the project's linter forbids them).
func SetupFlags() {
	Cmd.Flags().IntVarP(&port, "port", "p", defaultPort, "port to listen on")
}

func run(_ *cobra.Command, args []string) error {
	soPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve plugin path: %w", err)
	}

	if _, err := os.Stat(soPath); err != nil {
		return fmt.Errorf("plugin file not found: %w", err)
	}

	slog.Info("Loading site plugin", "path", soPath)
	site, err := loadSitePlugin(soPath)
	if err != nil {
		return fmt.Errorf("load plugin: %w", err)
	}

	slog.Info("Site loaded",
		"path", site.Path,
		"url", site.Url,
		"theme", site.ThemeValue(),
	)

	return serveSite(site, port)
}

// loadSitePlugin opens a .so file, looks up the Plugin symbol, and returns the
// first site converted to the host's sites.Site type. It mirrors the logic in
// plugin/loader.go's loadOne + toHostSite but operates on a single plugin and
// does not touch the global SiteManager or auth registry.
func loadSitePlugin(soPath string) (*sites.Site, error) {
	p, err := goplugin.Open(soPath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	sym, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("lookup Plugin: %w", err)
	}

	pluginPtr, ok := sym.(*sdk.Plugin)
	if !ok {
		return nil, fmt.Errorf("plugin symbol is %T, want *sdk.Plugin", sym)
	}
	if pluginPtr == nil {
		return nil, fmt.Errorf("plugin symbol is nil")
	}

	pl := *pluginPtr
	if len(pl.Sites) == 0 {
		return nil, fmt.Errorf("plugin %q has no sites", pl.Name)
	}

	// Take the first site. The standalone server handles one site at a time;
	// multi-site plugins should use the full jughead server.
	s := &pl.Sites[0]
	return convertSite(s)
}

// convertSite type-asserts the sdk.Site's any-typed fields against the host's
// concrete types, the same way plugin/loader.go's toHostSite does. This is
// duplicated rather than imported from plugin/ because the plugin package's
// loader registers into the global SiteManager (which we explicitly don't want
// for the standalone server).
func convertSite(s *sdk.Site) (*sites.Site, error) {
	hs := &sites.Site{
		Path:      s.Path,
		Url:       s.Url,
		Published: s.Published,
		Theme:     s.Theme,
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

// serveSite runs a minimal gin server that serves the single site under its
// configured Path. No database, no admin UI, no auth providers, no reverse
// proxy — just the site's routes (or its Template as a fallback).
func serveSite(site *sites.Site, port int) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Minimal TemplRender so templ components render through gin's HTMLRender.
	// The full host uses the same wrapper (see server.go).
	router.HTMLRender = &templRender{}

	// Serve static assets from ./static if present (site templates may
	// reference /static/...). Non-existent dir is silently skipped.
	if _, err := os.Stat("./static"); err == nil {
		router.Static("/static", "./static")
	}

	// Mount the site at its Path (the same prefix the full server uses) so
	// the site's Routes callback works identically. Some plugins register
	// both GET "" and GET "/" which collide at root level but are fine under
	// a group prefix. A redirect from "/" to the site path is added so the
	// developer can browse to http://localhost:port/ and land on the site.
	group := router.Group(site.Path)
	if site.Routes != nil {
		site.Routes(group, site.ThemeValue())
	} else {
		group.GET("", defaultViewHandler(site))
		group.GET("/", defaultViewHandler(site))
	}

	// Redirect root to the site path for convenience.
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, site.Path)
	})

	addr := fmt.Sprintf(":%d", port)
	slog.Info("Starting standalone site server", "port", port, "site", site.Path)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		Handler:      router,
	}
	return server.ListenAndServe()
}

// defaultViewHandler renders the site's Template as a fallback when Routes is
// nil. Mirrors the siteViewHandler in handlers.go but without the SiteManager
// lookup (we already have the site).
func defaultViewHandler(site *sites.Site) gin.HandlerFunc {
	return func(c *gin.Context) {
		if site.Template == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, site.Template); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}
}

// templRender is a minimal gin.Render implementation for templ components,
// equivalent to the TemplRender in server.go. Duplicated here to keep the
// standalone server self-contained without importing the main package.
type templRender struct{}

func (t *templRender) Instance(_ string, value any) render.Render {
	if templData, ok := value.(templ.Component); ok {
		return &templComponent{Code: http.StatusOK, Data: templData}
	}
	return nil
}

type templComponent struct {
	Code int
	Data templ.Component
}

func (t *templComponent) Render(w http.ResponseWriter) error {
	t.WriteContentType(w)
	w.WriteHeader(t.Code)
	if t.Data != nil {
		return t.Data.Render(context.Background(), w)
	}
	return nil
}

func (t *templComponent) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

// Ensure the sessions package is referenced — it's needed because plugins may
// import types from it transitively, and the standalone binary must link it
// identically to the host so the .so loads. This is a no-op reference.
var _ = sessions.CookieName
