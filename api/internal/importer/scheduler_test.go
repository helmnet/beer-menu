package importer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/importer"
)

// tickRuns calls s.Tick and reports whether it ran a sync, by the provider's list calls.
func tickRuns(t *testing.T, s *importer.Service, lists func() int) bool {
	t.Helper()
	before := lists()
	if err := s.Tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}
	return lists() != before
}

func TestTick(t *testing.T) {
	s, fake, clock := newService(t)
	ctx := context.Background()
	lists := func() int { return fake.Calls("ListBatches") }

	if err := s.SetInterval(ctx, "15m"); err != nil {
		t.Fatal(err)
	}
	if tickRuns(t, s, lists) {
		t.Fatal("not connected but synced")
	}
	mustConnect(t, s)
	if err := s.SetInterval(ctx, "off"); err != nil {
		t.Fatal(err)
	}
	if tickRuns(t, s, lists) {
		t.Fatal("interval off but synced")
	}
	if err := s.SetInterval(ctx, "every hour"); !errors.Is(err, importer.ErrInvalidInterval) {
		t.Fatalf("invalid interval = %v", err)
	}
	if err := s.SetInterval(ctx, "15m"); err != nil {
		t.Fatal(err)
	}

	// Never ran: due at once. Then due 15 minutes after the last start.
	if !tickRuns(t, s, lists) {
		t.Fatal("never ran but no sync")
	}
	clock.t = t0.Add(15*time.Minute - time.Second)
	if tickRuns(t, s, lists) {
		t.Fatal("synced before the interval")
	}
	clock.t = t0.Add(15 * time.Minute)
	if !tickRuns(t, s, lists) {
		t.Fatal("due but no sync")
	}

	// A restart keeps the cadence from sync_runs.
	restarted := importer.New(s.DB, s.Box, fake.Factory(), clock.Now)
	clock.t = t0.Add(29 * time.Minute)
	if tickRuns(t, restarted, lists) {
		t.Fatal("restarted service synced before the interval")
	}
	clock.t = t0.Add(30 * time.Minute)
	if !tickRuns(t, restarted, lists) {
		t.Fatal("restarted service did not sync when due")
	}

	// A closed rate gate holds the sync until retry_at.
	until := t0.Add(50 * time.Minute)
	fake.SetErr("ListBatches", &importer.RateLimitedError{Until: until})
	if _, err := s.Batches(ctx, true); err == nil {
		t.Fatal("gate did not close")
	}
	fake.SetErr("ListBatches", nil)
	clock.t = t0.Add(45 * time.Minute)
	if tickRuns(t, s, lists) {
		t.Fatal("synced behind a closed gate")
	}
	clock.t = until
	if !tickRuns(t, s, lists) {
		t.Fatal("no sync once the gate opened")
	}
}

func TestTickReconnectNeeded(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	if err := s.SetInterval(ctx, "1h"); err != nil {
		t.Fatal(err)
	}
	s.Box = importer.NewTestBox(t)
	if tickRuns(t, s, func() int { return fake.Calls("ListBatches") }) {
		t.Fatal("reconnect needed but synced")
	}
	if st, _ := s.Status(ctx); st.LastSync != nil {
		t.Fatalf("a skipped tick left a run: %+v", st.LastSync)
	}
}

func TestRunStops(t *testing.T) {
	s, _, _ := newService(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
