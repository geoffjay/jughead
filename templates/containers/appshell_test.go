package containers

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/geoffjay/jughead/templates/components/daisyui"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func mustContain(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("expected output to contain %q, got:\n%s", want, got)
	}
}

func mustNotContain(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("expected output to NOT contain %q, got:\n%s", want, got)
	}
}

func TestAppShellDefaults(t *testing.T) {
	got := render(t, AppShell(AppShellConfig{
		Title: "Jughead",
		MenuItems: []daisyui.MenuItem{
			{Label: "Dashboard", Href: "/"},
		},
		Content: templ.Raw("<p>main</p>"),
	}))
	mustContain(t, got, `data-appshell="root"`)
	mustContain(t, got, `class="h-screen w-full flex flex-col overflow-hidden"`)
	mustContain(t, got, `data-appshell="sidebar"`)
	mustContain(t, got, `data-state="open"`)
	mustContain(t, got, `data-appshell="toggle"`)
	mustContain(t, got, `data-appshell="collapse"`)
	mustContain(t, got, `data-appshell="collapse-icon-open"`)
	mustContain(t, got, `data-appshell="collapse-icon-collapsed"`)
	mustContain(t, got, ">Jughead<")
	mustContain(t, got, "Dashboard")
	mustContain(t, got, "<p>main</p>")
	mustContain(t, got, `class="flex-1 min-w-0 overflow-y-auto"`)
	mustContain(t, got, `appshell-sidebar-state`)
}

func TestAppShellSidebarStateCollapsed(t *testing.T) {
	got := render(t, AppShell(AppShellConfig{
		Title:        "Jughead",
		SidebarState: SidebarCollapsed,
		Content:      templ.Raw(""),
	}))
	mustContain(t, got, `data-state="collapsed"`)
}

func TestAppShellSidebarStateClosed(t *testing.T) {
	got := render(t, AppShell(AppShellConfig{
		Title:        "Jughead",
		SidebarState: SidebarClosed,
		Content:      templ.Raw(""),
	}))
	mustContain(t, got, `data-state="closed"`)
}

func TestAppShellIconsRendered(t *testing.T) {
	got := render(t, AppShell(AppShellConfig{Title: "x"}))
	mustContain(t, got, `<svg`)
	mustContain(t, got, `viewBox="0 0 256 256"`)
}

func TestAppShellHeaderExtra(t *testing.T) {
	got := render(t, AppShell(AppShellConfig{
		Title:       "x",
		HeaderExtra: templ.Raw(`<button data-x="theme">theme</button>`),
	}))
	mustContain(t, got, `data-x="theme"`)
}

func TestAppShellNilContent(t *testing.T) {
	got := render(t, AppShell(AppShellConfig{Title: "x"}))
	mustContain(t, got, `class="flex-1 min-w-0 overflow-y-auto"`)
	mustNotContain(t, got, "nil")
}

func TestSidebarStateString(t *testing.T) {
	if string(SidebarOpen) != "open" {
		t.Errorf("SidebarOpen = %q, want open", SidebarOpen)
	}
	if string(SidebarCollapsed) != "collapsed" {
		t.Errorf("SidebarCollapsed = %q, want collapsed", SidebarCollapsed)
	}
	if string(SidebarClosed) != "closed" {
		t.Errorf("SidebarClosed = %q, want closed", SidebarClosed)
	}
}
