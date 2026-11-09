package auth

import (
	"strconv"
	"testing"
	"time"
)

// fail runs one allowed attempt for username that fails.
func fail(t *testing.T, l *Limiter, username string) {
	t.Helper()
	if _, ok := l.Check(username); !ok {
		t.Fatalf("check for %s refused", username)
	}
	l.Done(username, true)
}

func TestLimiter(t *testing.T) {
	now := t0
	l := NewLimiter(func() time.Time { return now })
	for range 5 {
		fail(t, l, "owner")
		now = now.Add(time.Minute)
	}
	// Failures at t0..t0+4m; now is t0+5m, so the oldest leaves the window in 10 minutes.
	retry, ok := l.Check("owner")
	if ok || retry != 10*time.Minute {
		t.Fatalf("6th check = %v, %v; want refused with 10m", retry, ok)
	}
	if _, ok := l.Check("OWNER"); ok {
		t.Fatal("OWNER does not share owner's count")
	}
	if _, ok := l.Check("other"); !ok {
		t.Fatal("another username is limited")
	}
	l.Done("other", false)
	now = t0.Add(15*time.Minute - time.Second)
	if _, ok := l.Check("owner"); ok {
		t.Fatal("allowed before the oldest failure is 15 minutes old")
	}
	now = t0.Add(15 * time.Minute)
	if _, ok := l.Check("owner"); !ok {
		t.Fatal("still refused after the oldest failure left the window")
	}
	l.Done("Owner", true) // back to 5 in the window: t0+1m..t0+4m and now
	if _, ok := l.Check("owner"); ok {
		t.Fatal("window did not slide: 5 failures in it but allowed")
	}
}

func TestLimiterCountsPendingAttempts(t *testing.T) {
	now := t0
	l := NewLimiter(func() time.Time { return now })
	for i := range 5 {
		if _, ok := l.Check("owner"); !ok {
			t.Fatalf("check %d refused", i+1)
		}
	}
	retry, ok := l.Check("Owner")
	if ok || retry != time.Second {
		t.Fatalf("6th concurrent check = %v, %v; want refused with 1s", retry, ok)
	}
	l.Done("owner", false) // a success or an error frees its attempt without a failure
	if _, ok := l.Check("owner"); !ok {
		t.Fatal("check refused after an attempt ended without failing")
	}
	for range 5 {
		l.Done("owner", true)
	}
	if _, ok := l.Check("owner"); ok {
		t.Fatal("allowed with 5 failures in the window")
	}
	if len(l.pending) != 0 {
		t.Fatalf("pending = %v after every attempt is done, want empty", l.pending)
	}
}

func TestLimiterSprayKeepsVictimCount(t *testing.T) {
	now := t0
	l := NewLimiter(func() time.Time { return now })
	for range 5 {
		fail(t, l, "owner")
	}
	// Spraying other usernames must not evict owner and reset its count.
	for i := range maxTracked + 1 {
		fail(t, l, "user"+strconv.Itoa(i))
	}
	if _, ok := l.Check("owner"); ok {
		t.Fatal("owner allowed again after a username spray")
	}
}

func TestLimiterSweepsExpired(t *testing.T) {
	now := t0
	l := NewLimiter(func() time.Time { return now })
	for i := range maxTracked {
		fail(t, l, "user"+strconv.Itoa(i))
	}
	now = now.Add(failWindow)
	fail(t, l, "late")
	if n := len(l.failures); n != 1 {
		t.Fatalf("tracked %d usernames after their failures expired, want 1", n)
	}
}
