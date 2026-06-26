package github

import (
	"testing"

	"github.com/geoffjay/jughead/sites/auth"
)

func TestProvider_Name(t *testing.T) {
	if got, want := NewProvider().Name(), "github"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestProvider_Load_MissingEnvReturnsError(t *testing.T) {
	// Ensure the required env vars are unset for a deterministic result.
	t.Setenv(envClientID, "")
	t.Setenv(envClientSecret, "")

	p := NewProvider()
	_, err := p.Load(auth.AuthConfig{Provider: "github"})
	if err == nil {
		t.Fatal("Load with missing GITHUB_CLIENT_ID/SECRET returned nil error; want non-nil")
	}
}

func TestProvider_Load_CompleteEnvReturnsInstance(t *testing.T) {
	t.Setenv(envClientID, "test-id")
	t.Setenv(envClientSecret, "test-secret")
	t.Setenv(envRedirectURL, "") // exercise the default branch
	t.Setenv(envAllowedLogin, "")

	p := NewProvider()
	inst, err := p.Load(auth.AuthConfig{Provider: "github"})
	if err != nil {
		t.Fatalf("Load with complete env returned error: %v", err)
	}
	if inst == nil {
		t.Fatal("Load returned nil instance; want non-nil")
	}
}

func TestProvider_Load_AuthConfigOverridesEnv(t *testing.T) {
	t.Setenv(envClientID, "test-id")
	t.Setenv(envClientSecret, "test-secret")
	t.Setenv(envRedirectURL, "http://env.example/callback")
	t.Setenv(envAllowedLogin, "")

	p := NewProvider()
	inst, err := p.Load(auth.AuthConfig{
		Provider:     "github",
		RedirectURL:  "http://override.example/callback",
		AllowedLogin: "geoffjay",
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	gi, ok := inst.(*githubInstance)
	if !ok {
		t.Fatalf("Load returned %T, want *githubInstance", inst)
	}
	if got, want := gi.cfg.RedirectURL, "http://override.example/callback"; got != want {
		t.Errorf("RedirectURL override not applied; cfg.RedirectURL = %q, want %q", got, want)
	}
	if got, want := gi.cfg.AllowedLogin, "geoffjay"; got != want {
		t.Errorf("AllowedLogin override not applied; cfg.AllowedLogin = %q, want %q", got, want)
	}
}

func TestProvider_Load_DefaultRedirectURL(t *testing.T) {
	t.Setenv(envClientID, "test-id")
	t.Setenv(envClientSecret, "test-secret")
	t.Setenv(envRedirectURL, "")
	t.Setenv(envAllowedLogin, "")

	p := NewProvider()
	inst, err := p.Load(auth.AuthConfig{Provider: "github"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	gi := inst.(*githubInstance)
	if got, want := gi.cfg.RedirectURL, defaultRedirectURL; got != want {
		t.Errorf("cfg.RedirectURL = %q, want default %q", got, want)
	}
}

func TestLoadOAuthConfig_MissingClientID(t *testing.T) {
	t.Setenv(envClientID, "")
	t.Setenv(envClientSecret, "secret")
	if _, err := LoadOAuthConfig("", ""); err == nil {
		t.Error("LoadOAuthConfig with empty ClientID returned nil error; want non-nil")
	}
}

func TestLoadOAuthConfig_MissingClientSecret(t *testing.T) {
	t.Setenv(envClientID, "id")
	t.Setenv(envClientSecret, "")
	if _, err := LoadOAuthConfig("", ""); err == nil {
		t.Error("LoadOAuthConfig with empty ClientSecret returned nil error; want non-nil")
	}
}

func TestLoadOAuthConfig_EnvRedirectURLUsedWhenNoOverride(t *testing.T) {
	t.Setenv(envClientID, "id")
	t.Setenv(envClientSecret, "secret")
	t.Setenv(envRedirectURL, "http://env.example/cb")
	t.Setenv(envAllowedLogin, "")

	cfg, err := LoadOAuthConfig("", "")
	if err != nil {
		t.Fatalf("LoadOAuthConfig error: %v", err)
	}
	if got, want := cfg.RedirectURL, "http://env.example/cb"; got != want {
		t.Errorf("cfg.RedirectURL = %q, want %q", got, want)
	}
}
