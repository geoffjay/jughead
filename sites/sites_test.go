package sites

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestSite_ThemeValue(t *testing.T) {
	cases := []struct {
		name string
		site *Site
		want string
	}{
		{"nil site", nil, "light"},
		{"empty theme", &Site{Theme: ""}, "light"},
		{"set theme", &Site{Theme: "dark"}, "dark"},
		{"custom theme", &Site{Theme: "cupcake"}, "cupcake"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.site.ThemeValue(); got != tc.want {
				t.Errorf("ThemeValue() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInitialize_SkipsPluginRegisteredPath verifies that Initialize() does not
// overwrite a site already registered under the same path by a plugin. This
// matches the provider override semantics: plugins take precedence over
// built-in sites, so a plugin shipping a site replaces the in-repo site under
// the same path rather than being clobbered by it.
func TestInitialize_SkipsPluginRegisteredPath(t *testing.T) {
	resetSingleton(t)
	t.Cleanup(func() { resetSingleton(t) })

	sm := GetSiteManager()

	// Simulate a plugin registering a site under the same path as the in-repo
	// quux site. Use a distinct template/theme so we can detect which
	// registration survived.
	pluginMarker := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write([]byte("plugin-quux"))
		return err
	})
	pluginSite := &Site{
		Path:      "/sites/quux.geoffjay.com",
		Url:       "https://quux.geoffjay.com",
		Published: true,
		Theme:     "nord",
		Template:  pluginMarker,
	}
	sm.RegisterSite(pluginSite.Path, pluginSite)

	// Initialize() iterates the in-repo sites map, which also contains the
	// quux site under the same path. It must skip re-registration so the
	// plugin's site survives.
	t.Setenv("JUGHEAD_ENV", "production")
	Initialize()

	got := sm.GetSite("/sites/quux.geoffjay.com")
	if got == nil {
		t.Fatal("GetSite returned nil; expected the plugin-registered site to survive Initialize()")
	}
	// Verify the plugin's site survived by checking its theme (the plugin
	// uses "nord"; the in-repo quux site uses "kanagawa-dark").
	if got.Theme != "nord" {
		t.Errorf("site Theme = %q, want %q (plugin's theme); Initialize() overwrote the plugin site", got.Theme, "nord")
	}
	// Confirm the template is the plugin's marker, not the in-repo
	// SignInPrompt, by rendering it.
	var buf strings.Builder
	if err := got.Template.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render template: %v", err)
	}
	if buf.String() != "plugin-quux" {
		t.Errorf("rendered template = %q, want %q (plugin marker); Initialize() overwrote the plugin site", buf.String(), "plugin-quux")
	}
}

// TestInitialize_RegistersBuiltInWhenNoPlugin verifies the normal path: when
// no plugin has claimed a path, Initialize() registers the built-in site. It
// also confirms the docs site is NOT registered (it's plugin-only now; the
// in-repo entry was removed from the sites map).
func TestInitialize_RegistersBuiltInWhenNoPlugin(t *testing.T) {
	resetSingleton(t)
	t.Cleanup(func() { resetSingleton(t) })

	t.Setenv("JUGHEAD_ENV", "production")
	Initialize()

	sm := GetSiteManager()
	// quux is Published: true and should be registered in production.
	if got := sm.GetSite("/sites/quux.geoffjay.com"); got == nil {
		t.Error("GetSite(quux) returned nil; expected Initialize() to register the built-in site")
	}
	// docs is no longer in the in-repo sites map (it's a plugin-only site),
	// so Initialize() must not register it.
	if got := sm.GetSite("/sites/docs.jughead.geoffjay.com"); got != nil {
		t.Error("GetSite(docs) returned non-nil; the docs site is plugin-only and should not be registered by Initialize()")
	}
}
