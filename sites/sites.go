package sites

import (
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
