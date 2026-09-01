package chat

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Proxy is a reverse-proxy fallback for the chat site. It mirrors the quux
// proxy pattern but targets chat.geoffjay.com. It is unused in the demo (the
// site is served via Routes directly) but satisfies the Site.Proxy field.
func Proxy(c *gin.Context) {
	log.Println("Proxying to chat.geoffjay.com")

	remote, err := url.Parse("https://chat.geoffjay.com")
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
