package docs

import (
	"net/http"

	"github.com/geoffjay/jughead/sites/links"
	doc "github.com/geoffjay/jughead/templates/layouts/documentation"

	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// Routes is the sites.Site.Routes callback for the docs site. It registers a
// handler for each page in the registry onto the provided router group. The
// group's base path is the site path; the theme is the site's configured
// daisyUI theme (passed through for parity with other sites, though the
// documentation layout sources its theme from Config.Meta.Theme).
func Routes(router *gin.RouterGroup, theme string) {
	_ = theme
	router.GET("", func(c *gin.Context) { renderPage(c, "/") })
	router.GET("/", func(c *gin.Context) { renderPage(c, "/") })
	for _, p := range Pages() {
		p := p
		if p.Slug == "/" {
			continue
		}
		router.GET(p.Slug, func(c *gin.Context) { renderPage(c, p.Slug) })
	}
}

// renderPage renders the documentation page for the given site-relative slug
// using the documentation layout. When no page matches, a 404 is returned.
func renderPage(c *gin.Context, slug string) {
	p, ok := pageBySlug(slug)
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	r := resolver(c)
	cfg := pageConfig(r, Page{
		Slug:    p.Slug,
		Title:   p.Title,
		Section: p.Section,
		Body:    p.Body(),
	})

	siteTemplate := doc.Layout(&cfg)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

// resolver builds a LinkResolver for the docs site from the request.
func resolver(c *gin.Context) links.LinkResolver {
	return links.NewLinkResolver(c.Request, sitePath)
}
