package sessions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCookieName(t *testing.T) {
	if got := CookieName(); got != "jughead_session" {
		t.Errorf("CookieName() = %q, want %q", got, "jughead_session")
	}
}

func TestNewStore_Empty(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore() returned nil")
	}
}

func TestStore_Create_SetsCookieAndAllowsGet(t *testing.T) {
	s := NewStore()
	w := httptest.NewRecorder()
	token := s.Create(w, "alice")

	if token == "" {
		t.Fatal("Create() returned empty token")
	}

	resp := w.Result()
	defer resp.Body.Close()
	found := false
	for _, c := range resp.Cookies() {
		if c.Name == cookieName && c.Value == token {
			found = true
			if !c.HttpOnly {
				t.Error("session cookie is not HttpOnly")
			}
			if c.Path != "/" {
				t.Errorf("cookie Path = %q, want %q", c.Path, "/")
			}
			if c.MaxAge <= 0 {
				t.Errorf("cookie MaxAge = %d, want positive", c.MaxAge)
			}
		}
	}
	if !found {
		t.Error("Create() did not set the session cookie with the returned token")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	sess := s.Get(req)
	if sess == nil {
		t.Fatal("Get() returned nil after Create()")
	}
	if sess.Username != "alice" {
		t.Errorf("Get().Username = %q, want %q", sess.Username, "alice")
	}
	if sess.ExpiresAt.IsZero() {
		t.Error("Get().ExpiresAt is zero, want set")
	}
}

func TestStore_Get_NoCookie(t *testing.T) {
	s := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := s.Get(req); got != nil {
		t.Errorf("Get() without cookie = %v, want nil", got)
	}
}

func TestStore_Get_EmptyCookieValue(t *testing.T) {
	s := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "   "})
	if got := s.Get(req); got != nil {
		t.Errorf("Get() with whitespace cookie = %v, want nil", got)
	}
}

func TestStore_Get_UnknownToken(t *testing.T) {
	s := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "deadbeef"})
	if got := s.Get(req); got != nil {
		t.Errorf("Get() with unknown token = %v, want nil", got)
	}
}

func TestStore_Create_ReplacesPriorSessionForSameUser(t *testing.T) {
	s := NewStore()
	w1 := httptest.NewRecorder()
	t1 := s.Create(w1, "bob")
	w2 := httptest.NewRecorder()
	t2 := s.Create(w2, "bob")

	if t1 == t2 {
		t.Fatal("Create() returned the same token twice; expected a new token")
	}

	// Old token should no longer resolve.
	reqOld := httptest.NewRequest(http.MethodGet, "/", nil)
	reqOld.AddCookie(&http.Cookie{Name: cookieName, Value: t1})
	if got := s.Get(reqOld); got != nil {
		t.Errorf("Get() with old token = %v, want nil (replaced)", got)
	}

	// New token works.
	reqNew := httptest.NewRequest(http.MethodGet, "/", nil)
	reqNew.AddCookie(&http.Cookie{Name: cookieName, Value: t2})
	if got := s.Get(reqNew); got == nil || got.Username != "bob" {
		t.Errorf("Get() with new token = %v, want session for bob", got)
	}
}

func TestStore_Destroy_RemovesSessionAndClearsCookie(t *testing.T) {
	s := NewStore()
	w := httptest.NewRecorder()
	token := s.Create(w, "carol")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})

	w2 := httptest.NewRecorder()
	s.Destroy(w2, req)

	resp := w2.Result()
	defer resp.Body.Close()
	cleared := false
	for _, c := range resp.Cookies() {
		if c.Name == cookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("Destroy() did not set an expired cookie to clear it")
	}

	// Session should no longer resolve.
	if got := s.Get(req); got != nil {
		t.Errorf("Get() after Destroy() = %v, want nil", got)
	}
}

func TestStore_Destroy_NoCookieIsNoop(t *testing.T) {
	s := NewStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Destroy(w, req) // should not panic
}

func TestStore_Get_ExpiredSessionIsEvicted(t *testing.T) {
	s := NewStore()
	// Inject an already-expired session directly.
	const tok = "expired-token"
	s.mu.Lock()
	s.cookies[tok] = &Session{
		Username:  "dave",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: tok})
	if got := s.Get(req); got != nil {
		t.Errorf("Get() with expired session = %v, want nil", got)
	}
	// And the entry should have been removed.
	s.mu.Lock()
	_, stillThere := s.cookies[tok]
	s.mu.Unlock()
	if stillThere {
		t.Error("expired session was not evicted from the store")
	}
}

func TestNewToken_UniqueAndHexEncoded(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		tok := newToken()
		if _, dup := seen[tok]; dup {
			t.Fatalf("newToken() produced a duplicate: %s", tok)
		}
		seen[tok] = struct{}{}
		// 32 bytes -> 64 hex chars.
		if len(tok) != tokenBytes*2 {
			t.Errorf("newToken() length = %d, want %d (hex of %d bytes)", len(tok), tokenBytes*2, tokenBytes)
		}
		if !strings.HasPrefix(tok, "") || strings.ContainsAny(tok, "ghijklmnopqrstuvwxyz") {
			t.Errorf("newToken() = %q, want lowercase hex only", tok)
		}
	}
}
