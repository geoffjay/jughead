package github

import (
	"fmt"
	"os"
)

// OAuthConfig holds the GitHub OAuth App credentials and redirect URL read
// from the environment. The app authenticates via the standard authorization-
// code web flow (see https://docs.github.com/en/apps/oauth-apps).
//
// This type previously lived in sites/com/geoffjay/quux/config; it moved here
// so services/github owns its own config (and the sites layer no longer has to
// expose a config package for a single provider). The quux site references it
// indirectly through the auth.AuthConfig / provider registry.
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