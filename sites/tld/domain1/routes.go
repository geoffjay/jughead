package domain1

import (
	"net/http"

	"github.com/geoffjay/jughead/sites/links"
	"github.com/geoffjay/jughead/sites/tld/domain1/components"
	"github.com/geoffjay/jughead/templates"
	"github.com/geoffjay/jughead/templates/pages"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/gin-gonic/gin"
)

// sitePath is the configured path under which domain1 is served.
const sitePath = "/sites/domain1.tld"

// Routes registers the domain1 pages on the provided router group. The
// group's base path is the site path (e.g. "/sites/domain1.tld"). The theme is
// the site's configured daisyUI theme, passed through to templates.Layout.
//
// A links.LinkResolver is built from each request so that links render with
// the site-path prefix when accessed directly (e.g.
// http://localhost:9000/sites/domain1.tld) and root-relative when accessed via
// the FQDN reverse proxy (e.g. http://domain1.tld:9000).
func Routes(router *gin.RouterGroup, theme string) {
	router.GET("", func(c *gin.Context) { render(c, theme, "domain1.tld", Home(resolver(c))) })
	router.GET("/", func(c *gin.Context) { render(c, theme, "domain1.tld", Home(resolver(c))) })
	registerComponentRoutes(router, theme, componentRoutes())
}

// componentRoute binds a page title to the templ component factory that
// builds the page body from a per-request LinkResolver.
type componentRoute struct {
	Title string
	Page  func(links.LinkResolver) templ.Component
}

// componentRoutes returns the full set of component example pages keyed by
// their URL slug. Splitting this out of Routes keeps Routes under the linter's
// function statement budget.
func componentRoutes() map[string]componentRoute {
	return map[string]componentRoute{
		"accordion":              {"Accordion - domain1.tld", components.AccordionPage},
		"navbar":                 {"Navbar - domain1.tld", components.NavbarPage},
		"join":                   {"Join - domain1.tld", components.JoinPage},
		"stack":                  {"Stack - domain1.tld", components.StackPage},
		"divider":                {"Divider - domain1.tld", components.DividerPage},
		"mask":                   {"Mask - domain1.tld", components.MaskPage},
		"link":                   {"Link - domain1.tld", components.LinkPage},
		"kbd":                    {"Kbd - domain1.tld", components.KbdPage},
		"status":                 {"Status - domain1.tld", components.StatusPage},
		"loading":                {"Loading - domain1.tld", components.LoadingPage},
		"collapse":               {"Collapse - domain1.tld", components.CollapsePage},
		"swap":                   {"Swap - domain1.tld", components.SwapPage},
		"theme-controller":       {"Theme Controller - domain1.tld", components.ThemeControllerPage},
		"button":                 {"Button - domain1.tld", components.ButtonPage},
		"floating-action-button": {"FAB / Speed Dial - domain1.tld", components.FABPage},
		"input-field":            {"Text Input - domain1.tld", components.InputFieldPage},
		"textarea":               {"Textarea - domain1.tld", components.TextareaPage},
		"select":                 {"Select - domain1.tld", components.SelectPage},
		"checkbox":               {"Checkbox - domain1.tld", components.CheckboxPage},
		"radio":                  {"Radio - domain1.tld", components.RadioPage},
		"range":                  {"Range slider - domain1.tld", components.RangePage},
		"file-input":             {"File Input - domain1.tld", components.FileInputPage},
		"toggle":                 {"Toggle - domain1.tld", components.TogglePage},
		"rating":                 {"Rating - domain1.tld", components.RatingPage},
		"label":                  {"Label - domain1.tld", components.LabelPage},
		"fieldset":               {"Fieldset - domain1.tld", components.FieldsetPage},
		"validator":              {"Validator - domain1.tld", components.ValidatorPage},
		"alert":                  {"Alert - domain1.tld", components.AlertPage},
		"badge":                  {"Badge - domain1.tld", components.BadgePage},
		"progress":               {"Progress - domain1.tld", components.ProgressPage},
		"radial-progress":        {"Radial progress - domain1.tld", components.RadialProgressPage},
		"countdown":              {"Countdown - domain1.tld", components.CountdownPage},
		"skeleton":               {"Skeleton - domain1.tld", components.SkeletonPage},
		"toast":                  {"Toast - domain1.tld", components.ToastPage},
		"tooltip":                {"Tooltip - domain1.tld", components.TooltipPage},
		"indicator":              {"Indicator - domain1.tld", components.IndicatorPage},
		"chat-bubble":            {"Chat bubble - domain1.tld", components.ChatBubblePage},
		"breadcrumbs":            {"Breadcrumbs - domain1.tld", components.BreadcrumbsPage},
		"menu":                   {"Menu - domain1.tld", components.MenuPage},
		"pagination":             {"Pagination - domain1.tld", components.PaginationPage},
		"steps":                  {"Steps - domain1.tld", components.StepsPage},
		"tabs":                   {"Tabs - domain1.tld", components.TabsPage},
		"dock":                   {"Dock - domain1.tld", components.DockPage},
		"dropdown":               {"Dropdown - domain1.tld", components.DropdownPage},
		"footer":                 {"Footer - domain1.tld", components.FooterPage},
		"drawer-sidebar":         {"Drawer sidebar - domain1.tld", components.DrawerSidebarPage},
		"filter":                 {"Filter - domain1.tld", components.FilterPage},
	}
}

// registerComponentRoutes wires up each "/components/<slug>" route to render
// the corresponding page using the supplied theme.
func registerComponentRoutes(router *gin.RouterGroup, theme string, routes map[string]componentRoute) {
	for slug, cr := range routes {
		cr := cr
		router.GET("/components/"+slug, func(c *gin.Context) {
			render(c, theme, cr.Title, cr.Page(resolver(c)))
		})
	}
}

func resolver(c *gin.Context) links.LinkResolver {
	return links.NewLinkResolver(c.Request, sitePath)
}

func render(c *gin.Context, theme, title string, body templ.Component) {
	metaTags := pages.MetaTags(
		"domain1, daisyui, components",
		"domain1.tld daisyUI component examples.",
	)
	siteTemplate := templates.Layout(title, theme, metaTags, body)
	if err := htmx.NewResponse().RenderTempl(c.Request.Context(), c.Writer, siteTemplate); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
