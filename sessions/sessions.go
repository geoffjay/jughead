package sessions

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	cookieName    = "jughead_session"
	tokenBytes    = 32
	defaultMaxAge = 24 * time.Hour
)

// Session represents a single authenticated session.
type Session struct {
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store is an in-process session cache. No external service required.
type Store struct {
	mu      sync.Mutex
	maxAge  time.Duration
	cookies map[string]*Session
}

// NewStore creates an in-memory session store.
func NewStore() *Store {
	return &Store{
		maxAge:  defaultMaxAge,
		cookies: make(map[string]*Session),
	}
}

// Create makes a new session, returns its token and sets the cookie on the
// response. Concurrent calls with the same username replace the prior session.
func (s *Store) Create(w http.ResponseWriter, username string) string {
	token := newToken()

	now := time.Now()
	s.mu.Lock()
	// Drop any pre-existing session for this user to avoid stale entries.
	for t, sess := range s.cookies {
		if sess.Username == username {
			delete(s.cookies, t)
		}
	}
	s.cookies[token] = &Session{
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(s.maxAge),
	}
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.maxAge.Seconds()),
		Expires:  now.Add(s.maxAge),
	})

	return token
}

// Get returns the session for the request's cookie, or nil if none / expired.
func (s *Store) Get(r *http.Request) *Session {
	c, err := r.Cookie(cookieName)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.cookies[c.Value]
	if !ok {
		return nil
	}
	if time.Now().After(sess.ExpiresAt) {
		delete(s.cookies, c.Value)
		return nil
	}
	return sess
}

// Destroy removes the current session and clears the cookie.
func (s *Store) Destroy(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(cookieName)
	if err == nil {
		s.mu.Lock()
		delete(s.cookies, c.Value)
		s.mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// CookieName exposes the cookie name used by the store.
func CookieName() string { return cookieName }

func newToken() string {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		// rand.Read should never fail on supported platforms; panic to surface
		// the issue clearly rather than hand out a predictable token.
		panic("sessions: failed to read random bytes: " + err.Error())
	}
	return hex.EncodeToString(b)
}