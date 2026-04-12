package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Session holds a single user's Google credentials.
// The MCP server looks this up on every request via the session ID
// that Claude Code sends as a Bearer token.
const sessionTTL = 24 * time.Hour

type Session struct {
	Email        string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

// PendingAuth tracks state between /authorize and /callback.
type PendingAuth struct {
	ClientID          string
	CodeChallenge     string
	ClientRedirectURI string
	State             string
	CreatedAt         time.Time
}

// AuthCode maps our internal auth code to the Google tokens
// we received. Claude Code exchanges this code for a session.
type AuthCode struct {
	ClientID      string
	SessionID     string
	CodeChallenge string
	CreatedAt     time.Time
}

// Client represents a registered OAuth client (via Dynamic Client Registration).
type Client struct {
	ClientID                string
	ClientSecret            string   // empty for public clients (token_endpoint_auth_method=none)
	ClientName              string
	RedirectURIs            []string
	GrantTypes              []string
	TokenEndpointAuthMethod string   // "none" or "client_secret_post"
	CreatedAt               time.Time
}

// Store is an in-memory session store.
// Swap this for Redis if you need persistence across restarts.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session

	pendingMu sync.RWMutex
	pending   map[string]*PendingAuth

	codesMu sync.RWMutex
	codes   map[string]*AuthCode

	clientsMu sync.RWMutex
	clients   map[string]*Client
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
		pending:  make(map[string]*PendingAuth),
		codes:    make(map[string]*AuthCode),
		clients:  make(map[string]*Client),
	}
}

// --- Sessions ---

func (s *Store) CreateSession(sess Session) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess.CreatedAt = time.Now()
	id := randomHex(32)
	s.sessions[id] = &sess
	return id
}

func (s *Store) GetSession(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	if time.Since(sess.CreatedAt) > sessionTTL {
		delete(s.sessions, id)
		return nil, fmt.Errorf("session expired")
	}
	return sess, nil
}

func (s *Store) UpdateSession(id string, sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = sess
}

// --- Pending auth (between /authorize and Google callback) ---

func (s *Store) StorePending(key string, p PendingAuth) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	p.CreatedAt = time.Now()
	s.pending[key] = &p
}

func (s *Store) GetPending(key string) (*PendingAuth, error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	p, ok := s.pending[key]
	if !ok {
		return nil, fmt.Errorf("pending auth not found")
	}
	delete(s.pending, key) // one-time use
	if time.Since(p.CreatedAt) > 5*time.Minute {
		return nil, fmt.Errorf("pending auth expired")
	}
	return p, nil
}

// --- Auth codes (between Google callback and Claude Code /token) ---

func (s *Store) StoreAuthCode(code string, ac AuthCode) {
	s.codesMu.Lock()
	defer s.codesMu.Unlock()
	ac.CreatedAt = time.Now()
	s.codes[code] = &ac
}

func (s *Store) GetAuthCode(code string) (*AuthCode, error) {
	s.codesMu.Lock()
	defer s.codesMu.Unlock()

	ac, ok := s.codes[code]
	if !ok {
		return nil, fmt.Errorf("auth code not found")
	}
	delete(s.codes, code) // one-time use
	if time.Since(ac.CreatedAt) > 5*time.Minute {
		return nil, fmt.Errorf("auth code expired")
	}
	return ac, nil
}

// --- Registered OAuth clients ---

func (s *Store) RegisterClient(c Client) string {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	c.ClientID = randomHex(16)
	c.CreatedAt = time.Now()
	s.clients[c.ClientID] = &c
	return c.ClientID
}

func (s *Store) GetClient(clientID string) (*Client, error) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	c, ok := s.clients[clientID]
	if !ok {
		return nil, fmt.Errorf("client not found")
	}
	return c, nil
}

// StartCleanup runs a background goroutine that prunes expired entries every 10 minutes.
// The returned function stops the goroutine. Call it during shutdown.
func (s *Store) StartCleanup() (stop func()) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				now := time.Now()

				s.mu.Lock()
				for id, sess := range s.sessions {
					if now.Sub(sess.CreatedAt) > sessionTTL {
						delete(s.sessions, id)
					}
				}
				s.mu.Unlock()

				s.pendingMu.Lock()
				for key, p := range s.pending {
					if now.Sub(p.CreatedAt) > 5*time.Minute {
						delete(s.pending, key)
					}
				}
				s.pendingMu.Unlock()

				s.codesMu.Lock()
				for code, ac := range s.codes {
					if now.Sub(ac.CreatedAt) > 5*time.Minute {
						delete(s.codes, code)
					}
				}
				s.codesMu.Unlock()
			}
		}
	}()
	return func() { close(done) }
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
