package settings

import "sync"

// Session holds the password-derived key in memory for the server process's
// lifetime — there is no per-request auth token: this is a local, single-user
// desktop app, so "unlocked" is simply a process-wide state that Setup or a
// successful Unlock puts it into, and that a process restart clears.
type Session struct {
	mu  sync.RWMutex
	key []byte
}

// NewSession starts locked (or, if the app was never set up, permanently
// "locked" in the sense that Setup must run before there's anything to
// unlock).
func NewSession() *Session {
	return &Session{}
}

// Unlock stores key for subsequent Key() calls.
func (s *Session) Unlock(key []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
}

// Lock discards the held key.
func (s *Session) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = nil
}

// Key returns the held key and whether the session is unlocked.
func (s *Session) Key() ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.key, s.key != nil
}
