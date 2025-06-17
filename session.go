package slide

import (
	"sync"
	"time"
)

// SessionMetadata represents public data regarding a session.
type SessionMetadata struct {
	Created time.Time
	Updated time.Time
}

// Session represents a unique session.
type Session[T any] struct {
	mu       sync.Mutex
	data     T
	events   map[string]struct{}
	retired  bool
	metadata SessionMetadata
}

func (s *Session[T]) Data() T {
	return s.data
}

// shouldRemove returns true if a session should be removed by the janitor.
func (s *Session[T]) janitorShouldRemove(
	now time.Time,
	inactivityTimeout time.Duration,
	maxSessionTimeout *time.Duration,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.events) == 0 {
		// ongoing for too long
		if maxSessionTimeout != nil {
			return now.Sub(s.metadata.Created) > *maxSessionTimeout
		}
		// idle for too long
		return now.Sub(s.metadata.Updated) > inactivityTimeout
	}
	return false
}
