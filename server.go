package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/geoffjay/jughead/middleware"
	githubsvc "github.com/geoffjay/jughead/services/github"
	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites"
	"github.com/geoffjay/jughead/sites/auth"

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

const (
	serverReadTimeout  = 5 * time.Second
	serverWriteTimeout = 10 * time.Second
)

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
func (t *TemplRender) Instance(_ string, value any) render.Render {
	if templData, ok := value.(templ.Component); ok {
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

	// Register the GitHub OAuth provider so sites whose Auth.Provider ==
	// "github" can be wired by the SiteManager. Additional providers are
	// registered here as they're added.
	auth.Register(githubsvc.NewProvider())

	port, err := strconv.Atoi(gowebly.Getenv("BACKEND_PORT", "9000"))
	if err != nil {
		return err
	}

	router := gin.Default()
	store := sessions.NewStore()
	sm := sites.GetSiteManager()

	// Reverse-proxy map derived from each loaded site's Url/Path so requests
	// to a site's FQDN are forwarded to its localhost site-path upstream.
	router.Use(middleware.ReverseProxy(sm.BuildProxyTargets(port)))

	router.HTMLRender = &TemplRender{}
	router.Static("/static", "./static")

	router.GET("/", indexViewHandler)
	router.GET("/api/hello-world", showContentAPIHandler)

	// Wire every loaded site's routes + any requested OAuth provider routes,
	// driven by each site's config (sites.Site.Routes / .Auth). Replaces the
	// prior per-site special-casing in this file.
	sm.BuildSiteRoutes(router, store, siteViewHandler)

	// Admin password-auth routes. /login is public; /admin (and anything added
	// to its group) requires a valid session and redirects to /login otherwise.
	router.GET("/login", loginViewHandler)
	router.POST("/login", loginSubmitHandler(store))
	router.GET("/logout", logoutHandler(store))

	registerAdminRoutes(router, store)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		Handler:      router,
	}

	slog.Info("Starting server...", "port", port)

	return server.ListenAndServe()
}

func registerAdminRoutes(router *gin.Engine, store *sessions.Store) {
	admin := router.Group("/admin", middleware.AuthRequired(store))
	admin.GET("", adminViewHandler)
	admin.GET("/", adminViewHandler)
	admin.GET("/sites", sitesViewHandler)
	admin.GET("/users", usersViewHandler)
	admin.GET("/settings", settingsViewHandler)
	admin.GET("/logs", logsViewHandler)
}
