package slide

// SessionInitFunc is a function that will be invoked to initialize
// a new session whenever an event merits a new session.
type SessionInitFunc[T any] func(sessionDedupKey string) T

// OnEndFunc is a function that will be invoked whenever a
// session ends whether because of the inactivity timeout
// or because of the maximum session lifetime being exceeded.
type OnEndFunc[T any] func(data T, lastUpdated *SessionMetadata)

// Tracker represents a sliding window tracker.
type Tracker[T any] interface {
	Stop()
	EventStart(sessionDedupKey string, eventID string) *Session[T]
	EventEnd(eventID string) error
}
