package middleware

import (
	"net/http"

	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

// GitHubAuthRequired protects a route group that needs a valid GitHub OAuth
// session. Requests without an AccessToken in their session are redirected to
// the OAuth login start endpoint (/auth/login). The original target is passed
// via the `redirect` query param so the OAuth flow can return the user there.
//
// This is distinct from AuthRequired (which gates the admin password login);
// GitHubAuthRequired specifically checks for the GitHub token field.
//
// When the request arrived via the reverse proxy (X-Site-Base header present),
// the browser's URL is root-relative (e.g. "/"), not the inner site path (e.g.
// "/sites/quux.geoffjay.com"). The redirect target must use the browser-visible
// path to avoid a double-prefix after the proxy rewrites it again.
func GitHubAuthRequired(store *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := store.Get(c.Request)
		if sess != nil && sess.AccessToken != "" {
			c.Set("username", sess.Login)
			c.Set("github_token", sess.AccessToken)
			c.Set("github_login", sess.Login)
			c.Next()
			return
		}

		target := browserRedirectTarget(c)
		c.Redirect(http.StatusFound, "/auth/login?redirect="+target)
		c.Abort()
	}
}

// browserRedirectTarget returns the path the browser should be sent back to
// after OAuth. When proxied (X-Site-Base set), the browser sees root-relative
// URLs, so "/" is correct. When accessed directly, the browser is on the site
// path, so the full request path is used.
func browserRedirectTarget(c *gin.Context) string {
	if len(c.Request.Header.Values("X-Site-Base")) > 0 {
		return "/"
	}
	target := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		target += "?" + c.Request.URL.RawQuery
	}
	return target
}
