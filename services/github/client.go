// Package github implements the GitHub OAuth web flow and a thin typed REST
// client over the subset of GitHub API endpoints the quux review UI needs.
//
// The OAuth flow follows the standard authorization-code grant:
//
//  1. GET /auth/login  -> redirect to GitHub authorize URL (with state)
//  2. GitHub redirects to GITHUB_REDIRECT_URL with a code
//  3. GET /auth/callback -> exchange code for an access token, fetch /user,
//     create a session.
//
// The client wraps net/http (no third-party dependency) and maps GitHub JSON
// responses into the quux data package's domain types.
package github

import (
	"context"
	"fmt"
	"net/http"
)

// githubAPIBase is the REST API root. Overridable in tests via Client.base.
const githubAPIBase = "https://api.github.com"

// userAgent identifies the app to the GitHub API (required by GitHub's
// Acceptable Use Policy). Keep in sync with the API version header.
const (
	userAgent   = "jughead-quux/1.0"
	apiVersion  = "2022-11-28"
	acceptMedia = "application/vnd.github+json"
)

// Body-size caps to protect against unbounded responses. The token and user
// endpoints are tiny JSON payloads; listing endpoints are paged and bounded
// by perPage constants in api.go.
const (
	maxTokenBody = 1 << 14 // 16 KiB
	maxUserBody  = 1 << 14 // 16 KiB
	maxAPIBody   = 1 << 20 // 1 MiB per response (paged)
)

// Client is a minimal typed GitHub REST API client. It is safe for concurrent
// use; the underlying *http.Client is reused across calls.
type Client struct {
	token string
	http  *http.Client
	base  string // base URL; defaults to githubAPIBase, overridden in tests
}

// NewClient builds a Client authenticated with the given bearer token. The
// caller is responsible for storing/refreshing the token; the Client holds it
// for its lifetime.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{},
		base:  githubAPIBase,
	}
}

// newRequest constructs an authenticated GET to base+path with the standard
// GitHub API headers. The path must begin with "/". ctx is attached to the
// request so calls can be canceled/bounded by the caller.
func (c *Client) newRequest(ctx context.Context, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", acceptMedia)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}