package config

import (
	"fmt"
	"os"
)

// OAuthConfig holds the GitHub OAuth App credentials and redirect URL read
// from the environment. The app authenticates via the standard authorization-
// code web flow (see https://docs.github.com/en/apps/oauth-apps).
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AllowedLogin string // when non-empty, only this GitHub login may sign in
}

// Env var names read by Load.
const (
	envClientID     = "GITHUB_CLIENT_ID"
	envClientSecret = "GITHUB_CLIENT_SECRET"
	envRedirectURL  = "GITHUB_REDIRECT_URL"
	envAllowedLogin = "GITHUB_ALLOWED_LOGIN"
)

// DefaultRedirectURL is used when GITHUB_REDIRECT_URL is unset. It assumes
// the app is being browsed via the FQDN reverse proxy (the /auth routes are
// mounted at the app root, not the site path).
const defaultRedirectURL = "/auth/callback"

// Load reads the OAuth config from the environment. ClientID and ClientSecret
// are required; an error is returned when either is missing. RedirectURL and
// AllowedLogin default to "" / unset respectively.
func Load() (OAuthConfig, error) {
	cfg := OAuthConfig{
		ClientID:     os.Getenv(envClientID),
		ClientSecret: os.Getenv(envClientSecret),
		RedirectURL:  getenvDefault(envRedirectURL, defaultRedirectURL),
		AllowedLogin: os.Getenv(envAllowedLogin),
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
