package importer

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// cacheTTL is how long a batch list is reused without refresh (research R5).
const cacheTTL = 5 * time.Minute

// notAttempted is the message of batches an import skipped after the provider stopped
// answering (FR-025a).
const notAttempted = "Not attempted: Brewfather did not answer in time"

// Import outcomes, as the API returns them.
const (
	OutcomeImported        = "imported"
	OutcomeAlreadyImported = "already_imported"
	OutcomeArchived        = "archived"
	OutcomeNotFound        = "not_found"
	OutcomeRateLimited     = "rate_limited"
	OutcomeFailed          = "failed"
)

// BatchSummary is one batch in the list the API returns. BeerID is the beer imported from it,
// read from the database on every list, or nil.
type BatchSummary struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Number   *int64  `json:"number"`
	Status   string  `json:"status"`
	BrewDate *string `json:"brew_date"`
	BeerID   *int64  `json:"beer_id"`
}

// BatchList is the batch list and when it was fetched from the provider.
type BatchList struct {
	Batches   []BatchSummary `json:"batches"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// ImportResult is the outcome of importing one batch. Beer is the new beer, or the existing one
// for already_imported; Adjusted names the fields Sanitize changed and is never nil.
type ImportResult struct {
	BatchID  string      `json:"batch_id"`
	Outcome  string      `json:"outcome"`
	Beer     *beers.Beer `json:"beer"`
	Adjusted []string    `json:"adjusted"`
	Message  *string     `json:"message"`
}

// Batches returns every batch of the connected account that is not archived, newest brew date
// first and undated last, with each one's imported beer. The provider's list is reused for 5
// minutes unless refresh is set. It returns a RateLimitedError without a provider call while
// the rate gate is closed, and ErrNotConfigured, ErrNotConnected or ErrReconnectNeeded.
func (s *Service) Batches(ctx context.Context, refresh bool) (BatchList, error) {
	if err := s.gate(); err != nil {
		return BatchList{}, err
	}
	p, err := s.provider(ctx)
	if err != nil {
		return BatchList{}, err
	}
	s.mu.Lock()
	list, at, gen := s.cache, s.cacheAt, s.cacheGen
	s.mu.Unlock()
	if refresh || list == nil || s.Now().Sub(at) >= cacheTTL {
		if list, err = p.ListBatches(ctx); err != nil {
			return BatchList{}, s.observe(err)
		}
		list, at = s.keep(list, gen)
	}
	ids, err := beers.ImportedIDs(s.DB, providerName)
	if err != nil {
		return BatchList{}, err
	}
	out := BatchList{Batches: make([]BatchSummary, len(list)), FetchedAt: at}
	for i, b := range list {
		out.Batches[i] = BatchSummary{ID: b.Ref, Name: b.Name, Number: b.Number, Status: b.Status, BrewDate: b.BrewDate}
		if id, ok := ids[b.Ref]; ok {
			out.Batches[i].BeerID = &id
		}
	}
	return out, nil
}

// keep sorts a fresh copy of list newest first and caches it as fetched now, unless the cache
// was dropped since generation gen (a connect or disconnect while the list was in flight). It
// returns the sorted copy, never nil, and its time.
func (s *Service) keep(list []Batch, gen uint64) ([]Batch, time.Time) {
	list = slices.Clone(list)
	if list == nil {
		list = []Batch{}
	}
	slices.SortStableFunc(list, newestFirst)
	at := s.Now()
	s.mu.Lock()
	if s.cacheGen == gen {
		s.cache, s.cacheAt = list, at
	}
	s.mu.Unlock()
	return list, at
}

// newestFirst orders batches by brew date, newest first, undated last.
func newestFirst(a, b Batch) int {
	switch {
	case a.BrewDate == nil || b.BrewDate == nil:
		return cmp.Compare(boolInt(a.BrewDate == nil), boolInt(b.BrewDate == nil))
	default:
		return cmp.Compare(*b.BrewDate, *a.BrewDate)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Import imports each batch in ids as a beer and returns one result per id, in order. Each
// batch is read fresh; a batch already linked to a beer is reported with that beer and no
// provider call; an archived batch is refused; a batch's values are cleaned by beers.Sanitize
// and a hand import clears any dismissal of it. A rate limit stops the rest (rate_limited); an
// unreachable provider or an expired ctx stops the rest (failed, "Not attempted"); any other
// failure affects only its batch. It returns a RateLimitedError without a provider call while
// the rate gate is closed, and ErrNotConfigured, ErrNotConnected or ErrReconnectNeeded.
func (s *Service) Import(ctx context.Context, ids []string) ([]ImportResult, error) {
	if err := s.gate(); err != nil {
		return nil, err
	}
	p, err := s.provider(ctx)
	if err != nil {
		return nil, err
	}
	linked, err := beers.ImportedIDs(s.DB, providerName)
	if err != nil {
		return nil, err
	}
	results := make([]ImportResult, len(ids))
	var stop *ImportResult
	for i, id := range ids {
		r := &results[i]
		*r = ImportResult{BatchID: id, Adjusted: []string{}}
		switch {
		case stop != nil:
			r.Outcome, r.Message = stop.Outcome, stop.Message
		case ctx.Err() != nil:
			r.Outcome, r.Message = OutcomeFailed, ptr(notAttempted)
			stop = r
		default:
			if s.importOne(ctx, p, linked, r) {
				stop = &ImportResult{Outcome: r.Outcome, Message: ptr(notAttempted)}
				if r.Outcome == OutcomeRateLimited {
					stop.Message = r.Message
				}
			}
		}
	}
	return results, nil
}

// importOne fills r for one batch and reports whether the rest of the import must stop.
func (s *Service) importOne(ctx context.Context, p Provider, linked map[string]int64, r *ImportResult) (stop bool) {
	if id, ok := linked[r.BatchID]; ok {
		return s.alreadyImported(ctx, id, r)
	}
	var b Batch
	err := s.gate()
	if err == nil {
		b, err = p.GetBatch(ctx, r.BatchID)
		err = s.observe(err)
	}
	var rl *RateLimitedError
	switch {
	case errors.As(err, &rl):
		r.Outcome, r.Message = OutcomeRateLimited, ptr("Brewfather's rate limit is reached.")
		return true
	case errors.Is(err, ErrBatchNotFound):
		r.Outcome = OutcomeNotFound
		return false
	case errors.Is(err, ErrUnreachable) || ctx.Err() != nil:
		r.Outcome, r.Message = OutcomeFailed, ptr("Brewfather could not be reached.")
		return true
	case err != nil:
		r.Outcome, r.Message = OutcomeFailed, ptr("Brewfather could not read this batch.")
		return false
	case b.Archived:
		r.Outcome, r.Message = OutcomeArchived, ptr("This batch is archived in Brewfather.")
		return false
	}
	v, adjusted := sanitize(b)
	beer, err := beers.CreateImported(s.DB, providerName, r.BatchID, v, false, s.Now())
	if errors.Is(err, beers.ErrAlreadyImported) {
		// Another request imported it meanwhile.
		ids, err := beers.ImportedIDs(s.DB, providerName)
		if err != nil {
			return s.saveFailed(ctx, r, err)
		}
		return s.alreadyImported(ctx, ids[r.BatchID], r)
	}
	if err == nil {
		err = beers.ClearDismissal(s.DB, providerName, r.BatchID)
	}
	if err != nil {
		return s.saveFailed(ctx, r, err)
	}
	r.Outcome, r.Beer, r.Adjusted = OutcomeImported, &beer, adjusted
	return false
}

// alreadyImported fills r with beer id as already_imported.
func (s *Service) alreadyImported(ctx context.Context, id int64, r *ImportResult) bool {
	beer, err := beers.Get(s.DB, id)
	if err != nil {
		return s.saveFailed(ctx, r, err)
	}
	r.Outcome, r.Beer = OutcomeAlreadyImported, &beer
	return false
}

// saveFailed logs a database error (which holds no credential) and fills r as failed.
func (s *Service) saveFailed(ctx context.Context, r *ImportResult, err error) bool {
	slog.ErrorContext(ctx, "brewfather import failed", "provider", providerName, "err", err)
	r.Outcome, r.Message = OutcomeFailed, ptr("The beer could not be saved.")
	return false
}

// syncedFields are the beer fields a re-sync overwrites (FR-017).
var syncedFields = []string{"abv", "ibu", "srm", "brew_date", "status"}

// sanitize cleans b's values for a beer and names the fields it changed.
func sanitize(b Batch) (beers.Values, []string) {
	return beers.Sanitize(beers.Input{Name: b.Name, Style: b.Style, ABV: b.ABV, IBU: b.IBU, SRM: b.SRM,
		Description: b.Notes, BrewDate: b.BrewDate, Status: &b.Status})
}

// Resync reads beer id's batch fresh and overwrites only its ABV, IBU, SRM, brew date and
// status, archiving it by beers.ApplySync's rules; archived beers may be re-synced. It returns
// the beer and the synced fields Sanitize changed (never nil). A batch the provider no longer
// has marks the beer not found and changes nothing else, with no error. It returns
// beers.ErrNotFound for an unknown id and ErrNotImported for a beer from another source, both
// without a provider call; a RateLimitedError without a call while the gate is closed;
// ErrNotConfigured, ErrNotConnected or ErrReconnectNeeded; and the provider's error, writing
// nothing.
func (s *Service) Resync(ctx context.Context, id int64) (beers.Beer, []string, error) {
	beer, err := beers.Get(s.DB, id)
	if err != nil {
		return beers.Beer{}, nil, err
	}
	if beer.Source != providerName || beer.SourceRef == nil {
		return beers.Beer{}, nil, ErrNotImported
	}
	if err := s.gate(); err != nil {
		return beers.Beer{}, nil, err
	}
	p, err := s.provider(ctx)
	if err != nil {
		return beers.Beer{}, nil, err
	}
	b, err := p.GetBatch(ctx, *beer.SourceRef)
	if errors.Is(err, ErrBatchNotFound) {
		beer, err = beers.MarkNotFound(s.DB, id)
		return beer, []string{}, err
	}
	if err != nil {
		return beers.Beer{}, nil, s.observe(err)
	}
	v, adjusted := sanitize(b)
	if beer, err = beers.ApplySync(s.DB, id, v, b.Archived, s.Now()); err != nil {
		return beers.Beer{}, nil, err
	}
	return beer, slices.DeleteFunc(adjusted, func(f string) bool { return !slices.Contains(syncedFields, f) }), nil
}

func ptr[T any](v T) *T { return &v }
