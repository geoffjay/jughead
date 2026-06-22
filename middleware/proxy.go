package middleware

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"context"

	"github.com/gin-gonic/gin"
)

func ReverseProxy(domains map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("host: " + c.Request.Host)

		if c.Request.Host == "localhost:9000" {
			log.Println("continue for next")
			c.Next()
		} else {
			log.Println("continue for proxy")
		}

		// buffer the body to read it here and send it in the request
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
			return
		}

		// you can reassign the body if you need to parse it as multipart
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// create a new url from the raw RequestURI sent by the client
		url := fmt.Sprintf("%s://%s%s", "http", domains[c.Request.Host], c.Request.RequestURI)

		proxyReq, err := func() (*http.Request, error) {
			var (
				method = c.Request.Method
				body   = bytes.NewReader(body)
			)
			return http.NewRequestWithContext(context.Background(), method, url, body)
		}()

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadGateway)
			return
		}

		// We may want to filter some headers, otherwise we could just use a shallow copy
		// proxyReq.Header = c.Request.Header
		proxyReq.Header = make(http.Header)
		for h, val := range c.Request.Header {
			proxyReq.Header[h] = val
		}

		client := &http.Client{}
		resp, err := client.Do(proxyReq)

		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadGateway)
			return
		}

		defer resp.Body.Close()

		bodyContent, _ := io.ReadAll(resp.Body)
		c.Writer.Write(bodyContent)

		for h, val := range resp.Header {
			c.Writer.Header()[h] = val
		}

		c.Abort()
	}
}

// func ReverseProxyV2() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		originalHost := c.Request.Host
// 		targetPath := "/sites/" + originalHost
//
// 		targetURL, err := url.Parse("http://localhost:9000" + targetPath)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
// 			c.Abort()
// 			return
// 		}
//
// 		proxy := httputil.NewSingleHostReverseProxy(targetURL)
// 		proxy.Director = func(req *http.Request) {
// 			req.URL.Scheme = targetURL.Scheme
// 			req.URL.Host = targetURL.Host
// 			req.URL.Path = targetURL.Path
// 			req.Host = targetURL.Host
// 		}
//
// 		proxy.ServeHTTP(c.Writer, c.Request)
// 		c.Abort()
// 	}
// }
