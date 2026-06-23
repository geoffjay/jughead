package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestEffectiveHost_UsesHost(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "domain1.tld:9000"
	if got := effectiveHost(r); got != "domain1.tld" {
		t.Errorf("effectiveHost() = %q, want %q", got, "domain1.tld")
	}
}

func TestEffectiveHost_StripsPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "quux.geoffjay.com:443"
	if got := effectiveHost(r); got != "quux.geoffjay.com" {
		t.Errorf("effectiveHost() = %q, want %q", got, "quux.geoffjay.com")
	}
}

func TestEffectiveHost_NoPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "quux.geoffjay.com"
	if got := effectiveHost(r); got != "quux.geoffjay.com" {
		t.Errorf("effectiveHost() = %q, want %q", got, "quux.geoffjay.com")
	}
}

func TestEffectiveHost_XForwardedHostWins(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "localhost:9000"
	r.Header.Set("X-Forwarded-Host", "domain1.tld:9000")
	if got := effectiveHost(r); got != "domain1.tld" {
		t.Errorf("effectiveHost() with XFH = %q, want %q", got, "domain1.tld")
	}
}

func TestEffectiveHost_EmptyXFH_FallsBackToHost(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "localhost:9000"
	r.Header.Set("X-Forwarded-Host", "  ")
	if got := effectiveHost(r); got != "localhost" {
		t.Errorf("effectiveHost() with blank XFH = %q, want %q", got, "localhost")
	}
}

func TestNormalisePath(t *testing.T) {
	cases := []struct {
		name       string
		targetPath string
		joined     string
		want       string
	}{
		{"root request adds trailing slash", "/sites/domain1.tld", "/sites/domain1.tld/", "/sites/domain1.tld"},
		{"exact target no rest", "/sites/domain1.tld", "/sites/domain1.tld", "/sites/domain1.tld"},
		{"subpath preserved", "/sites/domain1.tld", "/sites/domain1.tld/foo", "/sites/domain1.tld/foo"},
		{"deeper subpath preserved", "/sites/domain1.tld", "/sites/domain1.tld/a/b/c", "/sites/domain1.tld/a/b/c"},
		{"no prefix passthrough", "/sites/domain1.tld", "/other/path", "/other/path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalisePath(tc.targetPath, tc.joined); got != tc.want {
				t.Errorf("normalisePath(%q, %q) = %q, want %q", tc.targetPath, tc.joined, got, tc.want)
			}
		})
	}
}

func TestReverseProxy_PanicsOnInvalidTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ReverseProxy did not panic on invalid target URL")
		}
	}()
	_ = ReverseProxy(map[string]string{"bad.host": "://missing-scheme"})
}

func TestReverseProxy_UnknownHostFallsThrough(t *testing.T) {
	called := false
	r := gin.New()
	r.Use(ReverseProxy(map[string]string{
		"known.tld": "http://localhost:9999/sites/known.tld",
	}))
	r.GET("/ping", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "pong")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Host = "localhost:9000"
	r.ServeHTTP(w, req)

	if !called {
		t.Error("downstream handler was not called for unknown host")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestReverseProxy_StaticPathBypassesProxy(t *testing.T) {
	// A request for /static/styles.css on a *known* proxied host should still
	// fall through to the app's own /static handler, not be forwarded.
	called := false
	r := gin.New()
	r.Use(ReverseProxy(map[string]string{
		"known.tld": "http://localhost:9999/sites/known.tld",
	}))
	r.GET("/static/styles.css", func(c *gin.Context) {
		called = true
		c.String(http.StatusOK, "css")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/static/styles.css", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "known.tld"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if !called {
		t.Error("static handler was not called for proxied host")
	}
}

func TestReverseProxy_ForwardsKnownHost(t *testing.T) {
	// Spin up a tiny upstream that records what it receives.
	upstreamHit := make(chan upstreamCall, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit <- upstreamCall{path: r.URL.Path, host: r.Host}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("from upstream"))
	}))
	defer upstream.Close()

	r := gin.New()
	r.Use(ReverseProxy(map[string]string{
		"proxy.example": upstream.URL,
	}))
	// No downstream routes needed; the proxy should handle it.

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "proxy.example"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	select {
	case call := <-upstreamHit:
		if !strings.Contains(call.path, "/foo") {
			t.Errorf("upstream path = %q, want it to contain /foo", call.path)
		}
	default:
		t.Fatal("upstream was not hit for known proxied host")
	}
}

type upstreamCall struct {
	path string
	host string
}
