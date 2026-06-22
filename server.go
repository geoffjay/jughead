package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/geoffjay/jughead/middleware"
	"github.com/geoffjay/jughead/sites"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	gowebly "github.com/gowebly/helpers"
)

// TemplRender implements the render.Render interface.
type TemplRender struct {
	Code int
	Data templ.Component
}

// Render implements the render.Render interface.
func (t TemplRender) Render(w http.ResponseWriter) error {
	t.WriteContentType(w)
	w.WriteHeader(t.Code)
	if t.Data != nil {
		return t.Data.Render(context.Background(), w)
	}
	return nil
}

// WriteContentType implements the render.Render interface.
func (t TemplRender) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

// Instance implements the render.Render interface.
func (t *TemplRender) Instance(name string, data any) render.Render {
	if templData, ok := data.(templ.Component); ok {
		return &TemplRender{
			Code: http.StatusOK,
			Data: templData,
		}
	}
	return nil
}

// runServer runs a new HTTP server with the loaded environment variables.
func runServer() error {
	sites.Initialize()

	port, err := strconv.Atoi(gowebly.Getenv("BACKEND_PORT", "9000"))

	if err != nil {
		return err
	}

	router := gin.Default()

	router.Use(middleware.ReverseProxy(map[string]string{
		"domain1.tld:9000": "http://localhost:9000/sites/domain1.tld",
		"domain2.tld:9000": "http://localhost:9000/sites/domain2.tld",
	}))

	router.HTMLRender = &TemplRender{}
	router.Static("/static", "./static")

	router.GET("/", indexViewHandler)
	router.GET("/sites/:site", siteViewHandler)
	router.GET("/api/hello-world", showContentAPIHandler)

	// siteManager := sites.GetSiteManager()
	// for _, site := range siteManager.Sites() {
	// 	fmt.Printf("Registering site: %s\n", site.Url)
	// 	router.Any("/*proxyPath", site.Proxy)
	// }

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      router,
	}

	slog.Info("Starting server...", "port", port)

	return server.ListenAndServe()
}
