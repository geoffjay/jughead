package links

import (
	"net/http"
	"strings"

	"github.com/a-h/templ"
)

// LinkResolver builds site-relative URLs that work both when the site is
// accessed under its /sites/<domain> path (e.g. browsing
// http://localhost:9000/sites/domain1.tld) and when it is accessed via its
// FQDN through the reverse proxy (e.g. http://domain1.tld:9000), where the
// browser sees root-relative paths.
//
// The resolver inspects the X-Site-Base request header. The proxy sets this
// header to an empty value when it strips the site prefix, signaling that
// links should be root-relative. When the header is absent the site is being
// accessed directly under its site path, so links are prefixed with that path.
type LinkResolver struct {
	base string
}

// NewLinkResolver builds a resolver for the given site path using the request
// to detect which access mode is active.
//
// sitePath is the configured path of the site, e.g. "/sites/domain1.tld".
// When the X-Site-Base header is present (set by the reverse proxy) the
// browser's base URL is root-relative, so resolved links drop the site path.
// When the header is absent the browser is hitting the site path directly, so
// links keep the prefix.
func NewLinkResolver(r *http.Request, sitePath string) LinkResolver {
	if hasSiteBase(r) || hasXFH(r) {
		return LinkResolver{base: ""}
	}
	return LinkResolver{base: strings.TrimRight(sitePath, "/")}
}

// hasSiteBase reports whether the X-Site-Base header is present. The proxy
// sets it to an empty string, so Header.Get (which returns "" for both absent
// and empty values) can't distinguish the two — use Header.Values instead,
// which returns nil only when the header is absent.
func hasSiteBase(r *http.Request) bool {
	return len(r.Header.Values("X-Site-Base")) > 0
}

// NewDirectResolver builds a resolver that always prefixes links with the
// given site path, regardless of request headers. Use it for contexts where
// no request is available (e.g. a pre-rendered template stored on a Site).
func NewDirectResolver(sitePath string) LinkResolver {
	return LinkResolver{base: strings.TrimRight(sitePath, "/")}
}

// hasXFH reports whether the request arrived via a forwarded host. The proxy
// deletes X-Forwarded-Host on inner requests to prevent re-entry, but external
// proxies (e.g. Fly's edge) may set it on the original request before our
// middleware runs. In that case the browser is talking to the FQDN and links
// must be root-relative.
func hasXFH(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("X-Forwarded-Host")) != ""
}

// Base returns the configured base path prefix ("" when root-relative).
func (l LinkResolver) Base() string {
	return l.base
}

// Resolve joins the base path with the given site-relative path. href should
// not include a leading site prefix — pass "/components/accordion", not
// "/sites/domain1.tld/components/accordion". An empty href resolves to the
// site root.
func (l LinkResolver) Resolve(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		if l.base == "" {
			return "/"
		}
		return l.base
	}
	if !strings.HasPrefix(href, "/") {
		href = "/" + href
	}
	return l.base + href
}

// SafeURL returns the resolved path as a templ.SafeURL for use in href
// attributes in templ templates.
func (l LinkResolver) SafeURL(href string) templ.SafeURL {
	return templ.SafeURL(l.Resolve(href))
}
