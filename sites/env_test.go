package sites

import (
	"testing"
)

func TestCurrentEnv_DefaultDevelopment(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "")
	if got := CurrentEnv(); got != EnvDevelopment {
		t.Errorf("CurrentEnv() = %q, want %q", got, EnvDevelopment)
	}
}

func TestCurrentEnv_Explicit(t *testing.T) {
	cases := map[string]string{
		"production":  EnvProduction,
		"test":        EnvTest,
		"development": EnvDevelopment,
		"custom":      "custom",
	}
	for val, want := range cases {
		t.Run(val, func(t *testing.T) {
			t.Setenv("JUGHEAD_ENV", val)
			if got := CurrentEnv(); got != want {
				t.Errorf("CurrentEnv() = %q, want %q", got, want)
			}
		})
	}
}

func TestIsDevelopment(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "development")
	if !IsDevelopment() {
		t.Error("IsDevelopment() = false, want true")
	}
	t.Setenv("JUGHEAD_ENV", "production")
	if IsDevelopment() {
		t.Error("IsDevelopment() = true, want false")
	}
}

func TestIsProduction(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "production")
	if !IsProduction() {
		t.Error("IsProduction() = false, want true")
	}
	t.Setenv("JUGHEAD_ENV", "development")
	if IsProduction() {
		t.Error("IsProduction() = true, want false")
	}
}

func TestIsTest(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	if !IsTest() {
		t.Error("IsTest() = false, want true")
	}
	t.Setenv("JUGHEAD_ENV", "development")
	if IsTest() {
		t.Error("IsTest() = true, want false")
	}
}

func TestShouldLoad(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "development")
	if !ShouldLoad(true) {
		t.Error("published site should load in development")
	}
	if !ShouldLoad(false) {
		t.Error("unpublished site should load in development")
	}

	t.Setenv("JUGHEAD_ENV", "production")
	if !ShouldLoad(true) {
		t.Error("published site should load in production")
	}
	if ShouldLoad(false) {
		t.Error("unpublished site should NOT load in production")
	}

	t.Setenv("JUGHEAD_ENV", "test")
	if !ShouldLoad(true) {
		t.Error("published site should load in test")
	}
	if ShouldLoad(false) {
		t.Error("unpublished site should NOT load in test")
	}
}
