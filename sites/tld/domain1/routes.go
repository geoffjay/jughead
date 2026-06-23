package domain1

import (
	"net/http"

	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// Routes registers the domain1 pages on the provided router group. The
// group's base path is the site path (e.g. "/sites/domain1.tld"). The theme is
// the site's configured daisyUI theme, passed through to templates.Layout.
func Routes(router *gin.RouterGroup, theme string) {
	router.GET("", func(c *gin.Context) { render(c, theme, "domain1.tld", Home()) })
	router.GET("/", func(c *gin.Context) { render(c, theme, "domain1.tld", Home()) })
	router.GET("/components/accordion", func(c *gin.Context) {
		render(c, theme, "Accordion - domain1.tld", AccordionPage())
	})
	router.GET("/components/navbar", func(c *gin.Context) {
		render(c, theme, "Navbar - domain1.tld", NavbarPage())
	})
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