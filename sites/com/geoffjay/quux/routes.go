package quux

import (
	"net/http"
	"strconv"

	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// Routes registers the quux subpages on the provided router group. The group's
// base path is the site path (e.g. "/sites/quux.geoffjay.com"). The theme is
// the site's configured daisyUI theme, passed through to templates.Layout.
func Routes(router *gin.RouterGroup, theme string) {
	router.GET("", func(c *gin.Context) { home(c, theme) })
	router.GET("/", func(c *gin.Context) { home(c, theme) })
	router.GET("/about", func(c *gin.Context) { about(c, theme) })
	router.GET("/counter", func(c *gin.Context) { counter(c, theme) })
}

func home(c *gin.Context, theme string)  { render(c, theme, "", 0, "quux.geoffjay.com") }
func about(c *gin.Context, theme string) { render(c, theme, "about", 0, "About - quux.geoffjay.com") }
func counter(c *gin.Context, theme string) {
	n := 0
	if q := c.Query("n"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			n = parsed
		}
	}
	render(c, theme, "counter", n, "Counter - quux.geoffjay.com")
}

func render(c *gin.Context, theme, current string, count int, title string) {
	metaTags := pages.MetaTags(
		"quux, geoffjay, sample site",
		"quux.geoffjay.com sample site for jughead routing tests.",
	)
	siteTemplate := templates.Layout(title, theme, metaTags, BodyContent(current, count))
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
