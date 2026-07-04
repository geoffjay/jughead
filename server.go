package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/geoffjay/jughead/db"
	"github.com/geoffjay/jughead/middleware"
	"github.com/geoffjay/jughead/plugin"
	"github.com/geoffjay/jughead/services"
	githubsvc "github.com/geoffjay/jughead/services/github"
	"github.com/geoffjay/jughead/sessions"
	"github.com/geoffjay/jughead/sites"
	"github.com/geoffjay/jughead/sites/auth"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	gowebly "github.com/gowebly/helpers"
	"github.com/jackc/pgx/v5/pgxpool"
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
	// Load runtime plugins first. Plugin providers register into the auth
	// registry and plugin sites register with the SiteManager, both BEFORE
	// sites.Initialize() and the built-in provider registration below, so a
	// plugin can intentionally override a built-in provider with the same
	// Name() (auth.Register replaces on conflict). A plugin load failure is
	// fatal when JUGHEAD_PLUGINS_STRICT is set; otherwise bad .so files are
	// logged and skipped.
	pluginsDir := gowebly.Getenv("JUGHEAD_PLUGINS_DIR", "")
	strict := gowebly.Getenv("JUGHEAD_PLUGINS_STRICT", "") == "1" ||
		gowebly.Getenv("JUGHEAD_PLUGINS_STRICT", "") == "true"
	if err := plugin.LoadAll(pluginsDir, strict); err != nil {
		return fmt.Errorf("load plugins: %w", err)
	}

	sites.Initialize()

	// Register the GitHub OAuth provider as a built-in fallback so the default
	// deployment works with zero plugins. A plugin shipping the same provider
	// name (registered above) already replaced this entry; re-registering the
	// built-in here would clobber the plugin, so we skip it when a plugin
	// already registered "github".
	if _, ok := auth.Get("github"); !ok {
		auth.Register(githubsvc.NewProvider())
	}

	port, err := strconv.Atoi(gowebly.Getenv("BACKEND_PORT", "9000"))
	if err != nil {
		return err
	}

	// Database pool. Optional: when DATABASE_URL is unset the server runs
	// without persistence (useful for tests and the legacy static-admin path).
	// When set, the pool connects as the non-superuser jughead_app role so RLS
	// policies apply at runtime.
	var pool *pgxpool.Pool
	if cfg, err := db.NewConfig(); err == nil {
		pool, err = db.NewPool(context.Background(), cfg)
		if err != nil {
			return fmt.Errorf("init database: %w", err)
		}
		defer pool.Close()
	} else {
		slog.Info("DATABASE_URL unset — running without a database pool")
	}

	router := gin.Default()
	store := sessions.NewStore()
	sm := sites.GetSiteManager()

	// Construct the service layer. Each service holds the concrete db
	// repositories and the shared pool. When the pool is nil (DB-disabled
	// deployments) the services are not constructed and the legacy static-
	// admin auth path is used instead.
	var (
		userSvc   *services.UserService
		orgSvc    *services.OrganizationService
		memSvc    *services.MembershipService
		inviteSvc *services.InvitationService
	)
	if pool != nil {
		userSvc = services.NewUserService(pool)
		orgSvc = services.NewOrganizationService(pool)
		memSvc = services.NewMembershipService()
		inviteSvc = services.NewInvitationService()
	}

	// Attach the DB pool to every request context so handlers can call
	// db.WithOrgContext without holding a direct pool reference. No-op when
	// the pool is nil (DB-disabled deployments).
	if pool != nil {
		router.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(db.WithPool(c.Request.Context(), pool))
			c.Next()
		})
	}

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
	// When a DB pool is available, login is backed by UserService.Authenticate;
	// otherwise the legacy static-credential fallback is used.
	router.GET("/login", loginViewHandler)
	router.POST("/login", loginSubmitHandler(store, userSvc))
	router.GET("/logout", logoutHandler(store))

	registerAdminRoutes(router, store, userSvc, orgSvc, memSvc, inviteSvc)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  serverReadTimeout,
		WriteTimeout: serverWriteTimeout,
		Handler:      router,
	}

	slog.Info("Starting server...", "port", port)

	return server.ListenAndServe()
}

// AdminDeps bundles the services the admin handlers need. When the DB pool is
// nil (DB-disabled deployments), all fields are nil and the admin handlers
// render empty placeholder states instead of querying the database.
type AdminDeps struct {
	Users   *services.UserService
	Orgs    *services.OrganizationService
	Mems    *services.MembershipService
	Invites *services.InvitationService
}

func registerAdminRoutes(
	router *gin.Engine,
	store *sessions.Store,
	userSvc *services.UserService,
	orgSvc *services.OrganizationService,
	memSvc *services.MembershipService,
	inviteSvc *services.InvitationService,
) {
	deps := &AdminDeps{
		Users:   userSvc,
		Orgs:    orgSvc,
		Mems:    memSvc,
		Invites: inviteSvc,
	}

	admin := router.Group("/admin", middleware.AuthRequired(store))
	admin.GET("", adminViewHandler(deps))
	admin.GET("/", adminViewHandler(deps))
	admin.GET("/sites", sitesViewHandler)
	admin.GET("/users", usersViewHandler(deps))
	admin.GET("/settings", settingsViewHandler)
	admin.GET("/logs", logsViewHandler)
}
