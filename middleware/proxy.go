package middleware

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// ReverseProxy routes incoming requests to per-site upstream targets based on
// the request's effective host. Hosts not present in `targets` fall through to
// the next handler (so localhost:9000 reaches the app's own routes).
//
// The effective host is taken from, in order of preference:
//  1. the `X-Forwarded-Host` header — lets you test in a browser via a
//     header-modifying extension (e.g. "Modify Header Value") without editing
//     /etc/hosts;
//  2. the request's `Host` field (what a browser sends after DNS resolution).
//
// `targets` maps host (with port) -> full upstream URL, e.g.
//
//	"domain1.tld:9000": "http://localhost:9000/sites/domain1.tld"
//
// The reverse proxy preserves the incoming request path, so
// `http://domain1.tld:9000/foo` is forwarded to
// `http://localhost:9000/sites/domain1.tld/foo`.
func ReverseProxy(targets map[string]string) gin.HandlerFunc {
	parsed := make(map[string]*url.URL, len(targets))
	for host, target := range targets {
		u, err := url.Parse(target)
		if err != nil {
			panic("middleware.ReverseProxy: invalid target for " + host + ": " + err.Error())
		}
		parsed[host] = u
	}

	return func(c *gin.Context) {
		host := effectiveHost(c.Request)
		log.Println("host:", host)

		target, ok := parsed[host]
		if !ok {
			c.Next()
			return
		}

		// Shared assets (styles.css, scripts.js, manifest.json, favicons, etc.)
		// are served by the core app at /static and are common to every site.
		// Pass them straight through to the app root instead of prepending the
		// site path, otherwise the browser's request for /static/styles.css on
		// a proxied host becomes /sites/<domain>/static/styles.css and 404s.
		if strings.HasPrefix(c.Request.URL.Path, "/static/") || c.Request.URL.Path == "/static" {
			c.Next()
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		original := proxy.Director
		proxy.Director = func(req *http.Request) {
			original(req)
			req.Host = target.Host
			// NewSingleHostReverseProxy concatenates target.Path with the
			// incoming path, producing "/sites/domain1.tld/" (trailing slash)
			// when the client hit "/". Normalise so the inner router sees the
			// exact site path and doesn't issue a 301 redirect.
			req.URL.Path = normalisePath(target.Path, req.URL.Path)
			// Prevent re-entry: the inner request hits this same middleware on
			// localhost:9000, so it must not look like a forwarded-host request
			// again, otherwise we loop forever.
			req.Header.Del("X-Forwarded-Host")
			// Tell downstream handlers that the browser's base URL has no
			// site-path prefix, so links should be rendered root-relative
			// (e.g. "/components/accordion") instead of prefixed with the site
			// path (e.g. "/sites/domain1.tld/components/accordion").
			req.Header.Set("X-Site-Base", "")
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy: %s -> %s: %v", host, target.String(), err)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

// effectiveHost returns the host (without port) that should drive routing.
// X-Forwarded-Host (if present) takes precedence over the request's Host field
// so the proxy can be exercised without real DNS entries for the target
// domains. The port is stripped because Fly's edge terminates TLS on 443 and
// forwards to the app on an arbitrary internal port; the browser therefore
// sends "quux.geoffjay.com" (no port), never "quux.geoffjay.com:8080".
func effectiveHost(r *http.Request) string {
	host := r.Host
	if xfh := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); xfh != "" {
		host = xfh
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// normalisePath collapses the join of target.Path and the request path so the
// inner router receives a clean site path. NewSingleHostReverseProxy sets
// req.URL.Path = target.Path + originalPath, so for target.Path
// "/sites/domain1.tld" and a request for "/" we get "/sites/domain1.tld/" —
// strip the duplicate slash so gin doesn't respond with a 301.
func normalisePath(targetPath, joined string) string {
	if !strings.HasPrefix(joined, targetPath) {
		return joined
	}
	rest := strings.TrimPrefix(joined, targetPath)
	// rest is the part after the site path; "" or "/" means the site root.
	if rest == "" || rest == "/" {
		return targetPath
	}
	return targetPath + rest
}