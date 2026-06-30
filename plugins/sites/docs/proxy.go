package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Proxy is the sites.Site.Proxy fallback for the docs site. It reverse-proxies
// requests to https://docs.jughead.geoffjay.com.
func Proxy(c *gin.Context) {
	log.Println("Proxying to docs.jughead.geoffjay.com")

	remote, err := url.Parse("https://docs.jughead.geoffjay.com")
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = c.Param("proxyPath")
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
