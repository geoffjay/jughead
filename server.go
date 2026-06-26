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
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux"
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/config"
	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/data"

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

	port, err := strconv.Atoi(gowebly.Getenv("BACKEND_PORT", "9000"))
	if err != nil {
		return err
	}

	router := gin.Default()

	store := sessions.NewStore()

	registerSiteRoutes(router, port)

	router.HTMLRender = &TemplRender{}
	router.Static("/static", "./static")

	router.GET("/", indexViewHandler)
	router.GET("/api/hello-world", showContentAPIHandler)

	sm := sites.GetSiteManager()

	// GitHub OAuth config. When unset (dev without a token) the quux site
	// falls back to a friendly "configure GITHUB_CLIENT_ID" error page; the
	// auth flow and service simply aren't registered.
	oauthCfg, cfgErr := config.Load()

	// Register the OAuth flow routes at the app root so they're reachable via
	// both localhost (/sites/...) and the FQDN reverse proxy (/). Only register
	// when the config is complete.
	if cfgErr == nil {
		registerAuthRoutes(router, store, oauthCfg)
	} else {
		slog.Warn("GitHub OAuth config incomplete; quux auth routes disabled",
			"hint", cfgErr.Error())
	}

	// Build the quux Service (auth client + cache) and register its routes
	// behind the GitHub auth middleware.
	quuxSite := sm.GetSite("/sites/quux.geoffjay.com")
	if quuxSite != nil {
		quuxGroup := router.Group(quuxSite.Path)
		if cfgErr == nil {
			quuxGroup.Use(middleware.GitHubAuthRequired(store))
			svc := quux.NewService(data.NewCache())
			svc.RegisterRoutes(quuxGroup, quuxSite.ThemeValue())
		} else {
			// No auth configured: render a placeholder so the site isn't a
			// bare 404. The default siteViewHandler still renders the (now
			// fixture-less) template.
			quuxGroup.GET("", siteViewHandler)
			quuxGroup.GET("/", siteViewHandler)
		}
	}

	// Register each remaining loaded site under its own path.
	for _, site := range sm.Sites() {
		if site.Path == "/sites/quux.geoffjay.com" {
			continue // handled above
		}
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

func registerSiteRoutes(router *gin.Engine, port int) {
	router.Use(middleware.ReverseProxy(map[string]string{
		"domain1.tld":       fmt.Sprintf("http://localhost:%d/sites/domain1.tld", port),
		"domain2.tld":       fmt.Sprintf("http://localhost:%d/sites/domain2.tld", port),
		"quux.geoffjay.com": fmt.Sprintf("http://localhost:%d/sites/quux.geoffjay.com", port),
	}))
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
