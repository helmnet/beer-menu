package importer_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
	"github.com/helmnet/beer-menu/api/internal/importer"
)

func ptr[T any](v T) *T { return &v }

func set[T any](v T) beers.Optional[T] { return beers.Optional[T]{Set: true, Value: &v} }

// batchesFixture is an account with four listed batches (one with no brew date) and an
// archived one.
func batchesFixture() []importer.Batch {
	return []importer.Batch{
		{Ref: "old", Name: "Old Ale", Number: ptr[int64](1), Status: "ready", BrewDate: ptr("2025-01-02"), ABV: ptr(7.0)},
		{Ref: "nodate", Name: "Someday", Status: "planned"},
		{Ref: "new", Name: "New IPA", Number: ptr[int64](3), Status: "fermenting", BrewDate: ptr("2026-10-30"), ABV: ptr(6.54), IBU: ptr(55.4),
			SRM: ptr(7.0), Style: ptr("American IPA"), Notes: ptr("Hazy.")},
		{Ref: "mid", Name: "Mid Stout", Status: "conditioning", BrewDate: ptr("2026-03-01")},
		{Ref: "gone", Name: "Archived Lager", Status: "ready", Archived: true, BrewDate: ptr("2026-11-01")},
	}
}

func refs(list importer.BatchList) []string {
	var out []string
	for _, b := range list.Batches {
		out = append(out, b.ID)
	}
	return out
}

func TestBatches(t *testing.T) {
	s, fake, clock := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	fake.SetBatches(batchesFixture()...)

	list, err := s.Batches(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(refs(list), ","); got != "new,mid,old,nodate" || !list.FetchedAt.Equal(t0) {
		t.Fatalf("batches = %s at %v, want new,mid,old,nodate at t0", got, list.FetchedAt)
	}
	first := list.Batches[0]
	if first.Name != "New IPA" || *first.Number != 3 || first.Status != "fermenting" || *first.BrewDate != "2026-10-30" || first.BeerID != nil {
		t.Fatalf("first batch = %+v", first)
	}

	// Served from cache, but the beer link is read fresh.
	res, err := s.Import(ctx, []string{"new"})
	if err != nil || res[0].Outcome != "imported" {
		t.Fatalf("import = %+v (err %v)", res, err)
	}
	clock.t = t0.Add(4 * time.Minute)
	list, err = s.Batches(ctx, false)
	if err != nil || fake.Calls("ListBatches") != 1 || !list.FetchedAt.Equal(t0) {
		t.Fatalf("cached batches: err %v, list calls %d, fetched %v", err, fake.Calls("ListBatches"), list.FetchedAt)
	}
	if id := list.Batches[0].BeerID; id == nil || *id != res[0].Beer.ID {
		t.Fatalf("beer_id = %v, want %d", id, res[0].Beer.ID)
	}

	if _, err := s.Batches(ctx, true); err != nil || fake.Calls("ListBatches") != 2 {
		t.Fatalf("refresh: err %v, list calls %d", err, fake.Calls("ListBatches"))
	}
	clock.t = clock.t.Add(5 * time.Minute)
	if list, err := s.Batches(ctx, false); err != nil || fake.Calls("ListBatches") != 3 || !list.FetchedAt.Equal(clock.t) {
		t.Fatalf("after 5 minutes: err %v, list calls %d", err, fake.Calls("ListBatches"))
	}

	// Connect and disconnect drop the cache.
	mustConnect(t, s)
	if _, err := s.Batches(ctx, false); err != nil || fake.Calls("ListBatches") != 4 {
		t.Fatalf("after reconnect: err %v, list calls %d", err, fake.Calls("ListBatches"))
	}
	if err := s.Disconnect(ctx); err != nil {
		t.Fatal(err)
	}
	mustConnect(t, s)
	if _, err := s.Batches(ctx, false); err != nil || fake.Calls("ListBatches") != 5 {
		t.Fatalf("after disconnect: err %v, list calls %d", err, fake.Calls("ListBatches"))
	}
}

func TestBatchesErrors(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Batches(ctx, false); !errors.Is(err, importer.ErrNotConnected) {
		t.Fatalf("not connected = %v", err)
	}
	mustConnect(t, s)
	fake.SetErr("ListBatches", importer.ErrUnreachable)
	if _, err := s.Batches(ctx, false); !errors.Is(err, importer.ErrUnreachable) {
		t.Fatalf("unreachable = %v", err)
	}
	until := t0.Add(2 * time.Minute)
	fake.SetErr("ListBatches", &importer.RateLimitedError{Until: until})
	var rl *importer.RateLimitedError
	if _, err := s.Batches(ctx, false); !errors.As(err, &rl) {
		t.Fatalf("rate limited = %v", err)
	}
	calls := fake.TotalCalls()
	if _, err := s.Batches(ctx, true); !errors.As(err, &rl) || !rl.Until.Equal(until) || fake.TotalCalls() != calls {
		t.Fatalf("closed gate = %v, calls %d -> %d", err, calls, fake.TotalCalls())
	}
	if _, err := s.Import(ctx, []string{"new"}); !errors.As(err, &rl) || fake.TotalCalls() != calls {
		t.Fatalf("import behind a closed gate = %v, calls %d -> %d", err, calls, fake.TotalCalls())
	}
}

func TestImport(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	fake.SetBatches(batchesFixture()...)
	if _, err := s.DB.Exec("INSERT INTO import_dismissed (source, ref, dismissed_at) VALUES ('brewfather', 'new', 1)"); err != nil {
		t.Fatal(err)
	}
	fake.SetErr("GetBatch:bad", errors.New("unreadable batch"))

	res, err := s.Import(ctx, []string{"new", "gone", "missing", "bad", "nodate"})
	if err != nil {
		t.Fatal(err)
	}
	var outcomes []string
	for _, r := range res {
		outcomes = append(outcomes, r.BatchID+"="+r.Outcome)
		if r.Adjusted == nil {
			t.Errorf("%s: adjusted is nil, want []", r.BatchID)
		}
	}
	if got := strings.Join(outcomes, ","); got != "new=imported,gone=archived,missing=not_found,bad=failed,nodate=imported" {
		t.Fatalf("outcomes = %s", got)
	}
	b := res[0].Beer
	if b == nil || b.Source != "brewfather" || *b.SourceRef != "new" || b.Name != "New IPA" || *b.ABV != 6.5 || *b.IBU != 55 ||
		*b.Style != "American IPA" || *b.Description != "Hazy." || *b.BrewDate != "2026-10-30" || b.Status != "fermenting" ||
		b.Brewer != nil || b.Sync == nil || !b.Sync.SyncedAt.Equal(t0) {
		t.Fatalf("imported beer = %+v", b)
	}
	if !slices.Equal(res[0].Adjusted, []string{"abv", "ibu"}) {
		t.Fatalf("adjusted = %v, want [abv ibu]", res[0].Adjusted)
	}
	if res[1].Beer != nil || res[2].Beer != nil || res[3].Beer != nil || res[3].Message == nil {
		t.Fatalf("non-imports = %+v %+v %+v", res[1], res[2], res[3])
	}
	if nd := res[4].Beer; nd.ABV != nil || nd.BrewDate != nil || nd.Status != "planned" {
		t.Fatalf("beer with missing values = %+v", nd)
	}
	var n int
	if err := s.DB.QueryRow("SELECT count(*) FROM import_dismissed").Scan(&n); err != nil || n != 0 {
		t.Fatalf("dismissals = %d (err %v), want 0", n, err)
	}

	// Already linked: the existing beer, no provider call.
	calls := fake.Calls("GetBatch")
	res, err = s.Import(ctx, []string{"new"})
	if err != nil || res[0].Outcome != "already_imported" || res[0].Beer == nil || res[0].Beer.ID != b.ID || fake.Calls("GetBatch") != calls {
		t.Fatalf("re-import = %+v (err %v), GetBatch calls %d -> %d", res, err, calls, fake.Calls("GetBatch"))
	}
	if err := s.DB.QueryRow("SELECT count(*) FROM beers").Scan(&n); err != nil || n != 2 {
		t.Fatalf("beers = %d, want 2", n)
	}
}

func TestImportStopsOnRateLimit(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	fake.SetBatches(batchesFixture()...)
	fake.SetErr("GetBatch:mid", &importer.RateLimitedError{Until: t0.Add(time.Minute)})
	res, err := s.Import(ctx, []string{"new", "mid", "old", "nodate"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range res {
		got = append(got, r.Outcome)
	}
	if strings.Join(got, ",") != "imported,rate_limited,rate_limited,rate_limited" || fake.Calls("GetBatch") != 2 {
		t.Fatalf("outcomes = %v, GetBatch calls %d", got, fake.Calls("GetBatch"))
	}
}

func TestImportStopsWhenUnreachable(t *testing.T) {
	s, fake, _ := newService(t)
	mustConnect(t, s)
	fake.SetBatches(batchesFixture()...)

	fake.SetErr("GetBatch:new", importer.ErrUnreachable)
	res, err := s.Import(context.Background(), []string{"old", "new", "mid", "nodate"})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Outcome != "imported" || res[1].Outcome != "failed" || fake.Calls("GetBatch") != 2 {
		t.Fatalf("results = %+v, GetBatch calls %d", res, fake.Calls("GetBatch"))
	}
	for _, r := range res[2:] {
		if r.Outcome != "failed" || r.Message == nil || *r.Message != "Not attempted: Brewfather did not answer in time" {
			t.Fatalf("untried = %+v", r)
		}
	}

	// An expired context stops the rest too.
	fake.SetErr("GetBatch:new", nil)
	fake.SetBlock(true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	res, err = s.Import(ctx, []string{"mid", "nodate"})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Outcome != "failed" || res[1].Outcome != "failed" || *res[1].Message != "Not attempted: Brewfather did not answer in time" {
		t.Fatalf("results after deadline = %+v %+v", res[0], res[1])
	}
}

// importOne imports batch ref through s and returns the new beer's id.
func importOne(t *testing.T, s *importer.Service, ref string) int64 {
	t.Helper()
	res, err := s.Import(context.Background(), []string{ref})
	if err != nil || res[0].Outcome != "imported" {
		t.Fatalf("import %s = %+v (err %v)", ref, res, err)
	}
	return res[0].Beer.ID
}

// mustJSON returns v as JSON text.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// addKeg gives beer a current keg, waiting (not on a tap).
func addKeg(t *testing.T, s *importer.Service, beer int64) {
	t.Helper()
	if _, err := s.DB.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, created_at, updated_at) VALUES (?, 1, 1, 1, 1)", beer); err != nil {
		t.Fatal(err)
	}
}

func TestResync(t *testing.T) {
	s, fake, clock := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	fake.SetBatches(batchesFixture()...)
	id := importOne(t, s, "new")
	if _, err := beers.Update(s.DB, id, beers.Patch{Name: set("My IPA"), Description: set("Mine.")}, t0); err != nil {
		t.Fatal(err)
	}

	// Measurements and status follow the batch; the owner's name and description stay.
	clock.t = t0.Add(time.Hour)
	fake.SetBatches(importer.Batch{Ref: "new", Name: "Renamed", Status: "ready", ABV: ptr(7.04), IBU: ptr(60.0), BrewDate: ptr("2026-10-31"),
		Notes: ptr("Other.")})
	b, adjusted, err := s.Resync(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "My IPA" || *b.Description != "Mine." || *b.ABV != 7.0 || *b.IBU != 60 || b.SRM != nil || *b.BrewDate != "2026-10-31" ||
		b.Status != "ready" || !b.Sync.SyncedAt.Equal(clock.t) || *b.Style != "American IPA" {
		t.Fatalf("re-synced beer = %+v", b)
	}
	if !slices.Equal(adjusted, []string{"abv"}) {
		t.Fatalf("adjusted = %v, want [abv]", adjusted)
	}

	// A missing batch marks the beer not found and changes nothing else.
	fake.SetBatches()
	clock.t = t0.Add(2 * time.Hour)
	nf, adjusted, err := s.Resync(ctx, id)
	if err != nil || !nf.Sync.NotFound || adjusted == nil || len(adjusted) != 0 {
		t.Fatalf("not found = %+v %v (err %v)", nf, adjusted, err)
	}
	nf.Sync.NotFound = false
	if before, after := mustJSON(t, b), mustJSON(t, nf); before != after {
		t.Fatalf("not found changed the beer:\n%s\n%s", before, after)
	}

	// Rate limited: the error, nothing written.
	fake.SetErr("GetBatch", &importer.RateLimitedError{Until: clock.t.Add(time.Minute)})
	var rl *importer.RateLimitedError
	if _, _, err := s.Resync(ctx, id); !errors.As(err, &rl) {
		t.Fatalf("rate limited = %v", err)
	}
	if got, _ := beers.Get(s.DB, id); !got.Sync.NotFound || !got.UpdatedAt.Equal(b.UpdatedAt) {
		t.Fatalf("rate limited re-sync wrote: %+v", got)
	}
	calls := fake.TotalCalls()
	if _, _, err := s.Resync(ctx, id); !errors.As(err, &rl) || fake.TotalCalls() != calls {
		t.Fatalf("closed gate = %v, calls %d -> %d", err, calls, fake.TotalCalls())
	}
}

func TestResyncArchive(t *testing.T) {
	s, fake, clock := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	fake.SetBatches(batchesFixture()...)
	free, kegged := importOne(t, s, "new"), importOne(t, s, "mid")
	addKeg(t, s, kegged)
	fake.SetBatches(importer.Batch{Ref: "new", Name: "x", Status: "ready", Archived: true}, importer.Batch{Ref: "mid", Name: "y", Status: "ready", Archived: true})
	clock.t = t0.Add(time.Hour)

	b, _, err := s.Resync(ctx, free)
	if err != nil || b.ArchivedAt == nil || !b.ArchivedAt.Equal(clock.t) || b.Sync.ArchivePending {
		t.Fatalf("archived batch, no keg = %+v (err %v)", b, err)
	}
	b, _, err = s.Resync(ctx, kegged)
	if err != nil || b.ArchivedAt != nil || !b.Sync.ArchivePending {
		t.Fatalf("archived batch, current keg = %+v (err %v)", b, err)
	}

	// Allowed on an archived beer.
	fake.SetBatches(importer.Batch{Ref: "new", Name: "x", Status: "ready", ABV: ptr(4.0)})
	b, _, err = s.Resync(ctx, free)
	if err != nil || b.ArchivedAt == nil || *b.ABV != 4.0 {
		t.Fatalf("re-sync of an archived beer = %+v (err %v)", b, err)
	}
}

func TestResyncRefused(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	manual, err := beers.Create(s.DB, beers.Input{Name: "Manual"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resync(ctx, manual.ID); !errors.Is(err, importer.ErrNotImported) {
		t.Fatalf("manual beer = %v", err)
	}
	if _, _, err := s.Resync(ctx, manual.ID+100); !errors.Is(err, beers.ErrNotFound) {
		t.Fatalf("unknown beer = %v", err)
	}
	if fake.TotalCalls() != 1 {
		t.Fatalf("refused re-syncs called the provider: %d calls", fake.TotalCalls())
	}
}
