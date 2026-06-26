package github

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites/auth"

	"github.com/gin-gonic/gin"
)

// OAuth timing constants.
const (
	oauthStateMaxAge = 600 // seconds; 10 minutes is plenty for the round trip
	oauthStateBytes  = 16
	oauthHTTPTimeout = 30 * time.Second
)

// stateCookieName carries the OAuth CSRF state between the login start and the
// callback. Same-site=Lax matches the session cookie convention.
const stateCookieName = "jughead_oauth_state"

// routePrefix is the URL prefix under which the GitHub provider mounts its
// auth routes. The SiteManager passes the root router to RegisterAuthRoutes,
// which registers /auth/github/{login,callback,logout}.
const routePrefix = "/auth/github"

// NewProvider returns the GitHub auth provider, ready for registration with
// auth.Register. It implements auth.Provider; Load reads the GitHub OAuth
// credentials from the environment and produces a githubInstance.
func NewProvider() auth.Provider { return githubProvider{} }

type githubProvider struct{}

func (githubProvider) Name() string { return "github" }

// Load reads GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET (and optionally
// GITHUB_REDIRECT_URL, GITHUB_ALLOWED_LOGIN) from the environment. The cfg
// overrides take precedence over the environment for RedirectURL and
// AllowedLogin, letting a site customize them without env plumbing.
//
// A non-nil error means the configuration is incomplete; the SiteManager
// treats that as "auth disabled for this site" and renders the site's default
// view instead of mounting protected routes.
func (githubProvider) Load(cfg auth.AuthConfig) (auth.ProviderInstance, error) {
	oc, err := LoadOAuthConfig(cfg.RedirectURL, cfg.AllowedLogin)
	if err != nil {
		return nil, err
	}
	return &githubInstance{cfg: oc}, nil
}

// githubInstance is a configured, ready-to-serve GitHub OAuth provider. It
// holds the loaded credentials and renders its routes/middleware from them.
type githubInstance struct {
	cfg OAuthConfig
}

// RegisterAuthRoutes wires the GitHub OAuth web flow onto the given router
// under /auth/github/. The routes are mounted at the app root (not under a
// site path) so they're reachable via both localhost and the FQDN reverse
// proxy.
func (g *githubInstance) RegisterAuthRoutes(router gin.IRouter, store *sessions.Store) {
	router.GET(routePrefix+"/login", func(c *gin.Context) { g.oauthLogin(c) })
	router.GET(routePrefix+"/callback", func(c *gin.Context) { g.oauthCallback(c, store) })
	router.GET(routePrefix+"/logout", func(c *gin.Context) { g.oauthLogout(c, store) })
}

// AuthMiddleware returns the gin handler that protects a site's route group.
// Requests without an AccessToken in their session are redirected to the
// GitHub login start endpoint (/auth/github/login). The original target is
// passed via the `redirect` query param so the OAuth flow can return the user
// there.
//
// When the request arrived via the reverse proxy (X-Site-Base header present),
// the browser's URL is root-relative (e.g. "/"), not the inner site path (e.g.
// "/sites/quux.geoffjay.com"). The redirect target must use the browser-visible
// path to avoid a double-prefix after the proxy rewrites it again.
func (g *githubInstance) AuthMiddleware(store *sessions.Store) gin.HandlerFunc {
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
		c.Redirect(http.StatusFound, routePrefix+"/login?redirect="+target)
		c.Abort()
	}
}

// oauthLogin starts the OAuth flow: generate a state nonce, stash it in a
// short-lived cookie, and redirect to GitHub's authorize URL.
func (g *githubInstance) oauthLogin(c *gin.Context) {
	state := newStateNonce()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   oauthStateMaxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	redirectURL := AuthorizeRedirect(g.cfg, state)
	c.Redirect(http.StatusFound, redirectURL)
}

// oauthCallback handles the redirect back from GitHub: validates state,
// exchanges the code for an access token, fetches the viewer profile, enforces
// the allowed-login restriction, creates a session, and redirects to the
// originally-requested target.
func (g *githubInstance) oauthCallback(c *gin.Context, store *sessions.Store) {
	redirect := pickPostLoginTarget(c.Query("redirect"))

	if err := c.Query("error"); err != "" {
		renderOAuthError(c, "GitHub returned an error: "+err)
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		renderOAuthError(c, "Missing code or state in GitHub callback.")
		return
	}
	if !validStateCookie(c, state) {
		renderOAuthError(c, "OAuth state mismatch — please try signing in again.")
		return
	}
	clearStateCookie(c.Writer)

	ctx, cancel := context.WithTimeout(c.Request.Context(), oauthHTTPTimeout)
	defer cancel()

	token, err := ExchangeCode(ctx, g.cfg, code)
	if err != nil {
		renderOAuthError(c, "Failed to exchange authorization code: "+err.Error())
		return
	}

	login, avatarURL, err := FetchViewer(ctx, token)
	if err != nil {
		renderOAuthError(c, "Failed to fetch GitHub user profile: "+err.Error())
		return
	}
	if !AllowedLogin(g.cfg, login) {
		renderOAuthError(c, "GitHub login "+login+" is not permitted to use this app.")
		return
	}

	store.CreateWithToken(c.Writer, login, token, avatarURL)
	c.Redirect(http.StatusFound, redirect)
}

// oauthLogout destroys the session and redirects to the login start.
func (g *githubInstance) oauthLogout(c *gin.Context, store *sessions.Store) {
	store.Destroy(c.Writer, c.Request)
	c.Redirect(http.StatusFound, routePrefix+"/login")
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

// pickPostLoginTarget returns the redirect query param or "/" when unset. The
// default "/" works on both the FQDN proxy (where "/" hits the site) and
// direct localhost access (where the browser is already on the site path).
func pickPostLoginTarget(redirect string) string {
	if redirect != "" {
		return redirect
	}
	return "/"
}

// newStateNonce returns 16 random bytes hex-encoded (32 chars).
func newStateNonce() string {
	b := make([]byte, oauthStateBytes)
	if _, err := rand.Read(b); err != nil {
		panic("oauth: rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// validStateCookie reports whether the state cookie matches the query state.
func validStateCookie(c *gin.Context, queryState string) bool {
	cookie, err := c.Cookie(stateCookieName)
	if err != nil || strings.TrimSpace(cookie) == "" {
		return false
	}
	return cookie == queryState
}

func clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: stateCookieName, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

// renderOAuthError writes a simple error page. Kept minimal (no new template)
// since OAuth errors are rare and a plain HTML response is fine.
func renderOAuthError(c *gin.Context, msg string) {
	html := oauthErrorHTML(msg)
	c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(html))
	c.Abort()
}

func oauthErrorHTML(msg string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="light">
<head><meta charset="utf-8"><title>Sign-in error</title>
<link rel="stylesheet" href="/static/styles.css"></head>
<body class="min-h-screen bg-base-200 grid place-content-center p-8">
<div class="card bg-base-100 shadow-lg max-w-md">
<div class="card-body">
<h1 class="text-xl font-semibold mb-2">Sign-in error</h1>
<p class="text-base-content/70 mb-4">%s</p>
<a href="/auth/github/login" class="btn btn-primary">Try again</a>
</div></div></body></html>`, msg)
}
