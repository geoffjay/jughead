package sites

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites/auth"

	"github.com/gin-gonic/gin"
)

// resetSingleton clears the package-level singleton so each test starts clean.
func resetSingleton(t *testing.T) {
	t.Helper()
	lock.Lock()
	instance = nil
	lock.Unlock()
}

// noopView is a stand-in default view for BuildSiteRoutes tests. It writes a
// fixed marker so tests can detect that the fallback path ran.
func noopView(c *gin.Context) {
	c.String(http.StatusOK, "default-view")
}

func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// --- BuildProxyTargets ---

func TestBuildProxyTargets_DerivesFromSites(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	sm.RegisterSite("/sites/quux.geoffjay.com", &Site{
		Path: "/sites/quux.geoffjay.com",
		Url:  "https://quux.geoffjay.com",
	})
	sm.RegisterSite("/sites/domain1.tld", &Site{
		Path: "/sites/domain1.tld",
		Url:  "http://domain1.tld",
	})

	targets := sm.BuildProxyTargets(9000)
	if got, want := targets["quux.geoffjay.com"], "http://localhost:9000/sites/quux.geoffjay.com"; got != want {
		t.Errorf("quux target = %q, want %q", got, want)
	}
	if got, want := targets["domain1.tld"], "http://localhost:9000/sites/domain1.tld"; got != want {
		t.Errorf("domain1 target = %q, want %q", got, want)
	}
}

func TestBuildProxyTargets_SkipsEmptyOrUnparseableURL(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	sm.RegisterSite("/sites/empty", &Site{Path: "/sites/empty", Url: ""})
	sm.RegisterSite("/sites/bad", &Site{Path: "/sites/bad", Url: "://no-scheme"})

	targets := sm.BuildProxyTargets(8080)
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for unusable URLs; got %v", targets)
	}
}

func TestBuildProxyTargets_StripsPortFromURL(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	sm.RegisterSite("/sites/porty", &Site{
		Path: "/sites/porty",
		Url:  "http://host.example:8443",
	})
	targets := sm.BuildProxyTargets(9000)
	if _, ok := targets["host.example"]; !ok {
		t.Errorf("expected host.example (no port) in targets; got %v", targets)
	}
}

func TestHostFromURL(t *testing.T) {
	cases := map[string]string{
		"https://quux.geoffjay.com":      "quux.geoffjay.com",
		"http://domain1.tld":             "domain1.tld",
		"http://host.example:8443":       "host.example",
		"":                               "",
		"://missing-scheme":              "",
		"https://quux.geoffjay.com/path": "quux.geoffjay.com",
	}
	for in, want := range cases {
		if got := hostFromURL(in); got != want {
			t.Errorf("hostFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- BuildSiteRoutes (no auth) ---

func TestBuildSiteRoutes_AuthNil_RegistersRoutesOrFallback(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	routed := false
	sm.RegisterSite("/sites/plain", &Site{
		Path:  "/sites/plain",
		Url:   "http://plain.example",
		Theme: "light",
		Routes: func(g *gin.RouterGroup, _ string) {
			routed = true
			g.GET("", func(c *gin.Context) { c.String(http.StatusOK, "plain-home") })
		},
	})
	sm.RegisterSite("/sites/noroutes", &Site{
		Path: "/sites/noroutes",
		Url:  "http://noroutes.example",
	})

	r := newTestEngine()
	sm.BuildSiteRoutes(r, sessions.NewStore(), noopView)

	if !routed {
		t.Error("site.Routes was not invoked for the Routes-bearing site")
	}

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Routes-bearing site serves its own handler.
	resp, _ := http.Get(srv.URL + "/sites/plain")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("plain site status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Site without Routes falls back to the default view.
	resp, _ = http.Get(srv.URL + "/sites/noroutes")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("noroutes site status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- BuildSiteRoutes (auth) ---

// fakeProvider / fakeInstance let the manager tests exercise the auth wiring
// without pulling in the real GitHub provider (which needs env vars). They
// record what the manager asked them to do so assertions can follow.
type fakeInstance struct {
	routed          bool
	middlewareCalls int
}

func (f *fakeInstance) RegisterAuthRoutes(router gin.IRouter, _ *sessions.Store) {
	f.routed = true
	router.GET("/auth/fake/login", func(c *gin.Context) { c.String(http.StatusOK, "fake-login") })
}

func (f *fakeInstance) AuthMiddleware(_ *sessions.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		f.middlewareCalls++
		c.Next()
	}
}

type fakeProvider struct {
	loadErr error
	inst    *fakeInstance
}

func (fakeProvider) Name() string { return "fake" }
func (p fakeProvider) Load(auth.AuthConfig) (auth.ProviderInstance, error) {
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	return p.inst, nil
}

func TestBuildSiteRoutes_AuthIncomplete_FallsBackToDefaultView(t *testing.T) {
	resetSingleton(t)
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	auth.Register(fakeProvider{loadErr: errIncomplete})
	sm := GetSiteManager()
	sm.RegisterSite("/sites/protected", &Site{
		Path: "/sites/protected",
		Url:  "http://protected.example",
		Auth: &auth.AuthConfig{Provider: "fake"},
	})

	r := newTestEngine()
	sm.BuildSiteRoutes(r, sessions.NewStore(), noopView)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/sites/protected")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("incomplete-config site status = %d, want 200 (fallback view)", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestBuildSiteRoutes_UnknownProvider_FallsBackToDefaultView(t *testing.T) {
	resetSingleton(t)
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	sm := GetSiteManager()
	sm.RegisterSite("/sites/unknown", &Site{
		Path: "/sites/unknown",
		Url:  "http://unknown.example",
		Auth: &auth.AuthConfig{Provider: "no-such-provider"},
	})

	r := newTestEngine()
	sm.BuildSiteRoutes(r, sessions.NewStore(), noopView)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/sites/unknown")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unknown-provider site status = %d, want 200 (fallback view)", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestBuildSiteRoutes_AuthComplete_AppliesMiddlewareAndCallsRoutes(t *testing.T) {
	resetSingleton(t)
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	inst := &fakeInstance{}
	auth.Register(fakeProvider{inst: inst})
	sm := GetSiteManager()
	sm.RegisterSite("/sites/protected", &Site{
		Path: "/sites/protected",
		Url:  "http://protected.example",
		Auth: &auth.AuthConfig{Provider: "fake"},
		Routes: func(g *gin.RouterGroup, _ string) {
			g.GET("", func(c *gin.Context) { c.String(http.StatusOK, "protected-home") })
		},
	})

	r := newTestEngine()
	sm.BuildSiteRoutes(r, sessions.NewStore(), noopView)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/sites/protected")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("protected site status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	if !inst.routed {
		t.Error("provider RegisterAuthRoutes was not invoked")
	}
	if inst.middlewareCalls == 0 {
		t.Error("provider AuthMiddleware was not applied (0 calls)")
	}
}

func TestBuildSiteRoutes_DedupsAuthRoutesAcrossSites(t *testing.T) {
	resetSingleton(t)
	auth.ResetForTest()
	t.Cleanup(auth.ResetForTest)

	inst := &fakeInstance{}
	auth.Register(fakeProvider{inst: inst})
	sm := GetSiteManager()
	// Two sites using the same provider — auth routes should mount once.
	sm.RegisterSite("/sites/a", &Site{
		Path:   "/sites/a",
		Url:    "http://a.example",
		Auth:   &auth.AuthConfig{Provider: "fake"},
		Routes: func(g *gin.RouterGroup, _ string) { g.GET("", func(c *gin.Context) {}) },
	})
	sm.RegisterSite("/sites/b", &Site{
		Path:   "/sites/b",
		Url:    "http://b.example",
		Auth:   &auth.AuthConfig{Provider: "fake"},
		Routes: func(g *gin.RouterGroup, _ string) { g.GET("", func(c *gin.Context) {}) },
	})

	r := newTestEngine()
	sm.BuildSiteRoutes(r, sessions.NewStore(), noopView)

	// RegisterAuthRoutes should have been called exactly once even though two
	// sites requested the fake provider.
	if inst.routed != true {
		t.Error("RegisterAuthRoutes was never called")
	}

	// Hit the login route once; gin would panic with a duplicate registration
	// at startup if the manager had mounted it twice, so a 200 confirms dedup.
	srv := httptest.NewServer(r)
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/auth/fake/login")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("deduped auth route status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// errIncomplete is a sentinel error for the fake provider's Load failure path.
var errIncomplete = &incompleteErr{}

type incompleteErr struct{}

func (*incompleteErr) Error() string { return "fake: config incomplete" }
