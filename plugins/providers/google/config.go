// config.go holds the Google OAuth credentials read from the environment. It
// reads GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / GOOGLE_REDIRECT_URL /
// GOOGLE_ALLOWED_LOGIN from the environment, mirroring the GitHub provider's
// config.go pattern.
package main

import (
	"fmt"
	"os"
)

// OAuthConfig holds the Google OAuth App credentials and redirect URL read
// from the environment.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AllowedLogin string // when non-empty, only this Google email may sign in
}

// Env var names read by LoadOAuthConfig.
const (
	envClientID     = "GOOGLE_CLIENT_ID"
	envClientSecret = "GOOGLE_CLIENT_SECRET"
	envRedirectURL  = "GOOGLE_REDIRECT_URL"
	envAllowedLogin = "GOOGLE_ALLOWED_LOGIN"
)

// defaultRedirectURL is used when GOOGLE_REDIRECT_URL is unset. Routes are
// namespaced under /auth/google/, so the callback lives at
// /auth/google/callback (mounted by the provider instance).
const defaultRedirectURL = "/auth/google/callback"

// LoadOAuthConfig reads the Google OAuth config from the environment. ClientID
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
