package main

import (
	"net/http"

	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// adminCredentials is the static user/password store used by the login handler
// until a real database-backed accounts table replaces it. Mirrors the shape of
// gin.Accounts so the future swap to gin.BasicAuth (or a DB lookup) is trivial.
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
func loginSubmitHandler(store *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")
		redirect := c.PostForm("redirect")
		if redirect == "" {
			redirect = "/admin"
		}

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

// adminViewHandler renders the (currently empty) admin overview page.
func adminViewHandler(c *gin.Context) {
	renderAdmin(c, "Admin", "Admin section", pages.AdminContent())
}

// sitesViewHandler renders the admin sites page.
func sitesViewHandler(c *gin.Context) {
	renderAdmin(c, "Admin · Sites", "Manage configured sites", pages.SitesContent())
}

// usersViewHandler renders the admin users page.
func usersViewHandler(c *gin.Context) {
	renderAdmin(c, "Admin · Users", "Manage user accounts", pages.UsersContent())
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
