package middleware

import (
	"net/http"

	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

// AuthRequired protects a route group. Requests without a valid session are
// redirected to the login page; the original target is passed via the
// `redirect` query param so login can return the user there.
func AuthRequired(store *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sess := store.Get(c.Request); sess != nil {
			c.Set("username", sess.Username)
			c.Next()
			return
		}

		// Avoid infinite redirect loops: if the unauthenticated request was
		// already aimed at /login, just let the login handler render.
		if c.Request.URL.Path == "/login" {
			c.Next()
			return
		}

		target := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(http.StatusFound, "/login?redirect="+target)
		c.Abort()
	}
}
