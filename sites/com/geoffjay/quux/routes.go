package quux

import (
	"net/http"
	"strconv"

	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/data"
	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// sitePath is the configured path under which quux is served.
const sitePath = "/sites/quux.geoffjay.com"

// Routes registers the quux pages on the provided router group. The group's
// base path is the site path. The theme is the site's configured daisyUI theme,
// passed through to templates.Layout.
//
// A links.LinkResolver is built from each request so links render with the
// site-path prefix when accessed directly and root-relative when accessed via
// the FQDN reverse proxy.
func Routes(router *gin.RouterGroup, theme string) {
	router.GET("", func(c *gin.Context) { home(c, theme) })
	router.GET("/", func(c *gin.Context) { home(c, theme) })
	router.GET("/about", func(c *gin.Context) { about(c, theme) })
	router.GET("/pr/:number", func(c *gin.Context) { pr(c, theme) })
}

func home(c *gin.Context, theme string) {
	state := data.GetReviewState()
	render(c, theme, "quux.geoffjay.com", BodyContent(resolver(c), state))
}

func about(c *gin.Context, theme string) {
	render(c, theme, "About - quux.geoffjay.com", AboutPage(resolver(c)))
}

// pr handles HTMX requests for a specific pull request. When the request is an
// HTMX swap it returns just the swappable review-pane fragment (center + right
// region); otherwise it renders the full page with that PR selected.
func pr(c *gin.Context, theme string) {
	number, err := strconv.Atoi(c.Param("number"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	state := data.GetReviewStateForPR(number)
	if state == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	r := resolver(c)
	if htmx.IsHTMX(c.Request) {
		renderPartial(c, reviewPane(r, *state))
		return
	}
	render(c, theme, "#"+strconv.Itoa(number)+" - quux.geoffjay.com", BodyContent(r, *state))
}

func resolver(c *gin.Context) links.LinkResolver {
	return links.NewLinkResolver(c.Request, sitePath)
}

// render wraps a body component in the full site layout and writes it.
func render(c *gin.Context, theme, title string, body templ.Component) {
	metaTags := pages.MetaTags(
		"quux, code review, geoffjay",
		"quux.geoffjay.com code review demo for jughead.",
	)
	siteTemplate := templates.Layout(title, theme, metaTags, body)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

// renderPartial writes a templ fragment without the page layout, for HTMX
// swaps. The fragment replaces the #review-pane region of the page.
func renderPartial(c *gin.Context, body templ.Component) {
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, body); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
