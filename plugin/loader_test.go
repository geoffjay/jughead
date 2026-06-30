//go:build linux || darwin

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffjay/jughead/sites/auth"
)

// TestLoadAll_LoadsGithubPlugin loads the real plugins/github.so (built via
// `make plugin NAME=github`) and verifies the GitHub provider registers into
// the auth registry. This is the end-to-end validation that:
//   - plugin.Open succeeds on a .so built from the same module/toolchain
//   - the "Plugin" symbol lookup yields *sdk.Plugin (same type identity)
//   - the provider's Name() comes through as "github"
//
// The test is skipped when plugins/github.so is not present (e.g. in CI that
// only runs `go test` without building plugins first), so it doesn't gate the
// normal test suite.
func TestLoadAll_LoadsGithubPlugin(t *testing.T) {
	so := filepath.Join("..", "plugins", "github.so")
	if _, err := os.Stat(so); err != nil {
		t.Skipf("plugin .so not built (%s); run `make plugin NAME=github` first: %v", so, err)
	}

	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	dir := filepath.Dir(so)
	if err := LoadAll(dir, true); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	p, ok := auth.Get("github")
	if !ok {
		t.Fatal("auth.Get(github) returned ok=false after LoadAll; expected the plugin to register the provider")
	}
	if got, want := p.Name(), "github"; got != want {
		t.Errorf("registered provider Name() = %q, want %q", got, want)
	}
}

// TestLoadAll_MissingDirIsNoOp verifies a non-existent plugins dir is silently
// skipped (the default deployment has no plugins).
func TestLoadAll_MissingDirIsNoOp(t *testing.T) {
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	if err := LoadAll("/nonexistent/plugins/dir", false); err != nil {
		t.Errorf("LoadAll on missing dir returned error: %v; want nil (skip)", err)
	}
}

// TestLoadAll_EmptyDirIsNoOp verifies an empty plugins dir registers nothing.
func TestLoadAll_EmptyDirIsNoOp(t *testing.T) {
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	dir := t.TempDir()
	if err := LoadAll(dir, false); err != nil {
		t.Errorf("LoadAll on empty dir returned error: %v", err)
	}
	if _, ok := auth.Get("github"); ok {
		t.Error("auth.Get(github) returned ok=true on empty plugins dir; expected no registration")
	}
}

// TestLoadAll_BadFileSkippedNonStrict verifies a non-plugin file in the
// plugins dir is logged and skipped (not fatal) when strict is false.
func TestLoadAll_BadFileSkippedNonStrict(t *testing.T) {
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	dir := t.TempDir()
	bad := filepath.Join(dir, "bogus.so")
	if err := os.WriteFile(bad, []byte("not a plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-strict: error is logged, not returned.
	if err := LoadAll(dir, false); err != nil {
		t.Errorf("LoadAll with bad file in non-strict mode returned error: %v; want nil", err)
	}
}

// TestLoadAll_BadFileFailsStrict verifies the same bad file is fatal when
// strict is true.
func TestLoadAll_BadFileFailsStrict(t *testing.T) {
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	dir := t.TempDir()
	bad := filepath.Join(dir, "bogus.so")
	if err := os.WriteFile(bad, []byte("not a plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadAll(dir, true); err == nil {
		t.Error("LoadAll with bad file in strict mode returned nil; want error")
	}
}
