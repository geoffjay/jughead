package domain1

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

func Proxy(ctx *gin.Context) {
	fmt.Println("Proxy for domain1.tld")

	remote, err := url.Parse("http://domain1.tld:9000")
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)

	proxy.Director = func(req *http.Request) {
		if ctx.Request.Host == "domain1.tld:9000" {
			req.Header = ctx.Request.Header
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = ctx.Param("proxyPath")

			fmt.Println("Proxying to", remote)
			fmt.Println(" > Path", ctx.Param("proxyPath"))
			fmt.Println(" > Host", req.URL.Host)
			fmt.Println(" > URL", req.URL.Path)
		}
	}

	proxy.ServeHTTP(ctx.Writer, ctx.Request)
}
