//go:build linux || darwin

package plugin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites"
	"github.com/geoffjay/jughead/sites/auth"

	"github.com/gin-gonic/gin"
)

// TestLoadAll_LoadsGithubPlugin loads the real plugins/github.so (built via
// `task plugin NAME=providers/github`) and verifies the GitHub provider registers into
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
		t.Skipf("plugin .so not built (%s); run `task plugin NAME=providers/github` first: %v", so, err)
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

// TestLoadAll_LoadsDocsSitePlugin loads the real plugins/docs.so (built via
// `task plugin NAME=sites/docs`) and verifies the docs site registers into the
// SiteManager. This is the Phase 4 validation that a site's templ.Component
// (Site.Template), its reverse-proxy fallback (Site.Proxy), and its route
// registrar (Site.Routes, carrying a *gin.RouterGroup) all cross the .so
// boundary with correct type identity — the harder case flagged in the plan
// because it shares the full templ runtime and gin router types with the host.
//
// The test is skipped when plugins/docs.so is not present.
func TestLoadAll_LoadsDocsSitePlugin(t *testing.T) {
	so := filepath.Join("..", "plugins", "docs.so")
	if _, err := os.Stat(so); err != nil {
		t.Skipf("plugin .so not built (%s); run `task plugin NAME=sites/docs` first: %v", so, err)
	}

	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	dir := filepath.Dir(so)
	if err := LoadAll(dir, true); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	const wantPath = "/sites/docs.jughead.geoffjay.com"
	s := sites.GetSiteManager().GetSite(wantPath)
	if s == nil {
		t.Fatalf("GetSite(%q) returned nil after LoadAll; expected the plugin to register the site", wantPath)
	}

	// The loader type-asserted the sdk.Site's any-typed fields against the
	// host's sites.Site field types. Verify the asserted fields came through:
	// Template (templ.Component) must be non-nil, Routes (the gin route
	// registrar) must be non-nil, and the site's path/url must match.
	if s.Template == nil {
		t.Error("registered site Template is nil; expected the plugin's IntroPlaceholder templ.Component to survive the .so boundary")
	}
	if s.Routes == nil {
		t.Error("registered site Routes is nil; expected the plugin's route registrar func to survive the .so boundary")
	}
	if s.Proxy == nil {
		t.Error("registered site Proxy is nil; expected the plugin's proxy func to survive the .so boundary")
	}
	if got, want := s.Path, wantPath; got != want {
		t.Errorf("registered site Path = %q, want %q", got, want)
	}
	if got, want := s.Url, "https://docs.jughead.geoffjay.com"; got != want {
		t.Errorf("registered site Url = %q, want %q", got, want)
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

// TestLoadAll_DocsSitePluginRendersPage is the full end-to-end Phase 4
// validation: load the docs site plugin, wire its routes onto a real gin
// engine via SiteManager.BuildSiteRoutes (the same wiring server.go uses),
// and HTTP-GET the docs site path. The response must be a 200 HTML page
// rendered through the plugin's templ.Component pipeline (the docs layout
// wrapping the intro page body).
//
// This proves the entire chain crosses the .so boundary intact:
//   - sdk.Site.Template (templ.Component) type-asserts and renders
//   - sdk.Site.Routes (func(*gin.RouterGroup, string)) registers handlers that
//     call back into the plugin's renderPage, which builds a doc.Config and
//     calls doc.Layout → returns a templ.Component the host renders
//   - the shared templates/layouts/documentation, templates/containers,
//     templates/components/daisyui, phosphor icons, and htmx-go packages all
//     resolve to the same type instances the host uses
//
// The test does NOT call sites.Initialize(), so only the plugin's site is
// registered (the in-repo docs site would otherwise overwrite it under the same
// path — the intended phased behavior where in-repo sites stay compiled-in).
//
// Skipped when plugins/docs.so is not built.
func TestLoadAll_DocsSitePluginRendersPage(t *testing.T) {
	so := filepath.Join("..", "plugins", "docs.so")
	if _, err := os.Stat(so); err != nil {
		t.Skipf("plugin .so not built (%s); run `task plugin NAME=sites/docs` first: %v", so, err)
	}

	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	dir := filepath.Dir(so)
	if err := LoadAll(dir, true); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := sessions.NewStore()

	// BuildSiteRoutes is the same wiring server.go uses. The docs site has
	// Auth == nil, so it's public: Routes is invoked on the site group.
	sites.GetSiteManager().BuildSiteRoutes(r, store, func(c *gin.Context) {
		c.String(http.StatusOK, "default-view")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sites/docs.jughead.geoffjay.com")
	if err != nil {
		t.Fatalf("GET docs site: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET docs site status = %d, want 200; body: %s", resp.StatusCode, body)
	}
	// The intro page renders an <h1>Introduction</h1> inside the documentation
	// layout. Its presence confirms the plugin's templ.Component rendered
	// through the host's gin handler end-to-end.
	if !strings.Contains(string(body), "Introduction") {
		t.Errorf("response body missing 'Introduction' heading; got: %s", body)
	}
	// The documentation layout wraps content in an AppShell with a data-appshell
	// root — confirms the shared templates/containers surface resolved.
	if !strings.Contains(string(body), `data-appshell="root"`) {
		t.Errorf("response body missing AppShell root; the shared layout surface did not render; got head: %s", string(body)[:min(500, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
