package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/geoffjay/jughead/auth"
	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/config"

	"github.com/gin-gonic/gin"
)

// stateCookieName carries the OAuth CSRF state between the login start and the
// callback. Same-site=Lax matches the session cookie convention.
const stateCookieName = "jughead_oauth_state"

// registerAuthRoutes wires the GitHub OAuth web flow onto the router. The
// routes are mounted at the app root (not under a site path) so they're
// reachable via both localhost and the FQDN reverse proxy.
func registerAuthRoutes(router *gin.Engine, store *sessions.Store, cfg config.OAuthConfig) {
	router.GET("/auth/login", func(c *gin.Context) { oauthLogin(c, cfg) })
	router.GET("/auth/callback", func(c *gin.Context) { oauthCallback(c, store, cfg) })
	router.GET("/auth/logout", func(c *gin.Context) { oauthLogout(c, store) })
}

// oauthLogin starts the OAuth flow: generate a state nonce, stash it in a
// short-lived cookie, and redirect to GitHub's authorize URL.
func oauthLogin(c *gin.Context, cfg config.OAuthConfig) {
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
	redirectURL := auth.AuthorizeRedirect(cfg, state)
	c.Redirect(http.StatusFound, redirectURL)
}

// oauthCallback handles the redirect back from GitHub: validates state,
// exchanges the code for an access token, fetches the viewer profile, enforces
// the allowed-login restriction, creates a session, and redirects to the
// originally-requested target (or the quux site).
func oauthCallback(c *gin.Context, store *sessions.Store, cfg config.OAuthConfig) {
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

	token, err := auth.ExchangeCode(ctx, cfg, code)
	if err != nil {
		renderOAuthError(c, "Failed to exchange authorization code: "+err.Error())
		return
	}

	login, avatarURL, err := auth.FetchViewer(ctx, token)
	if err != nil {
		renderOAuthError(c, "Failed to fetch GitHub user profile: "+err.Error())
		return
	}
	if !auth.AllowedLogin(cfg, login) {
		renderOAuthError(c, "GitHub login "+login+" is not permitted to use this app.")
		return
	}

	store.CreateWithToken(c.Writer, login, token, avatarURL)
	c.Redirect(http.StatusFound, redirect)
}

// oauthLogout destroys the session and redirects to the login start.
func oauthLogout(c *gin.Context, store *sessions.Store) {
	store.Destroy(c.Writer, c.Request)
	c.Redirect(http.StatusFound, "/auth/login")
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

// pickPostLoginTarget returns the redirect query param or "/" when unset. The
// default "/" works on both the FQDN proxy (where "/" hits the quux site) and
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
