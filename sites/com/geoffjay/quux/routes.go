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
// base path is the site path (e.g. "/sites/quux.geoffjay.com").
func Routes(router *gin.RouterGroup) {
	router.GET("", home)
	router.GET("/", home)
	router.GET("/about", about)
	router.GET("/counter", counter)
}

func home(c *gin.Context)  { render(c, "", 0, "quux.geoffjay.com") }
func about(c *gin.Context) { render(c, "about", 0, "About - quux.geoffjay.com") }
func counter(c *gin.Context) {
	n := 0
	if q := c.Query("n"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			n = parsed
		}
	}
	render(c, "counter", n, "Counter - quux.geoffjay.com")
}

func render(c *gin.Context, current string, count int, title string) {
	metaTags := pages.MetaTags(
		"quux, geoffjay, sample site",
		"quux.geoffjay.com sample site for jughead routing tests.",
	)
	siteTemplate := templates.Layout(title, metaTags, BodyContent(current, count))
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}