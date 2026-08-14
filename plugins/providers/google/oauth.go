// oauth.go implements the Google OAuth 2.0 authorization-code web flow
// helpers the provider uses: building the authorize URL, exchanging the code
// for an access token, and fetching the viewer profile from the userinfo
// endpoint. It mirrors the GitHub provider's oauth.go structure.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// OAuth endpoints on Google's authorization and token servers.
const (
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	googleUserInfoURL  = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// OAuthScopes requested from the user. "openid" enables OpenID Connect;
// "email" resolves the viewer's email address; "profile" resolves name and
// picture.
const oauthScopes = "openid email profile"

// tokenResponse models the JSON returned by the access-token exchange
// endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// viewerUser models the subset of GET /userinfo fields we need.
type viewerUser struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Picture string `json:"picture"`
}

// AuthorizeRedirect builds the Google authorize URL and returns it for the
// caller to issue a 302 with. state is an opaque CSRF token the caller must
// persist (cookie) and verify in the callback.
func AuthorizeRedirect(cfg OAuthConfig, state string) string {
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", oauthScopes)
	q.Set("state", state)
	return googleAuthorizeURL + "?" + q.Encode()
}

// ExchangeCode POSTs the authorization code to Google and returns the access
// token. Called from the OAuth callback handler.
func ExchangeCode(ctx context.Context, cfg OAuthConfig, code string) (string, error) {
	form := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {cfg.RedirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenBody))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tr.Error != "" {
		return "", fmt.Errorf("google: %s: %s", tr.Error, tr.ErrorDesc)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("google: empty access token")
	}
	return tr.AccessToken, nil
}

// FetchViewer calls GET /userinfo with the freshly minted token and returns
// the viewer's email (used as the login identity) and profile picture URL.
func FetchViewer(ctx context.Context, token string) (email, avatarURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, http.NoBody)
	if err != nil {
		return "", "", fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch viewer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("fetch viewer: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUserBody))
	if err != nil {
		return "", "", fmt.Errorf("read viewer response: %w", err)
	}

	var u viewerUser
	if err := json.Unmarshal(body, &u); err != nil {
		return "", "", fmt.Errorf("parse viewer: %w", err)
	}
	if u.Email == "" {
		return "", "", fmt.Errorf("viewer email is empty")
	}
	return u.Email, u.Picture, nil
}

// AllowedLogin enforces the GOOGLE_ALLOWED_LOGIN restriction. When
// cfg.AllowedLogin is empty any authenticated user is accepted; otherwise the
// resolved email must match exactly (case-insensitive, since email is
// case-insensitive by convention).
func AllowedLogin(cfg OAuthConfig, email string) bool {
	if cfg.AllowedLogin == "" {
		return true
	}
	return strings.EqualFold(cfg.AllowedLogin, email)
}
