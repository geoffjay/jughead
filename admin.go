package main

import (
	"errors"
	"net/http"

	"github.com/geoffjay/jughead/db"
	"github.com/geoffjay/jughead/services"
	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// adminCredentials is the static user/password store used by the login handler
// when no database pool is available (tests, DB-disabled deployments). When a
// UserService is wired, credentials are validated against the users table and
// this map is never consulted.
var adminCredentials = gin.Accounts{
	"admin": "password",
}

// loginViewHandler renders the login form.
func loginViewHandler(c *gin.Context) {
	redirect := c.Query("redirect")
	renderLogin(c, redirect, "")
}

// loginSubmitHandler validates posted credentials, creates a session on
// success, and redirects to the originally-requested target (or /admin).
//
// When a UserService is wired (DB pool available), credentials are validated
// against the users table via bcrypt. When userSvc is nil (DB-disabled
// deployments, tests), the legacy static-credential fallback is used so the
// admin UI remains functional without a database.
func loginSubmitHandler(store *sessions.Store, userSvc *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")
		redirect := c.PostForm("redirect")
		if redirect == "" {
			redirect = "/admin"
		}

		if userSvc != nil {
			u, err := userSvc.Authenticate(c.Request.Context(), username, password)
			if err != nil {
				if errors.Is(err, services.ErrUserInactive) {
					renderLogin(c, redirect, "This account has been deactivated")
				} else {
					renderLogin(c, redirect, "Invalid email or password")
				}
				return
			}

			// Auto-select the user's first organization so the admin UI has
			// a tenant context. The memberships_visible RLS policy admits
			// rows where user_id = current_user_id, so we scope to UserID.
			var orgID uuid.UUID
			mems, err := userSvc.ListMemberships(c.Request.Context(), db.Scope{UserID: u.ID})
			if err == nil && len(mems) > 0 {
				orgID = mems[0].OrganizationID
			}

			store.CreateWithUser(c.Writer, u.Email, u.ID, orgID)
			c.Redirect(http.StatusFound, redirect)
			return
		}

		// Legacy static-credential fallback (no DB pool).
		expected, ok := adminCredentials[username]
		if !ok || expected != password {
			renderLogin(c, redirect, "Invalid username or password")
			return
		}
		store.Create(c.Writer, username)
		c.Redirect(http.StatusFound, redirect)
	}
}

// logoutHandler destroys the session and redirects to the login page.
func logoutHandler(store *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		store.Destroy(c.Writer, c.Request)
		c.Redirect(http.StatusFound, "/login")
	}
}

// adminViewHandler renders the admin overview page with real org/user/member
// stats when DB services are available, or a placeholder when they're not.
func adminViewHandler(deps *AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps == nil || deps.Orgs == nil || deps.Users == nil {
			renderAdmin(c, "Admin", "Admin section", pages.AdminContent(pages.AdminData{}))
			return
		}

		scope := db.ScopeFromContext(c.Request.Context())
		ctx := c.Request.Context()

		orgs, _ := deps.Orgs.List(ctx, scope)
		mems, _ := deps.Mems.ListByOrganization(ctx, scope, scope.OrganizationID)

		data := pages.AdminData{
			OrgName:       orgNameForScope(orgs, scope.OrganizationID),
			OrgCount:      len(orgs),
			MemberCount:   len(mems),
			Organizations: orgs,
		}
		renderAdmin(c, "Admin", "Admin section", pages.AdminContent(data))
	}
}

// usersViewHandler renders the admin users page with the org's member list
// when DB services are available, or a placeholder when they're not.
func usersViewHandler(deps *AdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps == nil || deps.Mems == nil {
			renderAdmin(c, "Admin · Users", "Manage user accounts", pages.UsersContent(nil))
			return
		}

		scope := db.ScopeFromContext(c.Request.Context())
		mems, _ := deps.Mems.ListByOrganization(c.Request.Context(), scope, scope.OrganizationID)
		renderAdmin(c, "Admin · Users", "Manage user accounts", pages.UsersContent(mems))
	}
}

// orgNameForScope returns the name of the org matching scope.OrganizationID,
// or "—" when not found.
func orgNameForScope(orgs []db.Organization, id uuid.UUID) string {
	for _, o := range orgs {
		if o.ID == id {
			return o.Name
		}
	}
	return "—"
}

// sitesViewHandler renders the admin sites page.
func sitesViewHandler(c *gin.Context) {
	renderAdmin(c, "Admin · Sites", "Manage configured sites", pages.SitesContent())
}

// settingsViewHandler renders the admin settings page.
func settingsViewHandler(c *gin.Context) {
	renderAdmin(c, "Admin · Settings", "Application configuration", pages.SettingsContent())
}

// logsViewHandler renders the admin logs page.
func logsViewHandler(c *gin.Context) {
	renderAdmin(c, "Admin · Logs", "Application and audit logs", pages.LogsContent())
}

// renderAdmin renders an admin page with the standard layout and meta tags.
func renderAdmin(c *gin.Context, title, description string, body templ.Component) {
	metaTags := pages.MetaTags("jughead admin", description)
	tmpl := templates.Layout(title, "light", metaTags, body)

	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, tmpl); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

// renderLogin renders the login page with the given state.
func renderLogin(c *gin.Context, redirect, errorMessage string) {
	metaTags := pages.MetaTags("jughead login", "Sign in")
	body := pages.LoginContent(redirect, errorMessage)
	tmpl := templates.Layout("Login", "light", metaTags, body)

	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, tmpl); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
