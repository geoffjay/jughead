// config.go is a vendored copy of services/github/config.go, trimmed to the
// OAuth credentials the provider needs. It reads GITHUB_CLIENT_ID /
// GITHUB_CLIENT_SECRET / GITHUB_REDIRECT_URL / GITHUB_ALLOWED_LOGIN from the
// environment. Vendoring keeps the plugin self-contained so deployments that
// want GitHub auth via plugin don't also pull in the quux review UI's GitHub
// API client (services/github/api.go).
package main

import (
	"fmt"
	"os"
)

// OAuthConfig holds the GitHub OAuth App credentials and redirect URL read
// from the environment.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AllowedLogin string // when non-empty, only this GitHub login may sign in
}

// Env var names read by LoadOAuthConfig.
const (
	envClientID     = "GITHUB_CLIENT_ID"
	envClientSecret = "GITHUB_CLIENT_SECRET"
	envRedirectURL  = "GITHUB_REDIRECT_URL"
	envAllowedLogin = "GITHUB_ALLOWED_LOGIN"
)

// defaultRedirectURL is used when GITHUB_REDIRECT_URL is unset. Routes are
// namespaced under /auth/github/, so the callback lives at
// /auth/github/callback (mounted by the provider instance).
const defaultRedirectURL = "/auth/github/callback"

// LoadOAuthConfig reads the GitHub OAuth config from the environment. ClientID
// and ClientSecret are required; an error is returned when either is missing.
// RedirectURL and AllowedLogin default to defaultRedirectURL / "".
//
// siteOverride, when non-empty, takes precedence over the environment for the
// RedirectURL and AllowedLogin fields — it lets a site's AuthConfig override
// the provider-wide defaults without the provider knowing about sites.
func LoadOAuthConfig(redirectOverride, allowedOverride string) (OAuthConfig, error) {
	cfg := OAuthConfig{
		ClientID:     os.Getenv(envClientID),
		ClientSecret: os.Getenv(envClientSecret),
		RedirectURL:  getenvDefault(envRedirectURL, defaultRedirectURL),
		AllowedLogin: os.Getenv(envAllowedLogin),
	}
	if redirectOverride != "" {
		cfg.RedirectURL = redirectOverride
	}
	if allowedOverride != "" {
		cfg.AllowedLogin = allowedOverride
	}
	if cfg.ClientID == "" {
		return cfg, fmt.Errorf("%s is not set", envClientID)
	}
	if cfg.ClientSecret == "" {
		return cfg, fmt.Errorf("%s is not set", envClientSecret)
	}
	return cfg, nil
}

// getenvDefault returns the env var value or def when unset/empty.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
