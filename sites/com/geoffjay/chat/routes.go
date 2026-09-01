package chat

import (
	"net/http"

	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// sitePath is the configured path under which chat is served.
const sitePath = "/sites/chat.geoffjay.com"

// Routes is the Site.Routes callback for the chat site. Auth middleware is
// applied by the SiteManager before Routes runs, so handlers can rely on the
// google_email and google_avatar context keys being set by the Google
// provider's AuthMiddleware.
func Routes(router *gin.RouterGroup, theme string) {
	router.GET("", func(c *gin.Context) { home(c, theme) })
	router.GET("/", func(c *gin.Context) { home(c, theme) })
}

// home renders the authenticated chat landing page. The Google auth
// middleware sets google_email (the session login, which is the user's email
// address) and google_avatar (the profile picture URL from userinfo).
func home(c *gin.Context, theme string) {
	email, _ := c.Get("google_email")
	emailStr, _ := email.(string)

	avatar, _ := c.Get("google_avatar")
	avatarStr, _ := avatar.(string)

	r := resolver(c)
	render(c, theme, "chat.geoffjay.com", ChatPage(r, emailStr, avatarStr))
}

func resolver(c *gin.Context) links.LinkResolver {
	return links.NewLinkResolver(c.Request, sitePath)
}

// render wraps a body component in the full site layout and writes it.
func render(c *gin.Context, theme, title string, body templ.Component) {
	metaTags := pages.MetaTags(
		"chat, google, geoffjay",
		"chat.geoffjay.com Google OAuth demo for jughead.",
	)
	siteTemplate := templates.Layout(title, theme, metaTags, body)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
