// client.go is a vendored, trimmed copy of services/github/client.go. Only the
// minimal REST client surface the OAuth flow needs is kept: NewClient,
// newRequest, and the shared constants/headers. The full services/github
// client also exposes quux-specific endpoints (ListAssignedPRs, LoadPR, etc.)
// which are not part of the auth provider contract and would drag the quux
// data layer into the plugin.
package main

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
	userAgent   = "jughead/1.0"
	apiVersion  = "2022-11-28"
	acceptMedia = "application/vnd.github+json"
)

// Body-size caps to protect against unbounded responses.
const (
	maxTokenBody = 1 << 14 // 16 KiB
	maxUserBody  = 1 << 14 // 16 KiB
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
