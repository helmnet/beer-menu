package importer

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// tickEvery is how often Run checks whether a sync is due.
const tickEvery = time.Minute

// Run calls Tick every minute until ctx ends, logging a failed sync. Start it once, on a
// context that ends at shutdown; a sync it runs uses that context.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(tickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.Tick(ctx); err != nil {
				slog.ErrorContext(ctx, "brewfather scheduled sync failed", "provider", providerName, "err", err)
			}
		}
	}
}

// Tick runs one sync when one is due: the interval is not off, the rate gate is open, an
// account is connected and opens, and the last run started at least one interval ago (or
// none ever ran). The last start is read from sync_runs, so a restart keeps the cadence. A
// sync not due, or skipped for the account, the gate or a running sync, returns nil; any
// other Sync error is returned (the run already records it).
func (s *Service) Tick(ctx context.Context) error {
	interval, err := loadInterval(s.DB)
	if err != nil {
		return err
	}
	every, _ := ParseInterval(interval)
	if every == 0 {
		return nil
	}
	run, err := loadRun(s.DB)
	if err != nil {
		return err
	}
	if run != nil && s.Now().Before(run.StartedAt.Add(every)) {
		return nil
	}
	err = s.Sync(ctx)
	var rl *RateLimitedError
	if errors.As(err, &rl) || errors.Is(err, ErrSyncRunning) || errors.Is(err, ErrNotConfigured) ||
		errors.Is(err, ErrNotConnected) || errors.Is(err, ErrReconnectNeeded) {
		return nil
	}
	return err
}
