package quux

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/geoffjay/jughead/auth"
	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// sitePath is the configured path under which quux is served.
const sitePath = "/sites/quux.geoffjay.com"

// requestTimeout bounds GitHub API calls made on behalf of a single request.
const requestTimeout = 30 * time.Second

// RegisterRoutes wires the quux routes onto the given group using the Service
// for data loading. The group's base path is the site path; the theme is the
// site's configured daisyUI theme, passed through to templates.Layout.
//
// This replaces the older Routes(router, theme) signature so the Service can be
// injected; server.go calls RegisterRoutes instead of the bare Routes.
func (s *Service) RegisterRoutes(router *gin.RouterGroup, theme string) {
	router.GET("", func(c *gin.Context) { s.home(c, theme) })
	router.GET("/", func(c *gin.Context) { s.home(c, theme) })
	router.GET("/about", func(c *gin.Context) { about(c, theme) })
	router.GET("/pr/:owner/:repo/:number", func(c *gin.Context) { s.pr(c, theme) })
}

func (s *Service) home(c *gin.Context, theme string) {
	login, _ := c.Get("github_login")
	viewer, _ := login.(string)

	ctx, cancel := boundedCtx(c)
	defer cancel()

	state, err := s.LoadReviewState(ctx, clientFor(c), viewer)
	if err != nil {
		s.renderError(c, theme, "Loading your assigned PRs", err)
		return
	}
	render(c, theme, "quux.geoffjay.com", BodyContent(resolver(c), state))
}

func (s *Service) pr(c *gin.Context, theme string) {
	owner := c.Param("owner")
	repo := c.Param("repo")
	number, err := strconv.Atoi(c.Param("number"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	login, _ := c.Get("github_login")
	viewer, _ := login.(string)

	ctx, cancel := boundedCtx(c)
	defer cancel()

	state, err := s.LoadReviewStateForPR(ctx, clientFor(c), viewer, owner, repo, number)
	if err != nil {
		if isNotFound(err) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		s.renderError(c, theme, "Loading PR #"+strconv.Itoa(number), err)
		return
	}
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

func about(c *gin.Context, theme string) {
	render(c, theme, "About - quux.geoffjay.com", AboutPage(resolver(c)))
}

// renderError renders an error page when a GitHub API call fails. Kept on
// Service to avoid a global; theme is the site theme for layout consistency.
func (s *Service) renderError(c *gin.Context, theme, action string, err error) {
	render(c, theme, "Error - quux.geoffjay.com", ErrorPage(action, err.Error()))
}

func resolver(c *gin.Context) links.LinkResolver {
	return links.NewLinkResolver(c.Request, sitePath)
}

// boundedCtx returns a request context with a deadline, so a hung GitHub call
// doesn't hold the handler open indefinitely.
func boundedCtx(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), requestTimeout)
}

func isNotFound(err error) bool {
	var ae *auth.APIError
	if errors.As(err, &ae) {
		return ae.Status == http.StatusNotFound
	}
	return false
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
