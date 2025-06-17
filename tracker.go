package slide

import (
	"fmt"
	"maps"
	"sync"
	"time"
)

// tracker is the default implementation of Tracker.
type tracker[T any] struct {
	mu              sync.RWMutex
	sessions        map[string]*Session[T]
	eventToSession  map[string]*Session[T]
	retiredSessions map[*Session[T]]struct{}
	ticker          *time.Ticker

	sessionInitFunc   SessionInitFunc[T]
	inactivityTimeout time.Duration
	maxSessionTimeout *time.Duration
	onSessionEnd      OnEndFunc[T]
}

// config represents internal tracker configuration.
type config[T any] struct {
	janitorInterval   time.Duration
	inactivityTimeout time.Duration
	maxSessionTimeout *time.Duration
	onSessionEnd      OnEndFunc[T]
}

// NewTracker returns a new sliding window tracker.
func NewTracker[T any](sessionInitFunc SessionInitFunc[T], opts ...Option[T]) Tracker[T] {
	cfg := &config[T]{
		janitorInterval:   30 * time.Second,
		inactivityTimeout: 15 * time.Minute,
		maxSessionTimeout: nil,
		onSessionEnd:      nil,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	t := &tracker[T]{
		mu:                sync.RWMutex{},
		sessions:          make(map[string]*Session[T]),
		eventToSession:    make(map[string]*Session[T]),
		retiredSessions:   make(map[*Session[T]]struct{}),
		ticker:            time.NewTicker(cfg.janitorInterval),
		sessionInitFunc:   sessionInitFunc,
		inactivityTimeout: cfg.inactivityTimeout,
		maxSessionTimeout: cfg.maxSessionTimeout,
		onSessionEnd:      cfg.onSessionEnd,
	}
	go t.janitor()
	return t
}

// Stop stops the tracker.
func (t *tracker[T]) Stop() {
	t.ticker.Stop()
}

// EventStart marks the start of a session's event.
func (t *tracker[T]) EventStart(
	sessionDedupKey string,
	eventID string,
) *Session[T] {
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	session, exists := t.sessions[sessionDedupKey]
	if exists {
		if t.maxSessionTimeout != nil {
			session.mu.Lock()
			expired := now.Sub(session.metadata.Created) > *t.maxSessionTimeout
			session.mu.Unlock()

			// if the current session is expired, we need to
			// retire it and replace it with a new session.
			if expired {
				session.mu.Lock()
				session.retired = true
				delete(session.events, eventID)
				session.mu.Unlock()

				t.retiredSessions[session] = struct{}{}

				newSession := &Session[T]{
					mu:     sync.Mutex{},
					data:   t.sessionInitFunc(sessionDedupKey),
					events: map[string]struct{}{eventID: {}},
					metadata: SessionMetadata{
						Created: now,
						Updated: now,
					},
				}
				t.sessions[sessionDedupKey] = newSession
				t.eventToSession[eventID] = newSession
				return newSession
			}
		}

		// the current session is still valid, so we re-use it
		session.mu.Lock()
		session.events[eventID] = struct{}{}
		t.eventToSession[eventID] = session
		session.metadata.Updated = now
		session.mu.Unlock()
		return session
	}

	// no session found, create a new one.
	newSession := &Session[T]{
		mu:     sync.Mutex{},
		data:   t.sessionInitFunc(sessionDedupKey),
		events: map[string]struct{}{eventID: {}},
		metadata: SessionMetadata{
			Created: now,
			Updated: now,
		},
	}
	t.sessions[sessionDedupKey] = newSession
	t.eventToSession[eventID] = newSession
	return newSession
}

// EventEnd marks the end of session's event.
func (t *tracker[T]) EventEnd(
	eventID string,
) error {
	// Try to locate in active sessions
	t.mu.RLock()
	session, ok := t.eventToSession[eventID]
	t.mu.RUnlock()
	if !ok {
		return fmt.Errorf("event %s is not associated with any session", eventID)
	}

	session.mu.Lock()
	delete(session.events, eventID)
	session.metadata.Updated = time.Now()
	shouldDelete := len(session.events) == 0 && session.retired
	session.mu.Unlock()

	// Clean up the mapping
	t.mu.Lock()
	delete(t.eventToSession, eventID)
	t.mu.Unlock()

	if shouldDelete {
		t.mu.Lock()
		delete(t.retiredSessions, session)
		t.mu.Unlock()
		if t.onSessionEnd != nil {
			go t.onSessionEnd(session.data, &session.metadata)
		}
	}

	return nil
}

// janitor is the clean-up loop of the tracker.
func (t *tracker[T]) janitor() {
	for range t.ticker.C {
		now := time.Now()

		// clean up active sessions

		t.mu.RLock()
		snapshot := make(map[string]*Session[T], len(t.sessions))
		maps.Copy(snapshot, t.sessions)
		t.mu.RUnlock()

		for dedupKey, session := range snapshot {
			if session.janitorShouldRemove(now, t.inactivityTimeout, t.maxSessionTimeout) {
				t.mu.Lock()
				current, ok := t.sessions[dedupKey]
				if ok && current == session {
					delete(t.sessions, dedupKey)
				}
				t.mu.Unlock()

				if ok && current == session && t.onSessionEnd != nil {
					go t.onSessionEnd(session.data, &session.metadata)
				}
			}
		}

		// clean up retired sessions

		t.mu.RLock()
		snapshotRetired := make([]*Session[T], 0, len(t.retiredSessions))
		for sess := range t.retiredSessions {
			snapshotRetired = append(snapshotRetired, sess)
		}
		t.mu.RUnlock()

		for _, session := range snapshotRetired {
			if session.janitorShouldRemove(now, t.inactivityTimeout, t.maxSessionTimeout) {
				t.mu.Lock()
				delete(t.retiredSessions, session)
				t.mu.Unlock()

				if t.onSessionEnd != nil {
					go t.onSessionEnd(session.data, &session.metadata)
				}
			}
		}
	}
}
