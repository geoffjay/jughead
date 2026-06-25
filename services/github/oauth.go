package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/config"
)

// OAuth endpoints on github.com (not api.github.com).
const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
)

// OAuthScopes requested from the user. "repo" covers reading private repo PRs
// and diffs; "read:user" resolves the viewer's profile; "read:org" lets the
// app list collaborators on org repos.
const oauthScopes = "repo read:user read:org"

// tokenResponse models the JSON returned by the access-token exchange
// endpoint (when called with Accept: application/json).
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// viewerUser models the subset of GET /user fields we need.
type viewerUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// AuthorizeRedirect builds the GitHub authorize URL and returns it for the
// caller to issue a 302 with. state is an opaque CSRF token the caller must
// persist (cookie) and verify in the callback.
func AuthorizeRedirect(cfg config.OAuthConfig, state string) string {
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURL)
	q.Set("scope", oauthScopes)
	q.Set("state", state)
	return githubAuthorizeURL + "?" + q.Encode()
}

// ExchangeCode POSTs the authorization code to GitHub and returns the access
// token. Called from the OAuth callback handler.
func ExchangeCode(ctx context.Context, cfg config.OAuthConfig, code string) (string, error) {
	form := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {cfg.RedirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Accept", acceptMedia)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

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
		return "", fmt.Errorf("github: %s: %s", tr.Error, tr.ErrorDesc)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("github: empty access token")
	}
	return tr.AccessToken, nil
}

// FetchViewer calls GET /user with the freshly minted token and returns the
// viewer's login and avatar URL.
func FetchViewer(ctx context.Context, token string) (login, avatarURL string, err error) {
	client := NewClient(token)
	req, err := client.newRequest(ctx, "/user")
	if err != nil {
		return "", "", err
	}

	resp, err := client.http.Do(req)
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
	if u.Login == "" {
		return "", "", fmt.Errorf("viewer login is empty")
	}
	return u.Login, u.AvatarURL, nil
}

// AllowedLogin enforces the GITHUB_ALLOWED_LOGIN restriction. When
// cfg.AllowedLogin is empty any authenticated user is accepted; otherwise the
// resolved login must match exactly.
func AllowedLogin(cfg config.OAuthConfig, login string) bool {
	if cfg.AllowedLogin == "" {
		return true
	}
	return strings.EqualFold(cfg.AllowedLogin, login)
}