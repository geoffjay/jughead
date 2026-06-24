package sites

import (
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux"
	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/sites/tld/domain1"
	"github.com/geoffjay/jughead/sites/tld/domain2"
	"github.com/gin-gonic/gin"

	"github.com/a-h/templ"
)

type Site struct {
	Path      string
	Url       string
	Published bool
	Theme     string
	Template  templ.Component
	Proxy     func(ctx *gin.Context)
	// Routes, when set, registers site-specific routes (e.g. subpages) on the
	// provided router. The callback is invoked once during server startup for
	// every loaded site, receiving the site's configured daisyUI theme so
	// handlers can pass it through to templates.Layout.
	Routes func(router *gin.RouterGroup, theme string)
}

// ThemeValue returns the configured daisyUI theme for the site, defaulting to
// "light" when unset. daisyUI 5 ships 35 built-in themes that can be assigned
// via the Theme field (e.g. "dark", "cupcake", "nord", "dracula", ...).
func (s *Site) ThemeValue() string {
	if s == nil || s.Theme == "" {
		return "light"
	}
	return s.Theme
}

// TODO: Read from a configuration file
var sites = map[string]*Site{
	"/sites/domain1.tld": {
		Path:      "/sites/domain1.tld",
		Url:       "http://domain1.tld",
		Published: false,
		Theme:     "kanagawa-light",
		Template:  domain1.Home(links.NewDirectResolver("/sites/domain1.tld")),
		Proxy:     domain1.Proxy,
		Routes:    domain1.Routes,
	},
	"/sites/domain2.tld": {
		Path:      "/sites/domain2.tld",
		Url:       "http://domain2.tld",
		Published: false,
		Theme:     "coffee",
		Template:  domain2.BodyContent(),
		Proxy:     domain2.Proxy,
	},
	"/sites/quux.geoffjay.com": {
		Path:      "/sites/quux.geoffjay.com",
		Url:       "https://quux.geoffjay.com",
		Published: true,
		Theme:     "kanagawa-dark",
		Template:  quux.BodyContent("", 0),
		Proxy:     quux.Proxy,
		Routes:    quux.Routes,
	},
}

func Initialize() {
	sm := GetSiteManager()

	for _, site := range sites {
		if !ShouldLoad(site.Published) {
			continue
		}
		sm.RegisterSite(site.Path, site)
	}
}
