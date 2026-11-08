package auth

import (
	"strings"
	"sync"
	"time"
)

// Sign-in limits (research R8, FR-019).
const (
	maxFailures = 5
	failWindow  = 15 * time.Minute
	maxTracked  = 10_000
)

// Limiter counts failed sign-ins per username, ignoring case, in a sliding 15-minute window
// and refuses a username with 5 failures in it. Attempts still being checked count too, so
// concurrent guesses cannot all pass before the first failure is recorded. It never looks at
// the client address (FR-020). Counts live in memory and reset on restart. Safe for
// concurrent use.
type Limiter struct {
	now      func() time.Time
	mu       sync.Mutex
	failures map[string][]time.Time // oldest first, only those inside the window
	pending  map[string]int         // attempts allowed by Check and not yet Done
	sweepAt  int                    // sweep expired usernames when this many are tracked
}

// NewLimiter returns an empty Limiter that reads the time from now.
func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{now: now, failures: make(map[string][]time.Time), pending: make(map[string]int), sweepAt: maxTracked}
}

// Check reports whether username may try to sign in now. When it may, the attempt counts as a
// failure until the caller reports its result with Done, which it must always do. When it may
// not, retryAfter is the time until its oldest counted failure leaves the window, or one
// second when only attempts still being checked fill the limit.
func (l *Limiter) Check(username string) (retryAfter time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	key := strings.ToLower(username)
	fs := l.prune(key, now)
	if len(fs)+l.pending[key] < maxFailures {
		l.pending[key]++
		return 0, true
	}
	if len(fs) < maxFailures {
		return time.Second, false
	}
	return fs[0].Add(failWindow).Sub(now), false
}

// Done ends an attempt that Check allowed for username, recording a failure now if failed.
func (l *Limiter) Done(username string, failed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := strings.ToLower(username)
	if l.pending[key]--; l.pending[key] <= 0 {
		delete(l.pending, key)
	}
	if !failed {
		return
	}
	now := l.now()
	fs := l.prune(key, now)
	if fs == nil && len(l.failures) >= l.sweepAt {
		l.sweep(now)
	}
	l.failures[key] = append(fs, now)
}

// prune drops key's failures that have left the window and returns the rest.
func (l *Limiter) prune(key string, now time.Time) []time.Time {
	fs := l.failures[key]
	i := 0
	for i < len(fs) && now.Sub(fs[i]) >= failWindow {
		i++
	}
	if i == len(fs) {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = fs[i:]
	return fs[i:]
}

// sweep drops every username with no failure left in the window. Usernames with failures in
// the window are never dropped: evicting one would reset its count, so an attacker spraying
// usernames could buy more guesses at a victim. Every failure follows a password verify, which
// the hash slots throttle, so at most a few verifies per second times 15 minutes are tracked.
// Sweeping again only once the map doubles keeps this O(1) per failure on average.
func (l *Limiter) sweep(now time.Time) {
	for k := range l.failures {
		l.prune(k, now)
	}
	l.sweepAt = max(maxTracked, 2*len(l.failures))
}
