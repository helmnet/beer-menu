package importer_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
	"github.com/helmnet/beer-menu/api/internal/importer"
	"github.com/helmnet/beer-menu/api/internal/importer/importertest"
)

// hooked is a Provider that runs onGet before each GetBatch, to change the database mid-run.
type hooked struct {
	*importertest.Fake
	onGet func(id string)
}

func (h hooked) GetBatch(ctx context.Context, id string) (importer.Batch, error) {
	h.onGet(id)
	return h.Fake.GetBatch(ctx, id)
}

// lastRun returns the status's last sync, failing t if there is none.
func lastRun(t *testing.T, s *importer.Service) importer.LastSync {
	t.Helper()
	st, err := s.Status(context.Background())
	if err != nil || st.LastSync == nil {
		t.Fatalf("status = %+v (err %v)", st, err)
	}
	return *st.LastSync
}

// mustBeer returns beer id.
func mustBeer(t *testing.T, s *importer.Service, id int64) beers.Beer {
	t.Helper()
	b, err := beers.Get(s.DB, id)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// sourceRefs returns the source_ref of every beer imported from Brewfather, archived or not,
// sorted.
func sourceRefs(t *testing.T, s *importer.Service) string {
	t.Helper()
	ids, err := beers.ImportedIDs(s.DB, "brewfather")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for ref := range ids {
		out = append(out, ref)
	}
	slices.Sort(out)
	return strings.Join(out, ",")
}

func TestSync(t *testing.T) {
	s, fake, clock := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	fake.SetBatches(
		importer.Batch{Ref: "kept", Name: "Kept", Status: "fermenting", ABV: ptr(5.0)},
		importer.Batch{Ref: "arch", Name: "Arch", Status: "ready"},
		importer.Batch{Ref: "gone", Name: "Gone", Status: "ready"},
		importer.Batch{Ref: "lost", Name: "Lost", Status: "ready"},
		importer.Batch{Ref: "local", Name: "Local", Status: "ready", ABV: ptr(4.0)},
	)
	kept, arch, gone, lost, local := importOne(t, s, "kept"), importOne(t, s, "arch"), importOne(t, s, "gone"), importOne(t, s, "lost"), importOne(t, s, "local")
	if _, err := beers.MarkNotFound(s.DB, lost); err != nil {
		t.Fatal(err)
	}
	if _, err := beers.Update(s.DB, local, beers.Patch{Archived: set(true)}, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec("INSERT INTO import_dismissed (source, ref, dismissed_at) VALUES ('brewfather', 'dismissed', 1)"); err != nil {
		t.Fatal(err)
	}
	fake.SetBatches(
		importer.Batch{Ref: "kept", Name: "Renamed", Status: "conditioning", ABV: ptr(5.5)},
		importer.Batch{Ref: "arch", Name: "Arch", Status: "ready", Archived: true},
		importer.Batch{Ref: "local", Name: "Local", Status: "ready", ABV: ptr(9.0)},
		importer.Batch{Ref: "new-f", Name: "New F", Status: "fermenting", ABV: ptr(6.04)},
		importer.Batch{Ref: "new-c", Name: "New C", Status: "conditioning"},
		importer.Batch{Ref: "new-p", Name: "New P", Status: "planned"},
		importer.Batch{Ref: "new-r", Name: "New R", Status: "ready"},
		importer.Batch{Ref: "dismissed", Name: "Dismissed", Status: "fermenting"},
	)
	gets := fake.Calls("GetBatch")
	clock.t = t0.Add(time.Hour)

	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if fake.Calls("ListBatches") != 1 || fake.Calls("GetBatch")-gets != 2 {
		t.Fatalf("calls: list %d, get %d; want 1, 2 (arch and gone only)", fake.Calls("ListBatches"), fake.Calls("GetBatch")-gets)
	}
	if b := mustBeer(t, s, kept); b.Name != "Kept" || *b.ABV != 5.5 || b.Status != "conditioning" || !b.Sync.SyncedAt.Equal(clock.t) {
		t.Fatalf("listed beer = %+v", b)
	}
	if b := mustBeer(t, s, arch); b.ArchivedAt == nil || !b.ArchivedAt.Equal(clock.t) {
		t.Fatalf("archived batch's beer = %+v", b)
	}
	if b := mustBeer(t, s, gone); !b.Sync.NotFound || !b.UpdatedAt.Equal(t0) {
		t.Fatalf("missing batch's beer = %+v", b)
	}
	if b := mustBeer(t, s, lost); !b.Sync.NotFound {
		t.Fatalf("already missing beer = %+v", b)
	}
	if b := mustBeer(t, s, local); *b.ABV != 4.0 || !b.Sync.SyncedAt.Equal(t0) {
		t.Fatalf("locally archived beer was synced: %+v", b)
	}
	if got := sourceRefs(t, s); got != "arch,gone,kept,local,lost,new-c,new-f" {
		t.Fatalf("imported refs = %s", got)
	}
	ids, _ := beers.ImportedIDs(s.DB, "brewfather")
	if b := mustBeer(t, s, ids["new-f"]); b.Name != "New F" || *b.ABV != 6.0 || b.Status != "fermenting" || !b.Sync.SyncedAt.Equal(clock.t) {
		t.Fatalf("auto-imported beer = %+v", b)
	}
	run := lastRun(t, s)
	if !run.StartedAt.Equal(clock.t) || !run.FinishedAt.Equal(clock.t) || !*run.OK || run.Updated != 2 || run.Imported != 2 || run.NotFound != 2 ||
		*run.Message != "Sync finished." {
		t.Fatalf("run = %+v (message %q)", run, *run.Message)
	}

	// A beer already not found costs no call; its flag clears once its batch is listed again.
	fake.SetBatches(importer.Batch{Ref: "lost", Name: "Lost", Status: "ready"}, importer.Batch{Ref: "kept", Name: "Kept", Status: "ready"})
	gets = fake.Calls("GetBatch")
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	// new-f and new-c went missing; gone stays not found with no call.
	if n := fake.Calls("GetBatch") - gets; n != 2 {
		t.Fatalf("GetBatch calls = %d, want 2", n)
	}
	if b := mustBeer(t, s, lost); b.Sync.NotFound {
		t.Fatalf("listed again but still not found: %+v", b)
	}
	if run := lastRun(t, s); run.Updated != 2 || run.Imported != 0 || run.NotFound != 3 {
		t.Fatalf("second run = %+v", run)
	}
}

func TestSyncFailures(t *testing.T) {
	s, fake, _ := newService(t)
	ctx := context.Background()
	mustConnect(t, s)
	fake.SetBatches(importer.Batch{Ref: "a", Name: "Alpha", Status: "ready"}, importer.Batch{Ref: "b", Name: "Bravo", Status: "ready"},
		importer.Batch{Ref: "c", Name: "Charlie", Status: "ready"})
	a, b, c := importOne(t, s, "a"), importOne(t, s, "b"), importOne(t, s, "c")
	before := mustJSON(t, []beers.Beer{mustBeer(t, s, a), mustBeer(t, s, b), mustBeer(t, s, c)})

	// A list failure records a plain failed run and changes nothing.
	fake.SetErr("ListBatches", importer.ErrUnreachable)
	if err := s.Sync(ctx); !errors.Is(err, importer.ErrUnreachable) {
		t.Fatalf("list failure = %v", err)
	}
	if run := lastRun(t, s); *run.OK || *run.Message != "Brewfather could not be reached." || run.FinishedAt == nil {
		t.Fatalf("failed run = %+v", run)
	}
	if after := mustJSON(t, []beers.Beer{mustBeer(t, s, a), mustBeer(t, s, b), mustBeer(t, s, c)}); after != before {
		t.Fatalf("a failed list changed beers:\n%s\n%s", before, after)
	}
	fake.SetErr("ListBatches", nil)

	// One beer failing is named, the others still sync, and a beer deleted mid-run is skipped.
	fake.SetBatches(importer.Batch{Ref: "b", Name: "Bravo", Status: "fermenting"})
	fake.SetErr("GetBatch:a", errors.New("unreadable batch"))
	s.Factory = func(string, string) importer.Provider {
		return hooked{Fake: fake, onGet: func(string) {
			if _, err := s.DB.Exec("DELETE FROM beers WHERE id = ?", c); err != nil {
				t.Error(err)
			}
		}}
	}
	if err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	run := lastRun(t, s)
	if *run.OK || *run.Message != "Could not sync 1 beer: Alpha." || run.Updated != 1 {
		t.Fatalf("partial run = %+v (message %q)", run, *run.Message)
	}
	if got := mustBeer(t, s, b); got.Status != "fermenting" {
		t.Fatalf("other beer not synced: %+v", got)
	}
}

func TestSyncMessageLimit(t *testing.T) {
	s, fake, _ := newService(t)
	mustConnect(t, s)
	var bs []importer.Batch
	for i := range 30 {
		bs = append(bs, importer.Batch{Ref: fmt.Sprint(i), Name: strings.Repeat("n", 40) + fmt.Sprint(i), Status: "ready"})
	}
	fake.SetBatches(bs...)
	for i := range 30 {
		importOne(t, s, fmt.Sprint(i))
	}
	fake.SetBatches()
	fake.SetErr("GetBatch", importer.ErrUnreachable)
	gets := fake.Calls("GetBatch")
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	// An unreachable provider stops the per-beer calls; the rest fail without one.
	if n := fake.Calls("GetBatch") - gets; n != 1 {
		t.Fatalf("GetBatch calls = %d, want 1", n)
	}
	if run := lastRun(t, s); *run.OK || len([]rune(*run.Message)) != 500 || !strings.HasPrefix(*run.Message, "Could not sync 30 beers: ") {
		t.Fatalf("message = %q", *run.Message)
	}
}

func TestSyncRunning(t *testing.T) {
	s, fake, _ := newService(t)
	mustConnect(t, s)
	fake.SetBlock(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.StartSync(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Status(context.Background()); !st.SyncRunning || st.LastSync == nil || st.LastSync.FinishedAt != nil {
		t.Fatalf("status while running = %+v", st)
	}
	if err := s.Sync(context.Background()); !errors.Is(err, importer.ErrSyncRunning) {
		t.Fatalf("second sync = %v", err)
	}
	if err := s.StartSync(context.Background()); !errors.Is(err, importer.ErrSyncRunning) {
		t.Fatalf("second background sync = %v", err)
	}
	cancel()
	s.Wait()
	st, _ := s.Status(context.Background())
	if st.SyncRunning || st.LastSync.FinishedAt == nil || *st.LastSync.OK {
		t.Fatalf("status after a cancelled sync = %+v", st)
	}
	fake.SetBlock(false)
	if err := s.Sync(context.Background()); err != nil {
		t.Fatalf("sync after the lock is released = %v", err)
	}
}

func TestSyncRefused(t *testing.T) {
	s, fake, clock := newService(t)
	ctx := context.Background()
	if err := s.Sync(ctx); !errors.Is(err, importer.ErrNotConnected) {
		t.Fatalf("not connected = %v", err)
	}
	mustConnect(t, s)
	fake.SetErr("ListBatches", &importer.RateLimitedError{Until: clock.t.Add(time.Minute)})
	if _, err := s.Batches(ctx, true); err == nil {
		t.Fatal("gate did not close")
	}
	calls := fake.TotalCalls()
	var rl *importer.RateLimitedError
	if err := s.StartSync(ctx); !errors.As(err, &rl) || fake.TotalCalls() != calls {
		t.Fatalf("closed gate = %v, calls %d -> %d", err, calls, fake.TotalCalls())
	}
	if st, _ := s.Status(ctx); st.LastSync != nil || st.SyncRunning {
		t.Fatalf("refused sync left a run: %+v", st)
	}
}

// SC-005: 120 active batches across the 5 listed statuses, every imported beer listed, cost
// one list and no per-beer call.
func TestSyncCallBudget(t *testing.T) {
	s, fake, _ := newService(t)
	mustConnect(t, s)
	statuses := []string{"planned", "fermenting", "fermenting", "conditioning", "ready"}
	var bs []importer.Batch
	for i := range 120 {
		ref := fmt.Sprint("b", i)
		b := importer.Batch{Ref: ref, Name: ref, Status: statuses[i%5]}
		bs = append(bs, b)
		if i%2 == 0 {
			v, _ := beers.Sanitize(beers.Input{Name: b.Name, Status: &b.Status})
			if _, err := beers.CreateImported(s.DB, "brewfather", ref, v, false, t0); err != nil {
				t.Fatal(err)
			}
		}
	}
	fake.SetBatches(bs...)
	calls := fake.TotalCalls()
	if err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := fake.TotalCalls() - calls; n > 7 {
		t.Fatalf("provider calls = %d, want at most 7", n)
	}
	if run := lastRun(t, s); !*run.OK || run.Updated != 60 || run.Imported != 36 {
		t.Fatalf("run = %+v", run)
	}
}
