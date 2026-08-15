// provider.go implements the Google OAuth auth provider, conforming to the
// sdkauth.Provider / sdkauth.ProviderInstance contract. It mirrors the GitHub
// provider's provider.go structure: Load reads credentials from the
// environment, RegisterAuthRoutes mounts the OAuth web-flow routes, and
// AuthMiddleware protects a site's route group.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	sdkauth "github.com/geoffjay/jughead/sdk/auth"
	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

// OAuth timing constants.
const (
	oauthStateMaxAge = 600 // seconds; 10 minutes is plenty for the round trip
	oauthStateBytes  = 16
	oauthHTTPTimeout = 30 * time.Second
)

// Body-size caps to protect against unbounded responses.
const (
	maxTokenBody = 1 << 14 // 16 KiB
	maxUserBody  = 1 << 14 // 16 KiB
)

// stateCookieName carries the OAuth CSRF state between the login start and the
// callback. Same-site=Lax matches the session cookie convention.
const stateCookieName = "jughead_google_oauth_state"

// routePrefix is the URL prefix under which the Google provider mounts its
// auth routes. The SiteManager passes the root router to RegisterAuthRoutes,
// which registers /auth/google/{login,callback,logout}.
const routePrefix = "/auth/google"

// NewProvider returns the Google auth provider, ready for registration with
// auth.Register. It implements sdkauth.Provider; Load reads the Google OAuth
// credentials from the environment and produces a googleInstance.
func NewProvider() sdkauth.Provider { return googleProvider{} }

type googleProvider struct{}

func (googleProvider) Name() string { return "google" }

// Load reads GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET (and optionally
// GOOGLE_REDIRECT_URL, GOOGLE_ALLOWED_LOGIN) from the environment. The cfg
// overrides take precedence over the environment for RedirectURL and
// AllowedLogin, letting a site customize them without env plumbing.
//
// A non-nil error means the configuration is incomplete; the SiteManager
// treats that as "auth disabled for this site" and renders the site's default
// view instead of mounting protected routes.
func (googleProvider) Load(cfg sdkauth.AuthConfig) (sdkauth.ProviderInstance, error) {
	oc, err := LoadOAuthConfig(cfg.RedirectURL, cfg.AllowedLogin)
	if err != nil {
		return nil, err
	}
	return &googleInstance{cfg: oc}, nil
}

// googleInstance is a configured, ready-to-serve Google OAuth provider. It
// holds the loaded credentials and renders its routes/middleware from them.
type googleInstance struct {
	cfg OAuthConfig
}

// RegisterAuthRoutes wires the Google OAuth web flow onto the given router
// under /auth/google/. The routes are mounted at the app root (not under a
// site path) so they're reachable via both localhost and the FQDN reverse
// proxy.
func (g *googleInstance) RegisterAuthRoutes(router gin.IRouter, store *sessions.Store) {
	router.GET(routePrefix+"/login", func(c *gin.Context) { g.oauthLogin(c) })
	router.GET(routePrefix+"/callback", func(c *gin.Context) { g.oauthCallback(c, store) })
	router.GET(routePrefix+"/logout", func(c *gin.Context) { g.oauthLogout(c, store) })
}

// AuthMiddleware returns the gin handler that protects a site's route group.
// Requests without an AccessToken in their session are redirected to the
// Google login start endpoint (/auth/google/login). The original target is
// passed via the `redirect` query param so the OAuth flow can return the user
// there.
//
// When the request arrived via the reverse proxy (X-Site-Base header present),
// the browser's URL is root-relative (e.g. "/"), not the inner site path (e.g.
// "/sites/quux.geoffjay.com"). The redirect target must use the browser-visible
// path to avoid a double-prefix after the proxy rewrites it again.
func (g *googleInstance) AuthMiddleware(store *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := store.Get(c.Request)
		if sess != nil && sess.AccessToken != "" {
			c.Set("username", sess.Login)
			c.Set("google_token", sess.AccessToken)
			c.Set("google_email", sess.Login)
			c.Set("google_avatar", sess.AvatarURL)
			c.Next()
			return
		}

		target := browserRedirectTarget(c)
		c.Redirect(http.StatusFound, routePrefix+"/login?redirect="+target)
		c.Abort()
	}
}

// oauthLogin starts the OAuth flow: generate a state nonce, stash it in a
// short-lived cookie, and redirect to Google's authorize URL.
func (g *googleInstance) oauthLogin(c *gin.Context) {
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

// oauthCallback handles the redirect back from Google: validates state,
// exchanges the code for an access token, fetches the viewer profile, enforces
// the allowed-login restriction, creates a session, and redirects to the
// originally-requested target.
func (g *googleInstance) oauthCallback(c *gin.Context, store *sessions.Store) {
	redirect := pickPostLoginTarget(c.Query("redirect"))

	if err := c.Query("error"); err != "" {
		renderOAuthError(c, "Google returned an error: "+err)
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		renderOAuthError(c, "Missing code or state in Google callback.")
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

	email, avatarURL, err := FetchViewer(ctx, token)
	if err != nil {
		renderOAuthError(c, "Failed to fetch Google user profile: "+err.Error())
		return
	}
	if !AllowedLogin(g.cfg, email) {
		renderOAuthError(c, "Google account "+email+" is not permitted to use this app.")
		return
	}

	store.CreateWithToken(c.Writer, email, token, avatarURL)
	c.Redirect(http.StatusFound, redirect)
}

// oauthLogout destroys the session and redirects to the login start.
func (g *googleInstance) oauthLogout(c *gin.Context, store *sessions.Store) {
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
<a href="/auth/google/login" class="btn btn-primary">Try again</a>
</div></div></body></html>`, msg)
}
