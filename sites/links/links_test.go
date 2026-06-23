package links

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
)

func newReq() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/", nil)
}

func TestNewDirectResolver_Base(t *testing.T) {
	r := NewDirectResolver("/sites/domain1.tld")
	if got := r.Base(); got != "/sites/domain1.tld" {
		t.Errorf("Base() = %q, want %q", got, "/sites/domain1.tld")
	}
}

func TestNewDirectResolver_TrimsTrailingSlash(t *testing.T) {
	r := NewDirectResolver("/sites/domain1.tld/")
	if got := r.Base(); got != "/sites/domain1.tld" {
		t.Errorf("Base() = %q, want %q (trailing slash trimmed)", got, "/sites/domain1.tld")
	}
}

func TestNewDirectResolver_EmptyPath(t *testing.T) {
	r := NewDirectResolver("")
	if got := r.Base(); got != "" {
		t.Errorf("Base() = %q, want empty", got)
	}
}

func TestNewLinkResolver_NoHeaders_KeepsSitePath(t *testing.T) {
	r := NewLinkResolver(newReq(), "/sites/domain1.tld")
	if got := r.Base(); got != "/sites/domain1.tld" {
		t.Errorf("Base() without headers = %q, want %q", got, "/sites/domain1.tld")
	}
}

func TestNewLinkResolver_WithXSiteBase_RootRelative(t *testing.T) {
	req := newReq()
	req.Header.Set("X-Site-Base", "")
	r := NewLinkResolver(req, "/sites/domain1.tld")
	if got := r.Base(); got != "" {
		t.Errorf("Base() with X-Site-Base = %q, want empty", got)
	}
}

func TestNewLinkResolver_WithXForwardedHost_RootRelative(t *testing.T) {
	req := newReq()
	req.Header.Set("X-Forwarded-Host", "domain1.tld")
	r := NewLinkResolver(req, "/sites/domain1.tld")
	if got := r.Base(); got != "" {
		t.Errorf("Base() with X-Forwarded-Host = %q, want empty", got)
	}
}

func TestNewLinkResolver_XSiteBaseBeatsXFH(t *testing.T) {
	// Both headers set: X-Site-Base present should still result in root-relative.
	req := newReq()
	req.Header.Set("X-Forwarded-Host", "domain1.tld")
	req.Header.Set("X-Site-Base", "")
	r := NewLinkResolver(req, "/sites/domain1.tld")
	if got := r.Base(); got != "" {
		t.Errorf("Base() with both headers = %q, want empty", got)
	}
}

func TestLinkResolver_Resolve(t *testing.T) {
	cases := []struct {
		name string
		base string
		href string
		want string
	}{
		{"empty href, prefixed base", "/sites/domain1.tld", "", "/sites/domain1.tld"},
		{"empty href, empty base", "", "", "/"},
		{"relative path, prefixed base", "/sites/domain1.tld", "components/accordion", "/sites/domain1.tld/components/accordion"},
		{"leading slash, prefixed base", "/sites/domain1.tld", "/components/accordion", "/sites/domain1.tld/components/accordion"},
		{"relative path, empty base", "", "/about", "/about"},
		{"whitespace href trimmed", "/sites/domain1.tld", "  /about  ", "/sites/domain1.tld/about"},
		{"no leading slash, empty base", "", "about", "/about"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := LinkResolver{base: tc.base}
			if got := r.Resolve(tc.href); got != tc.want {
				t.Errorf("Resolve(%q) = %q, want %q", tc.href, got, tc.want)
			}
		})
	}
}

func TestLinkResolver_SafeURL(t *testing.T) {
	r := LinkResolver{base: "/sites/domain1.tld"}
	got := r.SafeURL("/about")
	if string(got) != "/sites/domain1.tld/about" {
		t.Errorf("SafeURL(%q) = %q, want %q", "/about", got, "/sites/domain1.tld/about")
	}
	// Sanity: returns templ.SafeURL type, not a plain string.
	var _ templ.SafeURL = got
}

func TestHasSiteBase_Absent(t *testing.T) {
	if hasSiteBase(newReq()) {
		t.Error("hasSiteBase() = true for request without header, want false")
	}
}

func TestHasSiteBase_PresentEmpty(t *testing.T) {
	req := newReq()
	req.Header.Set("X-Site-Base", "")
	if !hasSiteBase(req) {
		t.Error("hasSiteBase() = false for request with empty X-Site-Base, want true")
	}
}

func TestHasXFH(t *testing.T) {
	if hasXFH(newReq()) {
		t.Error("hasXFH() = true for request without header, want false")
	}
	req := newReq()
	req.Header.Set("X-Forwarded-Host", "  ")
	if hasXFH(req) {
		t.Error("hasXFH() = true for whitespace-only header, want false")
	}
	req.Header.Set("X-Forwarded-Host", "domain1.tld")
	if !hasXFH(req) {
		t.Error("hasXFH() = false for request with header, want true")
	}
}
