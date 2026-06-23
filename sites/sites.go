package sites

import (
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux"
	"github.com/geoffjay/jughead/sites/tld/domain1"
	"github.com/geoffjay/jughead/sites/tld/domain2"
	"github.com/gin-gonic/gin"

	"github.com/a-h/templ"
)

type Site struct {
	Path      string
	Url       string
	Published bool
	Template  templ.Component
	Proxy     func(ctx *gin.Context)
	// Routes, when set, registers site-specific routes (e.g. subpages) on the
	// provided router. The callback is invoked once during server startup for
	// every loaded site.
	Routes func(router *gin.RouterGroup)
}

// TODO: Read from a configuration file
var sites = map[string]*Site{
	"/sites/domain1.tld": {
		Path:      "/sites/domain1.tld",
		Url:       "http://domain1.tld",
		Published: false,
		Template:  domain1.BodyContent(),
		Proxy:     domain1.Proxy,
	},
	"/sites/domain2.tld": {
		Path:      "/sites/domain2.tld",
		Url:       "http://domain2.tld",
		Published: false,
		Template:  domain2.BodyContent(),
		Proxy:     domain2.Proxy,
	},
	"/sites/quux.geoffjay.com": {
		Path:      "/sites/quux.geoffjay.com",
		Url:       "https://quux.geoffjay.com",
		Published: true,
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
