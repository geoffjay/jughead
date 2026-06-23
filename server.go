package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/geoffjay/jughead/middleware"
	"github.com/geoffjay/jughead/sessions"
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

	store := sessions.NewStore()

	router.Use(middleware.ReverseProxy(map[string]string{
		"domain1.tld":            fmt.Sprintf("http://localhost:%d/sites/domain1.tld", port),
		"domain2.tld":            fmt.Sprintf("http://localhost:%d/sites/domain2.tld", port),
		"quux.geoffjay.com":      fmt.Sprintf("http://localhost:%d/sites/quux.geoffjay.com", port),
	}))

	router.HTMLRender = &TemplRender{}
	router.Static("/static", "./static")

	router.GET("/", indexViewHandler)
	router.GET("/api/hello-world", showContentAPIHandler)

	// Register each loaded site under its own path so sites can own their
	// subpages. Sites without a Routes callback get a default handler that
	// renders the site's template.
	sm := sites.GetSiteManager()
	for _, site := range sm.Sites() {
		group := router.Group(site.Path)
		if site.Routes != nil {
			site.Routes(group, site.ThemeValue())
		} else {
			group.GET("", siteViewHandler)
			group.GET("/", siteViewHandler)
		}
	}

	// Auth routes. /login is public; /admin (and anything added to its group)
	// requires a valid session and redirects to /login otherwise.
	router.GET("/login", loginViewHandler)
	router.POST("/login", loginSubmitHandler(store))
	router.GET("/logout", logoutHandler(store))

	admin := router.Group("/admin", middleware.AuthRequired(store))
	admin.GET("", adminViewHandler)
	admin.GET("/", adminViewHandler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      router,
	}

	slog.Info("Starting server...", "port", port)

	return server.ListenAndServe()
}
