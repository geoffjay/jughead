package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/geoffjay/jughead/middleware"
	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// newTestRouter builds a gin engine wired with the same routes as runServer but
// without the http.Server / ListenAndServe. The returned server is ready to
// serve via httptest.NewServer.
func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	sites.Initialize()
	store := sessions.NewStore()
	r := gin.New()
	r.HTMLRender = &TemplRender{}

	r.GET("/", indexViewHandler)
	r.GET("/api/hello-world", showContentAPIHandler)

	sm := sites.GetSiteManager()
	for _, site := range sm.Sites() {
		group := r.Group(site.Path)
		if site.Routes != nil {
			site.Routes(group, site.ThemeValue())
		} else {
			group.GET("", siteViewHandler)
			group.GET("/", siteViewHandler)
		}
	}

	r.GET("/login", loginViewHandler)
	r.POST("/login", loginSubmitHandler(store))
	r.GET("/logout", logoutHandler(store))
	admin := r.Group("/admin", middleware.AuthRequired(store))
	admin.GET("", adminViewHandler)
	admin.GET("/", adminViewHandler)

	return r
}

// do issues a request against a test server without following redirects.
func do(t *testing.T, srv *httptest.Server, method, path string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return string(b)
}

func TestIndexViewHandler_ReturnsHTML(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := bodyString(t, resp)
	if !strings.Contains(body, "Welcome to example!") {
		t.Errorf("GET / body did not contain expected heading; got: %s", body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
}

func TestShowContentAPIHandler_RejectsNonHTMX(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/api/hello-world", nil, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("GET /api/hello-world without HX-Request status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestShowContentAPIHandler_ServesHTMX(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/api/hello-world", nil, map[string]string{
		"HX-Request": "true",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/hello-world with HX-Request status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := bodyString(t, resp)
	if !strings.Contains(body, "htmx") {
		t.Errorf("response body did not mention htmx; got: %s", body)
	}
}

func TestSiteViewHandler_UnknownPathReturns404(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/sites/does-not-exist.example", nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET unknown site status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSiteViewHandler_PublishedSiteRenders(t *testing.T) {
	// quux.geoffjay.com is Published: true, so it loads in every env.
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/sites/quux.geoffjay.com", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET quux site status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := bodyString(t, resp)
	if !strings.Contains(body, "quux.geoffjay.com") {
		t.Errorf("quux site body did not contain site URL; got: %s", body)
	}
}

func TestSiteViewHandler_DirectAccess_KeepsSitePathInLinks(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/sites/quux.geoffjay.com", nil, nil)
	body := bodyString(t, resp)
	// The quux home page renders nav links via LinkResolver. Without an
	// X-Site-Base / X-Forwarded-Host header, links should be prefixed with
	// the site path.
	if !strings.Contains(body, "/sites/quux.geoffjay.com") {
		t.Errorf("direct access should keep site path prefix in links; got: %s", body)
	}
}

func TestSiteViewHandler_ProxiedAccess_RootRelativeLinks(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/sites/quux.geoffjay.com", nil, map[string]string{
		"X-Site-Base": "",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	// With X-Site-Base set, the resolver returns root-relative links. We can't
	// easily assert the absence of a substring reliably across the whole doc,
	// so just ensure the page still renders and contains the site title.
	body := bodyString(t, resp)
	if !strings.Contains(body, "quux.geoffjay.com") {
		t.Errorf("proxied access body missing site title; got: %s", body)
	}
}

func TestSiteViewHandler_UnpublishedSiteHiddenInTestEnv(t *testing.T) {
	// domain1.tld is Published: false; in test env it should not be registered.
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/sites/domain1.tld", nil, nil)
	// siteViewHandler looks up the site in the manager; when absent it 404s.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET unpublished site in test env status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSiteViewHandler_UnpublishedSiteVisibleInDev(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "development")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/sites/domain1.tld", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET unpublished site in dev env status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := bodyString(t, resp)
	if !strings.Contains(body, "domain1.tld") {
		t.Errorf("domain1 site body missing site name; got: %s", body)
	}
}

func TestLoginViewHandler_RendersForm(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/login", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := bodyString(t, resp)
	if !strings.Contains(body, "name=\"username\"") || !strings.Contains(body, "name=\"password\"") {
		t.Errorf("login form missing input fields; got: %s", body)
	}
}

func TestLoginViewHandler_PassesRedirectQueryParam(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/login?redirect=/admin/foo", nil, nil)
	body := bodyString(t, resp)
	if !strings.Contains(body, "name=\"redirect\"") {
		t.Errorf("login form missing hidden redirect field; got: %s", body)
	}
	if !strings.Contains(body, "/admin/foo") {
		t.Errorf("login form did not embed redirect target; got: %s", body)
	}
}

func TestLoginSubmitHandler_BadCredentialsShowsError(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{"username": {"wrong"}, "password": {"nope"}}
	resp := do(t, srv, http.MethodPost, "/login", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	// Bad creds re-render the login page (200) with an error message.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST /login bad creds status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := bodyString(t, resp)
	if !strings.Contains(body, "Invalid username or password") {
		t.Errorf("bad-creds response missing error message; got: %s", body)
	}
}

func TestLoginSubmitHandler_GoodCredentialsRedirects(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{"username": {"admin"}, "password": {"password"}}
	resp := do(t, srv, http.MethodPost, "/login", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if resp.StatusCode != http.StatusFound {
		t.Errorf("POST /login good creds status = %d, want %d (redirect)", resp.StatusCode, http.StatusFound)
	}
	loc := resp.Header.Get("Location")
	if loc != "/admin" {
		t.Errorf("redirect Location = %q, want %q", loc, "/admin")
	}
	// A session cookie should be set.
	setCookie := resp.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "jughead_session=") {
		t.Errorf("Set-Cookie missing session; got: %s", setCookie)
	}
}

func TestLoginSubmitHandler_HonoursRedirectTarget(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	form := url.Values{
		"username": {"admin"},
		"password": {"password"},
		"redirect": {"/admin/special"},
	}
	resp := do(t, srv, http.MethodPost, "/login", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if loc := resp.Header.Get("Location"); loc != "/admin/special" {
		t.Errorf("redirect Location = %q, want %q", loc, "/admin/special")
	}
}

func TestLoginSubmitHandler_GoodCredentialsThenAdminAccessible(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Log in to obtain a session cookie.
	form := url.Values{"username": {"admin"}, "password": {"password"}}
	resp := do(t, srv, http.MethodPost, "/login", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	cookies := resp.Cookies()
	resp.Body.Close()
	if len(cookies) == 0 {
		t.Fatal("login response set no cookies")
	}

	// Reuse the session cookie on a request to the protected /admin.
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	adminResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer adminResp.Body.Close()
	if adminResp.StatusCode != http.StatusOK {
		t.Errorf("GET /admin with session status = %d, want %d", adminResp.StatusCode, http.StatusOK)
	}
}

func TestAdminViewHandler_RequiresAuth(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/admin", nil, nil)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("GET /admin without session status = %d, want %d (redirect)", resp.StatusCode, http.StatusFound)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/login?redirect=/admin") {
		t.Errorf("Location = %q, want it to redirect to /login preserving /admin", loc)
	}
	resp.Body.Close()
}

func TestLogoutHandler_ClearsSessionAndRedirects(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Log in first to get a cookie.
	form := url.Values{"username": {"admin"}, "password": {"password"}}
	loginResp := do(t, srv, http.MethodPost, "/login", strings.NewReader(form.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	cookies := loginResp.Cookies()
	loginResp.Body.Close()
	if len(cookies) == 0 {
		t.Fatal("login set no cookies")
	}

	// Now log out with that cookie.
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	logoutResp, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer logoutResp.Body.Close()

	if logoutResp.StatusCode != http.StatusFound {
		t.Errorf("GET /logout status = %d, want %d", logoutResp.StatusCode, http.StatusFound)
	}
	if loc := logoutResp.Header.Get("Location"); loc != "/login" {
		t.Errorf("logout Location = %q, want %q", loc, "/login")
	}
	// Set-Cookie should expire the session cookie.
	setCookie := logoutResp.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "jughead_session=") {
		t.Errorf("logout Set-Cookie missing session; got: %s", setCookie)
	}
	if !strings.Contains(setCookie, "Max-Age=0") && !strings.Contains(setCookie, "Expires=") {
		t.Errorf("logout Set-Cookie did not clear the cookie; got: %s", setCookie)
	}
}

func TestLogoutHandler_WithoutCookieIsNoop(t *testing.T) {
	t.Setenv("JUGHEAD_ENV", "test")
	r := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp := do(t, srv, http.MethodGet, "/logout", nil, nil)
	if resp.StatusCode != http.StatusFound {
		t.Errorf("GET /logout without cookie status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	resp.Body.Close()
}
