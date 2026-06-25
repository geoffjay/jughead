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

		target := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(http.StatusFound, "/auth/login?redirect="+target)
		c.Abort()
	}
}
