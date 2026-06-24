package domain1

import (
	"net/http"

	"github.com/geoffjay/jughead/sites/links"
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
	router.GET("/components/accordion", func(c *gin.Context) {
		render(c, theme, "Accordion - domain1.tld", AccordionPage(resolver(c)))
	})
	router.GET("/components/navbar", func(c *gin.Context) {
		render(c, theme, "Navbar - domain1.tld", NavbarPage(resolver(c)))
	})
	router.GET("/components/join", func(c *gin.Context) {
		render(c, theme, "Join - domain1.tld", JoinPage(resolver(c)))
	})
	router.GET("/components/stack", func(c *gin.Context) {
		render(c, theme, "Stack - domain1.tld", StackPage(resolver(c)))
	})
	router.GET("/components/divider", func(c *gin.Context) {
		render(c, theme, "Divider - domain1.tld", DividerPage(resolver(c)))
	})
	router.GET("/components/mask", func(c *gin.Context) {
		render(c, theme, "Mask - domain1.tld", MaskPage(resolver(c)))
	})
	router.GET("/components/link", func(c *gin.Context) {
		render(c, theme, "Link - domain1.tld", LinkPage(resolver(c)))
	})
	router.GET("/components/kbd", func(c *gin.Context) {
		render(c, theme, "Kbd - domain1.tld", KbdPage(resolver(c)))
	})
	router.GET("/components/status", func(c *gin.Context) {
		render(c, theme, "Status - domain1.tld", StatusPage(resolver(c)))
	})
	router.GET("/components/loading", func(c *gin.Context) {
		render(c, theme, "Loading - domain1.tld", LoadingPage(resolver(c)))
	})
	router.GET("/components/collapse", func(c *gin.Context) {
		render(c, theme, "Collapse - domain1.tld", CollapsePage(resolver(c)))
	})
	router.GET("/components/swap", func(c *gin.Context) {
		render(c, theme, "Swap - domain1.tld", SwapPage(resolver(c)))
	})
	router.GET("/components/theme-controller", func(c *gin.Context) {
		render(c, theme, "Theme Controller - domain1.tld", ThemeControllerPage(resolver(c)))
	})
	router.GET("/components/button", func(c *gin.Context) {
		render(c, theme, "Button - domain1.tld", ButtonPage(resolver(c)))
	})
	router.GET("/components/floating-action-button", func(c *gin.Context) {
		render(c, theme, "FAB / Speed Dial - domain1.tld", FABPage(resolver(c)))
	})
	router.GET("/components/input-field", func(c *gin.Context) {
		render(c, theme, "Text Input - domain1.tld", InputFieldPage(resolver(c)))
	})
	router.GET("/components/textarea", func(c *gin.Context) {
		render(c, theme, "Textarea - domain1.tld", TextareaPage(resolver(c)))
	})
	router.GET("/components/select", func(c *gin.Context) {
		render(c, theme, "Select - domain1.tld", SelectPage(resolver(c)))
	})
	router.GET("/components/checkbox", func(c *gin.Context) {
		render(c, theme, "Checkbox - domain1.tld", CheckboxPage(resolver(c)))
	})
	router.GET("/components/radio", func(c *gin.Context) {
		render(c, theme, "Radio - domain1.tld", RadioPage(resolver(c)))
	})
	router.GET("/components/range", func(c *gin.Context) {
		render(c, theme, "Range slider - domain1.tld", RangePage(resolver(c)))
	})
	router.GET("/components/file-input", func(c *gin.Context) {
		render(c, theme, "File Input - domain1.tld", FileInputPage(resolver(c)))
	})
	router.GET("/components/toggle", func(c *gin.Context) {
		render(c, theme, "Toggle - domain1.tld", TogglePage(resolver(c)))
	})
	router.GET("/components/rating", func(c *gin.Context) {
		render(c, theme, "Rating - domain1.tld", RatingPage(resolver(c)))
	})
	router.GET("/components/label", func(c *gin.Context) {
		render(c, theme, "Label - domain1.tld", LabelPage(resolver(c)))
	})
	router.GET("/components/fieldset", func(c *gin.Context) {
		render(c, theme, "Fieldset - domain1.tld", FieldsetPage(resolver(c)))
	})
	router.GET("/components/validator", func(c *gin.Context) {
		render(c, theme, "Validator - domain1.tld", ValidatorPage(resolver(c)))
	})
	router.GET("/components/alert", func(c *gin.Context) {
		render(c, theme, "Alert - domain1.tld", AlertPage(resolver(c)))
	})
	router.GET("/components/badge", func(c *gin.Context) {
		render(c, theme, "Badge - domain1.tld", BadgePage(resolver(c)))
	})
	router.GET("/components/progress", func(c *gin.Context) {
		render(c, theme, "Progress - domain1.tld", ProgressPage(resolver(c)))
	})
	router.GET("/components/radial-progress", func(c *gin.Context) {
		render(c, theme, "Radial progress - domain1.tld", RadialProgressPage(resolver(c)))
	})
	router.GET("/components/countdown", func(c *gin.Context) {
		render(c, theme, "Countdown - domain1.tld", CountdownPage(resolver(c)))
	})
	router.GET("/components/skeleton", func(c *gin.Context) {
		render(c, theme, "Skeleton - domain1.tld", SkeletonPage(resolver(c)))
	})
	router.GET("/components/toast", func(c *gin.Context) {
		render(c, theme, "Toast - domain1.tld", ToastPage(resolver(c)))
	})
	router.GET("/components/tooltip", func(c *gin.Context) {
		render(c, theme, "Tooltip - domain1.tld", TooltipPage(resolver(c)))
	})
	router.GET("/components/indicator", func(c *gin.Context) {
		render(c, theme, "Indicator - domain1.tld", IndicatorPage(resolver(c)))
	})
	router.GET("/components/chat-bubble", func(c *gin.Context) {
		render(c, theme, "Chat bubble - domain1.tld", ChatBubblePage(resolver(c)))
	})
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
