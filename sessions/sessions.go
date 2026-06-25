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

	// GitHub OAuth fields. Populated by CreateWithToken when the user signs in
	// via the GitHub OAuth web flow; left zero-valued for the legacy admin
	// password login. AccessToken is the bearer token sent to api.github.com.
	AccessToken string
	Login       string // GitHub user login, e.g. "geoffjay"
	AvatarURL   string // GitHub user avatar image URL
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
	return s.create(w, &Session{Username: username})
}

// CreateWithToken makes a new session populated with GitHub OAuth fields and
// sets the cookie on the response. Used by the GitHub login flow; the legacy
// password login uses Create. Username is set to the GitHub login for parity
// with the admin path (middleware reads c.Set("username", sess.Username)).
func (s *Store) CreateWithToken(w http.ResponseWriter, login, accessToken, avatarURL string) string {
	return s.create(w, &Session{
		Username:    login,
		Login:       login,
		AccessToken: accessToken,
		AvatarURL:   avatarURL,
	})
}

// create is the shared implementation backing Create and CreateWithToken. It
// generates a fresh token, evicts any prior session for the same username,
// stores the session, and sets the cookie.
func (s *Store) create(w http.ResponseWriter, sess *Session) string {
	token := newToken()

	now := time.Now()
	s.mu.Lock()
	// Drop any pre-existing session for this user to avoid stale entries.
	for t, existing := range s.cookies {
		if existing.Username == sess.Username {
			delete(s.cookies, t)
		}
	}
	sess.CreatedAt = now
	sess.ExpiresAt = now.Add(s.maxAge)
	s.cookies[token] = sess
	s.mu.Unlock()

	s.setCookie(w, token, now.Add(s.maxAge))

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

	s.setCookie(w, "", time.Time{})
}

// setCookie writes the session cookie with the given token and expiry. An
// empty token with a zero expiry clears the cookie (MaxAge: -1).
func (s *Store) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if token == "" {
		cookie.MaxAge = -1
	} else {
		cookie.MaxAge = int(s.maxAge.Seconds())
		cookie.Expires = expires
	}
	http.SetCookie(w, cookie)
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
