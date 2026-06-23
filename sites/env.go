package sites

import "os"

const (
	EnvTest        = "test"
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// CurrentEnv returns the current JUGHEAD_ENV value, defaulting to "development"
// when the variable is unset or empty.
func CurrentEnv() string {
	if env := os.Getenv("JUGHEAD_ENV"); env != "" {
		return env
	}
	return EnvDevelopment
}

// IsDevelopment reports whether the current environment is development.
func IsDevelopment() bool {
	return CurrentEnv() == EnvDevelopment
}

// IsProduction reports whether the current environment is production.
func IsProduction() bool {
	return CurrentEnv() == EnvProduction
}

// IsTest reports whether the current environment is test.
func IsTest() bool {
	return CurrentEnv() == EnvTest
}

// ShouldLoad reports whether a site with the given published flag should be
// loaded in the current environment.
//
// Published sites are loaded in every environment. Unpublished sites are only
// loaded in a development environment.
func ShouldLoad(published bool) bool {
	if published {
		return true
	}
	return IsDevelopment()
}