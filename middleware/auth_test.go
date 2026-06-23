package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geoffjay/jughead/sessions"

	"github.com/gin-gonic/gin"
)

func TestAuthRequired_RedirectsWhenNoSession(t *testing.T) {
	store := sessions.NewStore()
	r := gin.New()
	protected := r.Group("/admin", AuthRequired(store))
	protected.GET("", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	protected.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Don't follow redirects.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want %d (redirect to login)", resp.StatusCode, http.StatusFound)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		t.Error("missing Location header on redirect")
	}
	if !strings.Contains(loc, "/login?redirect=") {
		t.Errorf("Location = %q, want it to contain /login?redirect=", loc)
	}
	if !strings.Contains(loc, "/admin") {
		t.Errorf("Location = %q, want it to preserve /admin as redirect target", loc)
	}
}

func TestAuthRequired_PreservesQueryInRedirect(t *testing.T) {
	store := sessions.NewStore()
	r := gin.New()
	protected := r.Group("/admin", AuthRequired(store))
	protected.GET("/page", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/admin/page?x=1&y=2", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "x=1") || !strings.Contains(loc, "y=2") {
		t.Errorf("Location = %q, want it to preserve query params", loc)
	}
}

func TestAuthRequired_AllowsWhenSessionExists(t *testing.T) {
	store := sessions.NewStore()
	rec := httptest.NewRecorder()
	token := store.Create(rec, "admin")

	r := gin.New()
	reached := false
	protected := r.Group("/admin", AuthRequired(store))
	protected.GET("", func(c *gin.Context) {
		reached = true
		if u, exists := c.Get("username"); !exists || u != "admin" {
			t.Errorf("c.Get(\"username\") = %v, exists=%v, want \"admin\"", u, exists)
		}
		c.String(http.StatusOK, "ok")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessions.CookieName(), Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if !reached {
		t.Error("protected handler was not reached despite valid session")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAuthRequired_AllowsLoginPathWithoutSession(t *testing.T) {
	store := sessions.NewStore()
	r := gin.New()
	// Attach the middleware directly to /login to verify the loop-avoidance
	// branch: even without a session, /login should pass through.
	r.GET("/login", AuthRequired(store), func(c *gin.Context) {
		c.String(http.StatusOK, "login form")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/login", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d (login path should be allowed)", resp.StatusCode, http.StatusOK)
	}
}
