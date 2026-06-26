package sites

import (
	"fmt"
	"log/slog"
	"net/url"
	"sync"

	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites/auth"

	"github.com/gin-gonic/gin"
)

type SiteManager struct {
	sites map[string]*Site
}

var (
	lock     = &sync.Mutex{}
	instance *SiteManager
)

func GetSiteManager() *SiteManager {
	if instance == nil {
		lock.Lock()
		defer lock.Unlock()
		if instance == nil {
			instance = &SiteManager{
				sites: make(map[string]*Site),
			}
		}
	}

	return instance
}

func (sm *SiteManager) RegisterSite(path string, site *Site) {
	sm.sites[path] = site
}

func (sm *SiteManager) GetSite(path string) *Site {
	return sm.sites[path]
}

func (sm *SiteManager) Sites() map[string]*Site {
	return sm.sites
}

// BuildProxyTargets builds the {FQDN-host: localhost:port/sites/<path>} map
// consumed by middleware.ReverseProxy. Each loaded site contributes an entry
// mapping the host portion of its Url to the localhost upstream that serves
// the site under its /sites/<path> group.
//
// Sites whose Url is empty or unparseable are skipped with a warning, since
// they can't be reached via a host-based reverse proxy anyway.
func (sm *SiteManager) BuildProxyTargets(port int) map[string]string {
	targets := make(map[string]string, len(sm.sites))
	for _, site := range sm.sites {
		host := hostFromURL(site.Url)
		if host == "" {
			slog.Warn("site has no usable host for reverse proxy; skipping",
				"path", site.Path, "url", site.Url)
			continue
		}
		targets[host] = fmt.Sprintf("http://localhost:%d%s", port, site.Path)
	}
	return targets
}

// hostFromURL extracts the host (no port, no scheme, no path) from a site
// URL. Returns "" when u is empty or can't be parsed.
func hostFromURL(u string) string {
	if u == "" {
		return ""
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Hostname()
}

// BuildSiteRoutes iterates the loaded sites and wires each onto the router,
// applying auth middleware and registering provider OAuth routes where a
// site's Auth config requests it. It replaces the per-site special-casing
// that previously lived in server.go.
//
// defaultView is the fallback handler used when a site has neither Routes nor
// a working auth provider (e.g. the GitHub provider with missing env config):
// the site still responds, rendering its default Template, instead of 404ing.
//
// Per site:
//   - When site.Auth is nil the site is public: site.Routes is invoked if set,
//     otherwise defaultView is registered.
//   - When site.Auth is non-nil the named provider is looked up in the
//     sites/auth registry and Load is called with the site's AuthConfig.
//     On success: the provider's /auth/<provider>/* routes are mounted on the
//     root router once (deduped by provider name across sites), the site's
//     group is wrapped in the provider's AuthMiddleware, and site.Routes (or
//     defaultView) is registered on the protected group.
//     On error (incomplete config, unknown provider): the site falls back to
//     defaultView so it renders its Template (e.g. a sign-in prompt) instead
//     of mounting protected routes.
//
// The router is accepted as gin.IRouter so both *gin.Engine (production) and a
// test router group work.
func (sm *SiteManager) BuildSiteRoutes(router gin.IRouter, store *sessions.Store, defaultView gin.HandlerFunc) {
	seenProviders := make(map[string]bool)

	for _, site := range sm.sites {
		group := router.Group(site.Path)

		if site.Auth == nil {
			registerSiteGroup(group, site, defaultView)
			continue
		}

		// Auth requested: look up the provider.
		provider, ok := auth.Get(site.Auth.Provider)
		if !ok {
			slog.Warn("auth provider not registered; site falls back to default view",
				"path", site.Path, "provider", site.Auth.Provider)
			group.GET("", defaultView)
			group.GET("/", defaultView)
			continue
		}

		instance, err := provider.Load(*site.Auth)
		if err != nil {
			slog.Warn("auth provider config incomplete; site falls back to default view",
				"path", site.Path, "provider", site.Auth.Provider, "hint", err.Error())
			group.GET("", defaultView)
			group.GET("/", defaultView)
			continue
		}

		// Mount the provider's OAuth routes once, deduped by provider name.
		// Routes are registered on the root router (not the site group) so
		// they're reachable via both localhost and the FQDN reverse proxy.
		if !seenProviders[site.Auth.Provider] {
			instance.RegisterAuthRoutes(router, store)
			seenProviders[site.Auth.Provider] = true
		}

		group.Use(instance.AuthMiddleware(store))
		registerSiteGroup(group, site, defaultView)
	}
}

// registerSiteGroup wires a site's own routes (or the default view) onto an
// already-created group. Kept as a helper since the public/no-auth and
// auth-success branches share this final step.
func registerSiteGroup(group *gin.RouterGroup, site *Site, defaultView gin.HandlerFunc) {
	if site.Routes != nil {
		site.Routes(group, site.ThemeValue())
	} else {
		group.GET("", defaultView)
		group.GET("/", defaultView)
	}
}
