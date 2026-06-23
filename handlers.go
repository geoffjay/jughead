package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/geoffjay/jughead/sites"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// indexViewHandler handles a view for the index page.
func indexViewHandler(c *gin.Context) {
	metaTags := pages.MetaTags(
		"gowebly, htmx example page, go with htmx",
		"Welcome to example! You're here because it worked out.",
	)

	bodyContent := pages.BodyContent(
		"Welcome to example!",
		"You're here because it worked out.",
	)

	indexTemplate := templates.Layout(
		"Welcome to example!",
		"light",
		metaTags,
		bodyContent,
	)

	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, indexTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

// siteViewHandler handles a view for a site page.
func siteViewHandler(c *gin.Context) {
	// meta, body, and template need to be pulled from the site requested
	// and not hard-coded like this example.
	//
	// to do this:
	//
	// 1. get the site from the request path
	// 2. get the meta, body, and template from the site manager implementation
	// 3. render the template with the meta and body

	site := sites.GetSiteManager().GetSite(strings.TrimRight(c.Request.URL.Path, "/"))
	if site == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	metaTags := pages.MetaTags(
		"testing a site",
		fmt.Sprintf("Welcome to %s", site.Url),
	)

	siteTemplate := templates.Layout(
		fmt.Sprintf("Welcome to %s", site.Url),
		site.ThemeValue(),
		metaTags,
		site.Template,
	)

	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

// showContentAPIHandler handles an API endpoint to show content.
func showContentAPIHandler(c *gin.Context) {
	// Check, if the current request has a 'HX-Request' header.
	// For more information, see https://htmx.org/docs/#request-headers
	if !htmx.IsHTMX(c.Request) {
		// If not, return HTTP 400 error.
		_ = c.AbortWithError(http.StatusBadRequest, errors.New("non-htmx request"))

		return
	}

	// Write HTML content.
	if _, err := c.Writer.WriteString("<p>🎉 Yes, <strong>htmx</strong> is ready to use! (<code>GET /api/hello-world</code>)</p>"); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("write response: %w", err))
		return
	}

	// Send htmx response.
	if err := htmx.NewResponse().Write(c.Writer); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("write htmx response: %w", err))
		return
	}
}
