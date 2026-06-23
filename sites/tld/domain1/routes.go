package domain1

import (
	"net/http"

	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// sitePath is the configured path under which domain1 is served.
const sitePath = "/sites/domain1.tld"

// Routes registers the domain1 pages on the provided router group. The
// group's base path is the site path (e.g. "/sites/domain1.tld"). The theme is
// the site's configured daisyUI theme, passed through to templates.Layout.
//
// A links.LinkResolver is built from each request so that links render with
// the site-path prefix when accessed directly (e.g.
// http://localhost:9000/sites/domain1.tld) and root-relative when accessed via
// the FQDN reverse proxy (e.g. http://domain1.tld:9000).
func Routes(router *gin.RouterGroup, theme string) {
	router.GET("", func(c *gin.Context) { render(c, theme, "domain1.tld", Home(resolver(c))) })
	router.GET("/", func(c *gin.Context) { render(c, theme, "domain1.tld", Home(resolver(c))) })
	router.GET("/components/accordion", func(c *gin.Context) {
		render(c, theme, "Accordion - domain1.tld", AccordionPage(resolver(c)))
	})
	router.GET("/components/navbar", func(c *gin.Context) {
		render(c, theme, "Navbar - domain1.tld", NavbarPage(resolver(c)))
	})
}

func resolver(c *gin.Context) links.LinkResolver {
	return links.NewLinkResolver(c.Request, sitePath)
}

func render(c *gin.Context, theme, title string, body templ.Component) {
	metaTags := pages.MetaTags(
		"domain1, daisyui, components",
		"domain1.tld daisyUI component examples.",
	)
	siteTemplate := templates.Layout(title, theme, metaTags, body)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
