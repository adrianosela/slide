package slide

import "time"

// Option represents a configuration option for a Tracker.
type Option[T any] func(*config[T])

// WithJanitorInterval sets the Tracker's janitor interval.
func WithJanitorInterval[T any](janitorInterval time.Duration) Option[T] {
	return func(c *config[T]) { c.janitorInterval = janitorInterval }
}

// WithInactivityTimeout sets the Tracker's inactivity timeout i.e.
// how long of a period of inactivity must pass before any new event
// is considered a new session.
func WithInactivityTimeout[T any](inactivityTimeout time.Duration) Option[T] {
	return func(c *config[T]) { c.inactivityTimeout = inactivityTimeout }
}

// WithMaxSessionTimeout sets the Tracker's max session timeout i.e.
// how long a session can be before any new events produce a new session.
func WithMaxSessionTimeout[T any](maxSessionTimeout time.Duration) Option[T] {
	return func(c *config[T]) { c.maxSessionTimeout = &maxSessionTimeout }
}

// WithOnSessionEnd sets the Tracker's onSessionEnd handler, i.e.
// a function that will be ran on every session after its done.
func WithOnSessionEnd[T any](onSessionEnd OnEndFunc[T]) Option[T] {
	return func(c *config[T]) { c.onSessionEnd = onSessionEnd }
}
