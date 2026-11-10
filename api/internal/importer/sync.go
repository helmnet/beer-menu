package importer

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// SetInterval stores the scheduled sync interval: off, 15m, 1h, 6h or 24h. It returns
// ErrInvalidInterval, storing nothing, for any other value.
func (s *Service) SetInterval(ctx context.Context, interval string) error {
	if err := saveInterval(s.DB, interval, s.Now()); err != nil {
		return err
	}
	slog.InfoContext(ctx, "brewfather sync interval set", "provider", providerName, "interval", interval)
	return nil
}

// Sync runs one sync and returns when it ends (research R10, FR-021): one ListBatches; every
// imported beer not archived locally whose batch is listed gets ApplySync; one missing from the
// list gets one GetBatch (archived: ApplySync with the archive rules; gone: marked not found),
// unless it is already marked not found; then every listed fermenting or conditioning batch
// not linked to a beer and not dismissed is imported from the list. Each beer's change is its
// own statement; one beer failing is named in the run's message and the rest go on, except that
// once a GetBatch finds the provider unreachable the remaining missing beers fail uncalled. The run
// is recorded in sync_runs. It returns ErrSyncRunning while another sync runs; ErrNotConfigured,
// ErrNotConnected, ErrReconnectNeeded or a RateLimitedError (gate closed) without recording a
// run; and the list call's error after recording a failed run.
func (s *Service) Sync(ctx context.Context) error {
	p, err := s.beginSync(ctx)
	if err != nil {
		return err
	}
	defer s.syncing.Store(false)
	return s.runSync(ctx, p)
}

// StartSync starts the same sync as Sync in the background on ctx, which should end only at
// shutdown, and returns once the run is recorded as started. It returns Sync's errors that come
// before a run starts. Wait waits for it to end.
func (s *Service) StartSync(ctx context.Context) error {
	p, err := s.beginSync(ctx)
	if err != nil {
		return err
	}
	s.background.Add(1)
	go func() {
		defer s.background.Done()
		defer s.syncing.Store(false)
		// runSync records and logs the outcome; nobody waits on the error.
		_ = s.runSync(ctx, p)
	}()
	return nil
}

// Wait returns when every sync StartSync started has ended.
func (s *Service) Wait() { s.background.Wait() }

// beginSync takes the sync lock, checks the account and the rate gate, and records the run as
// started. On error the lock is released and no run is recorded.
func (s *Service) beginSync(ctx context.Context) (p Provider, err error) {
	if !s.syncing.CompareAndSwap(false, true) {
		return nil, ErrSyncRunning
	}
	defer func() {
		if err != nil {
			s.syncing.Store(false)
		}
	}()
	if err := s.gate(); err != nil {
		return nil, err
	}
	if p, err = s.provider(ctx); err != nil {
		return nil, err
	}
	if err := startRun(s.DB, s.Now()); err != nil {
		return nil, err
	}
	return p, nil
}

// syncResult counts a run's work. failed holds the names of beers or batches that could not be
// synced.
type syncResult struct {
	updated, imported, notFound int64
	failed                      []string
}

// runSync does a started run's work and records its end. The lock must be held.
func (s *Service) runSync(ctx context.Context, p Provider) error {
	var res syncResult
	s.mu.Lock()
	gen := s.cacheGen
	s.mu.Unlock()
	list, err := p.ListBatches(ctx)
	if err != nil {
		err = s.observe(err)
		s.endRun(ctx, res, listFailure(err), err)
		return err
	}
	list, _ = s.keep(list, gen)
	if err := s.resyncAll(ctx, p, list, &res); err != nil {
		s.endRun(ctx, res, "The beer library could not be read.", err)
		return err
	}
	if err := s.importNew(list, &res); err != nil {
		s.endRun(ctx, res, "The beer library could not be read.", err)
		return err
	}
	s.endRun(ctx, res, "", nil)
	return nil
}

// resyncAll syncs every imported beer that is not archived locally from list, or from its own
// GetBatch when list lacks it. It returns an error only when the beers cannot be read.
func (s *Service) resyncAll(ctx context.Context, p Provider, list []Batch, res *syncResult) error {
	listed := make(map[string]Batch, len(list))
	for _, b := range list {
		listed[b.Ref] = b
	}
	local, err := beers.List(s.DB, false)
	if err != nil {
		return err
	}
	// unreachable is set once a GetBatch finds the provider gone or ctx ended; later missing
	// beers then fail without a call, as Import stops, instead of each waiting out a timeout.
	unreachable := false
	for _, beer := range local {
		if beer.Source != providerName || beer.SourceRef == nil {
			continue
		}
		b, ok := listed[*beer.SourceRef]
		if !ok && beer.Sync.NotFound {
			// FR-021: only a hand re-sync checks a batch already missing.
			res.notFound++
			continue
		}
		if !ok {
			if unreachable {
				res.failed = append(res.failed, beer.Name)
				continue
			}
			err := s.gate()
			if err == nil {
				b, err = p.GetBatch(ctx, *beer.SourceRef)
				err = s.observe(err)
				unreachable = errors.Is(err, ErrUnreachable) || ctx.Err() != nil
			}
			if errors.Is(err, ErrBatchNotFound) {
				s.count(ctx, res, beer.Name, &res.notFound, func() error { _, err := beers.MarkNotFound(s.DB, beer.ID); return err })
				continue
			}
			if err != nil {
				res.failed = append(res.failed, beer.Name)
				continue
			}
		}
		v, _ := sanitize(b)
		s.count(ctx, res, beer.Name, &res.updated, func() error {
			_, err := beers.ApplySync(s.DB, beer.ID, v, b.Archived, s.Now())
			return err
		})
	}
	return nil
}

// importNew imports every listed fermenting or conditioning batch that is not linked to a beer
// and not dismissed, from the list's values. It returns an error only when the links or
// dismissals cannot be read.
func (s *Service) importNew(list []Batch, res *syncResult) error {
	linked, err := beers.ImportedIDs(s.DB, providerName)
	if err != nil {
		return err
	}
	dismissed, err := beers.DismissedRefs(s.DB, providerName)
	if err != nil {
		return err
	}
	for _, b := range list {
		if b.Archived || (b.Status != "fermenting" && b.Status != "conditioning") {
			continue
		}
		if _, ok := linked[b.Ref]; ok || dismissed[b.Ref] {
			continue
		}
		v, _ := sanitize(b)
		_, err := beers.CreateImported(s.DB, providerName, b.Ref, v, false, s.Now())
		switch {
		case errors.Is(err, beers.ErrAlreadyImported):
			// A hand import got there first.
		case err != nil:
			slog.Error("brewfather sync could not import a batch", "provider", providerName, "err", err)
			res.failed = append(res.failed, v.Name)
		default:
			res.imported++
		}
	}
	return nil
}

// count runs one beer's write and adds it to n, skips a beer deleted meanwhile, and records
// any other failure under name.
func (s *Service) count(ctx context.Context, res *syncResult, name string, n *int64, write func() error) {
	switch err := write(); {
	case err == nil:
		*n++
	case errors.Is(err, beers.ErrNotFound):
		// Deleted during the run: nothing left to sync.
	default:
		slog.ErrorContext(ctx, "brewfather sync could not save a beer", "provider", providerName, "err", err)
		res.failed = append(res.failed, name)
	}
}

// endRun records the run's end: failed with failure when it is set, else failed naming
// res.failed when any, else ok. err is the run-stopping error, logged when set; it holds no
// credential or provider body.
func (s *Service) endRun(ctx context.Context, res syncResult, failure string, err error) {
	ok, msg := failure == "" && len(res.failed) == 0, "Sync finished."
	switch {
	case failure != "":
		msg = failure
	case len(res.failed) == 1:
		msg = "Could not sync 1 beer: " + res.failed[0] + "."
	case len(res.failed) > 1:
		msg = "Could not sync " + strconv.Itoa(len(res.failed)) + " beers: " + strings.Join(res.failed, ", ") + "."
	}
	attrs := []any{"provider", providerName, "ok", ok, "updated", res.updated, "imported", res.imported,
		"not_found", res.notFound, "failed", len(res.failed)}
	if err != nil {
		attrs = append(attrs, "err", err)
	}
	slog.InfoContext(ctx, "brewfather sync", attrs...)
	if err := finishRun(s.DB, s.Now(), ok, msg, res.updated, res.imported, res.notFound); err != nil {
		slog.ErrorContext(ctx, "brewfather sync could not record its end", "provider", providerName, "err", err)
	}
}

// listFailure is the plain message of a run whose batch list failed.
func listFailure(err error) string {
	var rl *RateLimitedError
	switch {
	case errors.As(err, &rl):
		return "Brewfather's rate limit is reached; the sync will run again later."
	case errors.Is(err, ErrCredentialsRefused):
		return "Brewfather refused the stored credentials; connect again."
	case errors.Is(err, ErrNoBatchAccess):
		return "The API key cannot read batches; give it the batches.read scope."
	default:
		return "Brewfather could not be reached."
	}
}
