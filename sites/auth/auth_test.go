package auth

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/geoffjay/jughead/sessions"
)

// stubProvider is a minimal Provider for registry tests. It never returns an
// instance; tests only exercise Name/Load wiring here, and instance behaviour
// is covered by the real provider's own tests.
type stubProvider struct{ name string }

func (s stubProvider) Name() string                              { return s.name }
func (s stubProvider) Load(AuthConfig) (ProviderInstance, error) { return nil, nil }

// stubInstance lets callers verify RegisterAuthRoutes / AuthMiddleware ran.
type stubInstance struct {
	routed  bool
	wrapped bool
}

func (s *stubInstance) RegisterAuthRoutes(_ gin.IRouter, _ *sessions.Store) { s.routed = true }
func (s *stubInstance) AuthMiddleware(_ *sessions.Store) gin.HandlerFunc {
	s.wrapped = true
	return func(c *gin.Context) {}
}

func TestRegisterAndGet(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	p := stubProvider{name: "stub"}
	Register(p)

	got, ok := Get("stub")
	if !ok {
		t.Fatal("Get(stub) returned ok=false after Register")
	}
	if got.Name() != "stub" {
		t.Errorf("Get(stub).Name() = %q, want %q", got.Name(), "stub")
	}
}

func TestGet_MissingProvider(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	if _, ok := Get("nope"); ok {
		t.Error("Get(unregistered) returned ok=true; want false")
	}
}

func TestRegister_ReplacesExisting(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	Register(stubProvider{name: "github"})
	Register(stubProvider{name: "github"}) // second registration should not panic

	got, ok := Get("github")
	if !ok {
		t.Fatal("Get(github) returned ok=false after re-registration")
	}
	if got.Name() != "github" {
		t.Errorf("Get(github).Name() = %q, want %q", got.Name(), "github")
	}
}

func TestGet_ConcurrentSafe(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	Register(stubProvider{name: "github"})

	// Hammer Get from multiple goroutines; the RWMutex should keep this safe.
	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			_, _ = Get("github")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
