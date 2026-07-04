package middleware

import (
	"net/http"

	"github.com/geoffjay/jughead/db"
	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

// AuthRequired protects a route group. Requests without a valid session are
// redirected to the login page; the original target is passed via the
// `redirect` query param so login can return the user there.
//
// When a valid session is found, the user's DB identity (UserID and
// OrganizationID) is loaded into the request context as a db.Scope so every
// downstream service call is automatically tenant-scoped. Legacy sessions
// (static admin, no UserID) get a zero Scope and are not RLS-gated.
func AuthRequired(store *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := store.Get(c.Request)
		if sess != nil {
			c.Set("username", sess.Username)

			// Attach the RLS scope so handlers can call service methods that
			// rely on db.WithOrgContext resolving a scope from the context.
			scope := db.Scope{
				UserID:         sess.UserID,
				OrganizationID: sess.OrganizationID,
			}
			c.Request = c.Request.WithContext(db.WithScope(c.Request.Context(), scope))

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
