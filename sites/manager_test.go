package sites

import (
	"sync"
	"testing"
)

// resetSingleton clears the package-level singleton so each test starts clean.
func resetSingleton(t *testing.T) {
	t.Helper()
	lock.Lock()
	instance = nil
	lock.Unlock()
}

func TestGetSiteManager_Singleton(t *testing.T) {
	resetSingleton(t)
	a := GetSiteManager()
	b := GetSiteManager()
	if a == nil || b == nil {
		t.Fatal("GetSiteManager() returned nil")
	}
	if a != b {
		t.Error("GetSiteManager() should return the same instance (singleton)")
	}
}

func TestGetSiteManager_ConcurrentSafe(t *testing.T) {
	resetSingleton(t)
	var wg sync.WaitGroup
	instances := make([]*SiteManager, 20)
	for i := range instances {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			instances[i] = GetSiteManager()
		}(i)
	}
	wg.Wait()
	first := instances[0]
	if first == nil {
		t.Fatal("concurrent GetSiteManager() returned nil")
	}
	for i, sm := range instances[1:] {
		if sm != first {
			t.Errorf("instance %d differs from first; singleton not safe under concurrency", i+1)
		}
	}
}

func TestSiteManager_RegisterAndGetSite(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	s := &Site{Path: "/sites/example", Url: "http://example"}
	sm.RegisterSite("/sites/example", s)

	got := sm.GetSite("/sites/example")
	if got != s {
		t.Error("GetSite() did not return the registered site")
	}
	if got.Path != "/sites/example" {
		t.Errorf("GetSite().Path = %q, want %q", got.Path, "/sites/example")
	}
}

func TestSiteManager_GetSite_Missing(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	if got := sm.GetSite("/does/not/exist"); got != nil {
		t.Errorf("GetSite() for missing path = %v, want nil", got)
	}
}

func TestSiteManager_Sites_MapContents(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	sm.RegisterSite("/sites/a", &Site{Path: "/sites/a"})
	sm.RegisterSite("/sites/b", &Site{Path: "/sites/b"})
	sites := sm.Sites()
	if len(sites) != 2 {
		t.Errorf("Sites() has %d entries, want 2", len(sites))
	}
	if _, ok := sites["/sites/a"]; !ok {
		t.Error("Sites() missing /sites/a")
	}
	if _, ok := sites["/sites/b"]; !ok {
		t.Error("Sites() missing /sites/b")
	}
}

func TestSiteManager_Sites_EmptyByDefault(t *testing.T) {
	resetSingleton(t)
	sm := GetSiteManager()
	if got := sm.Sites(); len(got) != 0 {
		t.Errorf("fresh manager Sites() len = %d, want 0", len(got))
	}
}
