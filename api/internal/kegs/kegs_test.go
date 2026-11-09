package kegs

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// set returns an Optional holding v.
func set[T any](v T) beers.Optional[T] { return beers.Optional[T]{Set: true, Value: &v} }

// null returns an Optional sent as null.
func null[T any]() beers.Optional[T] { return beers.Optional[T]{Set: true} }

// mustCreate creates a keg from in at t0, failing t on error.
func mustCreate(t *testing.T, db *sql.DB, in KegInput) Keg {
	t.Helper()
	k, err := CreateKeg(db, in, t0)
	if err != nil {
		t.Fatalf("CreateKeg %+v: %v", in, err)
	}
	return k
}

// newTap adds a tap labelled label and returns its id.
func newTap(t *testing.T, db *sql.DB, label string) int64 {
	t.Helper()
	tp, err := CreateTap(db, TapInput{Label: &label}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return tp.ID
}

func TestCreateKegDefaults(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", 7.0)
	k := mustCreate(t, db, KegInput{BeerID: beer})
	if k.ID == 0 || k.BeerID != beer || k.BeerName != "Pale" || k.BeerColor == nil || *k.BeerColor != beers.SRMColors[6] ||
		k.Type != "corny_5_gal" || k.SizeL != 18.93 || k.RemainingL != 18.93 || k.TapID != nil || k.TapLabel != nil ||
		k.State != "waiting" || k.TappedAt != nil || k.KickedAt != nil || !k.CreatedAt.Equal(t0) || !k.UpdatedAt.Equal(t0) {
		t.Fatalf("default keg = %+v", k)
	}
	if k := mustCreate(t, db, KegInput{BeerID: beer, Type: ptr("sanke_half")}); k.SizeL != 58.67 || k.RemainingL != 58.67 || k.Type != "sanke_half" {
		t.Fatalf("half barrel = %+v", k)
	}
	if k := mustCreate(t, db, KegInput{BeerID: beer, SizeL: ptr(19.999)}); k.SizeL != 20 || k.RemainingL != 20 {
		t.Fatalf("explicit size = %+v", k)
	}
	if k := mustCreate(t, db, KegInput{BeerID: beer, SizeL: ptr(30.0), RemainingL: ptr(12.345)}); k.SizeL != 30 || k.RemainingL != 12.35 {
		t.Fatalf("explicit remaining = %+v", k)
	}
	if _, err := CreateKeg(db, KegInput{BeerID: beer, SizeL: ptr(10.0), RemainingL: ptr(10.01)}, t0); fieldKeys(err) != "remaining_l" {
		t.Fatalf("remaining above size = %v", err)
	}
	if _, err := CreateKeg(db, KegInput{BeerID: beer, Type: ptr("keg"), SizeL: ptr(0.0)}, t0); fieldKeys(err) != "size_l,type" {
		t.Fatalf("bad type and size = %v", err)
	}
}

func TestCreateKegOnTap(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", nil)
	archived := insertBeer(t, db, "Old", nil)
	exec(t, db, "UPDATE beers SET archived_at = 1 WHERE id = ?", archived)
	tap := int64(1)

	k := mustCreate(t, db, KegInput{BeerID: beer, TapID: set(tap)})
	if k.State != "on_tap" || *k.TapID != tap || *k.TapLabel != "Tap 1" || !k.TappedAt.Equal(t0) || k.BeerColor != nil {
		t.Fatalf("keg on tap = %+v", k)
	}
	if _, err := CreateKeg(db, KegInput{BeerID: beer, TapID: set(tap)}, t0); !errors.Is(err, ErrTapOccupied) {
		t.Fatalf("second keg on tap = %v", err)
	}
	if _, err := CreateKeg(db, KegInput{BeerID: beer, TapID: set(int64(99))}, t0); fieldKeys(err) != "tap_id" {
		t.Fatalf("missing tap = %v", err)
	}
	for _, b := range []int64{0, 99, archived} {
		if _, err := CreateKeg(db, KegInput{BeerID: b}, t0); fieldKeys(err) != "beer_id" {
			t.Fatalf("beer %d = %v", b, err)
		}
	}
	if k := mustCreate(t, db, KegInput{BeerID: beer, TapID: null[int64]()}); k.State != "waiting" {
		t.Fatalf("tap_id null = %+v", k)
	}
	off := newTap(t, db, "Off")
	if _, err := UpdateTap(db, off, TapPatch{Enabled: set(false)}, t0); err != nil {
		t.Fatal(err)
	}
	if k := mustCreate(t, db, KegInput{BeerID: beer, TapID: set(off)}); k.State != "on_tap" {
		t.Fatalf("keg on disabled tap = %+v", k)
	}
}

func TestCreateKegRace(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", nil)
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { _, errs[i] = CreateKeg(db, KegInput{BeerID: beer, TapID: set(int64(1))}, t0) })
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, ErrTapOccupied):
			t.Errorf("racing create = %v, want ErrTapOccupied", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d racing creates succeeded, want 1", ok)
	}
}

// TestUniqueViolationIsTapOccupied checks the database's one-keg-per-tap index, the backstop
// behind the explicit check, answers ErrTapOccupied rather than an internal error.
func TestUniqueViolationIsTapOccupied(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", nil)
	insertKeg(t, db, beer, 1, nil)
	_, err := db.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (?, 1, 1, 1, 1, 1)", beer)
	if !errors.Is(occupied(err), ErrTapOccupied) {
		t.Fatalf("occupied(%v) is not ErrTapOccupied", err)
	}
	if err := occupied(errors.New("x")); errors.Is(err, ErrTapOccupied) {
		t.Fatal("occupied maps every error")
	}
}

func TestMoveKeg(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", nil)
	tap1 := int64(1)
	tap2 := newTap(t, db, "Right")
	k := mustCreate(t, db, KegInput{BeerID: beer})
	t1, t2 := t0.Add(time.Hour), t0.Add(2*time.Hour)

	k, err := UpdateKeg(db, k.ID, KegPatch{TapID: set(tap1)}, t1)
	if err != nil || k.State != "on_tap" || *k.TapID != tap1 || !k.TappedAt.Equal(t1) || *k.TapLabel != "Tap 1" || !k.UpdatedAt.Equal(t1) {
		t.Fatalf("on tap = %+v (err %v)", k, err)
	}
	if _, err := UpdateTap(db, tap1, TapPatch{Label: set("Left")}, t1); err != nil {
		t.Fatal(err)
	}
	if k, _ := GetKeg(db, k.ID); *k.TapLabel != "Left" {
		t.Fatalf("live tap label = %v", *k.TapLabel)
	}
	if ks, _ := ListKegs(db, KegFilter{}); len(ks) != 1 || *ks[0].TapLabel != "Left" {
		t.Fatalf("listed live tap label = %+v", ks)
	}
	k, err = UpdateKeg(db, k.ID, KegPatch{TapID: set(tap2)}, t2)
	if err != nil || *k.TapID != tap2 || !k.TappedAt.Equal(t1) || *k.TapLabel != "Right" {
		t.Fatalf("moved = %+v (err %v)", k, err)
	}
	k, err = UpdateKeg(db, k.ID, KegPatch{TapID: null[int64]()}, t2)
	if err != nil || k.State != "waiting" || k.TapID != nil || *k.TapLabel != "Right" || !k.TappedAt.Equal(t1) {
		t.Fatalf("taken off = %+v (err %v)", k, err)
	}
	if ts, _ := ListKegs(db, KegFilter{TapID: tap2}); len(ts) != 0 {
		t.Fatalf("tap filter matched a keg taken off: %+v", ts)
	}
	k, err = UpdateKeg(db, k.ID, KegPatch{TapID: null[int64]()}, t2)
	if err != nil || k.State != "waiting" || *k.TapLabel != "Right" {
		t.Fatalf("off again = %+v (err %v)", k, err)
	}

	other := mustCreate(t, db, KegInput{BeerID: beer, TapID: set(tap1)})
	if _, err := UpdateKeg(db, k.ID, KegPatch{TapID: set(tap1)}, t2); !errors.Is(err, ErrTapOccupied) {
		t.Fatalf("onto occupied tap = %v", err)
	}
	if _, err := UpdateKeg(db, other.ID, KegPatch{TapID: set(tap1)}, t2); err != nil {
		t.Fatalf("onto its own tap = %v", err)
	}
	if _, err := UpdateKeg(db, k.ID, KegPatch{TapID: set(int64(99))}, t2); fieldKeys(err) != "tap_id" {
		t.Fatalf("onto missing tap = %v", err)
	}
	if _, err := UpdateTap(db, tap2, TapPatch{Enabled: set(false)}, t2); err != nil {
		t.Fatal(err)
	}
	if k, err := UpdateKeg(db, k.ID, KegPatch{TapID: set(tap2)}, t2); err != nil || k.State != "on_tap" {
		t.Fatalf("onto disabled tap = %+v (err %v)", k, err)
	}
}

func TestCorrectKeg(t *testing.T) {
	db := openDB(t)
	pale := insertBeer(t, db, "Pale", nil)
	stout := insertBeer(t, db, "Stout", 40.0)
	k := mustCreate(t, db, KegInput{BeerID: pale})

	k, err := UpdateKeg(db, k.ID, KegPatch{Type: set("sanke_half")}, t0)
	if err != nil || k.Type != "sanke_half" || k.SizeL != 18.93 || k.RemainingL != 18.93 {
		t.Fatalf("type alone = %+v (err %v)", k, err)
	}
	k, err = UpdateKeg(db, k.ID, KegPatch{BeerID: set(stout), SizeL: set(10.0)}, t0)
	if err != nil || k.BeerName != "Stout" || *k.BeerColor != beers.SRMColors[39] || k.SizeL != 10 || k.RemainingL != 10 {
		t.Fatalf("beer and smaller size = %+v (err %v)", k, err)
	}
	k, err = UpdateKeg(db, k.ID, KegPatch{SizeL: set(20.0)}, t0)
	if err != nil || k.SizeL != 20 || k.RemainingL != 10 {
		t.Fatalf("bigger size = %+v (err %v)", k, err)
	}
	k, err = UpdateKeg(db, k.ID, KegPatch{SizeL: set(30.0), RemainingL: set(25.0)}, t0)
	if err != nil || k.SizeL != 30 || k.RemainingL != 25 {
		t.Fatalf("size and remaining = %+v (err %v)", k, err)
	}
	if _, err := UpdateKeg(db, k.ID, KegPatch{SizeL: set(5.0), RemainingL: set(6.0)}, t0); fieldKeys(err) != "remaining_l" {
		t.Fatalf("remaining above new size = %v", err)
	}
	exec(t, db, "UPDATE beers SET archived_at = 1 WHERE id = ?", pale)
	if _, err := UpdateKeg(db, k.ID, KegPatch{BeerID: set(pale)}, t0); fieldKeys(err) != "beer_id" {
		t.Fatalf("archived beer = %v", err)
	}
	bad := KegPatch{BeerID: null[int64](), Type: set("keg"), SizeL: set(0.0), RemainingL: null[float64](), Kicked: set(false)}
	if _, err := UpdateKeg(db, k.ID, bad, t0); fieldKeys(err) != "beer_id,kicked,remaining_l,size_l,type" {
		t.Fatalf("bad patch = %v", err)
	}
	if _, err := UpdateKeg(db, k.ID, KegPatch{Kicked: null[bool]()}, t0); fieldKeys(err) != "kicked" {
		t.Fatalf("kicked null = %v", err)
	}
	if _, err := UpdateKeg(db, k.ID, KegPatch{}, t0); !errors.Is(err, beers.ErrEmptyPatch) {
		t.Fatalf("empty patch = %v", err)
	}
	if _, err := UpdateKeg(db, 999, KegPatch{SizeL: set(1.0)}, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing keg = %v", err)
	}
	if k, _ := GetKeg(db, k.ID); k.SizeL != 30 || k.RemainingL != 25 || k.BeerName != "Stout" {
		t.Fatalf("refused patches changed the keg: %+v", k)
	}
}

func TestListKegsAndTaps(t *testing.T) {
	db := openDB(t)
	pale := insertBeer(t, db, "Pale", 4.0)
	stout := insertBeer(t, db, "Stout", nil)
	tap2 := newTap(t, db, "Tap 2")
	onTap := mustCreate(t, db, KegInput{BeerID: pale, TapID: set(int64(1))})
	waiting := mustCreate(t, db, KegInput{BeerID: stout})
	kicked := insertKeg(t, db, pale, tap2, t0.Unix())

	ids := func(ks []Keg, err error) []int64 {
		t.Helper()
		if err != nil || ks == nil {
			t.Fatalf("ListKegs = %v (err %v)", ks, err)
		}
		var out []int64
		for _, k := range ks {
			out = append(out, k.ID)
		}
		return out
	}
	eq := func(got []int64, want ...int64) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	if got := ids(ListKegs(db, KegFilter{})); !eq(got, onTap.ID, waiting.ID) {
		t.Fatalf("current = %v", got)
	}
	if got := ids(ListKegs(db, KegFilter{State: "current"})); !eq(got, onTap.ID, waiting.ID) {
		t.Fatalf("state current = %v", got)
	}
	if got := ids(ListKegs(db, KegFilter{State: "waiting"})); !eq(got, waiting.ID) {
		t.Fatalf("waiting = %v", got)
	}
	if got := ids(ListKegs(db, KegFilter{State: "kicked"})); !eq(got, kicked) {
		t.Fatalf("kicked = %v", got)
	}
	if got := ids(ListKegs(db, KegFilter{BeerID: stout})); !eq(got, waiting.ID) {
		t.Fatalf("beer filter = %v", got)
	}
	if got := ids(ListKegs(db, KegFilter{TapID: 1})); !eq(got, onTap.ID) {
		t.Fatalf("tap filter = %v", got)
	}
	if got := ids(ListKegs(db, KegFilter{BeerID: 999})); len(got) != 0 {
		t.Fatalf("no match = %v", got)
	}
	if _, err := ListKegs(db, KegFilter{State: "x"}); err == nil {
		t.Fatal("unknown state accepted")
	}

	ts, err := ListTaps(db)
	if err != nil || len(ts) != 2 || ts[0].Keg == nil || ts[0].Keg.ID != onTap.ID || ts[0].Keg.BeerName != "Pale" || *ts[0].Keg.BeerColor != beers.SRMColors[3] || ts[1].Keg != nil {
		t.Fatalf("taps = %+v (err %v)", ts, err)
	}
	if tp, err := GetTap(db, 1); err != nil || tp.Keg == nil || tp.Keg.ID != onTap.ID {
		t.Fatalf("GetTap = %+v (err %v)", tp, err)
	}

	if err := DeleteKeg(db, onTap.ID); err != nil {
		t.Fatal(err)
	}
	if err := DeleteKeg(db, onTap.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
	if _, err := GetKeg(db, onTap.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetKeg deleted = %v", err)
	}
	if tp, _ := GetTap(db, 1); tp.Keg != nil {
		t.Fatalf("tap after keg delete = %+v", tp)
	}
}

func TestPour(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", nil)
	k := mustCreate(t, db, KegInput{BeerID: beer, TapID: set(int64(1))})
	t1 := t0.Add(time.Hour)
	k, err := Pour(db, k.ID, 0.5, t1)
	if err != nil || k.RemainingL != 18.43 || !k.UpdatedAt.Equal(t1) || k.State != "on_tap" {
		t.Fatalf("pour 0.5 = %+v (err %v)", k, err)
	}
	for _, x := range []float64{0, -1, 0.004, 100.01} {
		if _, err := Pour(db, k.ID, x, t1); fieldKeys(err) != "amount_l" {
			t.Errorf("pour %v = %v", x, err)
		}
	}
	if _, err := Pour(db, 999, 0.5, t1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pour missing = %v", err)
	}
	low := mustCreate(t, db, KegInput{BeerID: beer, RemainingL: ptr(0.3)})
	low, err = Pour(db, low.ID, 0.5, t1)
	if err != nil || low.RemainingL != 0 || low.State != "waiting" || low.KickedAt != nil {
		t.Fatalf("pour past empty on a waiting keg = %+v (err %v)", low, err)
	}
}

func TestConcurrentPours(t *testing.T) {
	db := openDB(t)
	k := mustCreate(t, db, KegInput{BeerID: insertBeer(t, db, "Pale", nil)})
	var wg sync.WaitGroup
	errs := make([]error, 50)
	for i := range errs {
		wg.Go(func() { _, errs[i] = Pour(db, k.ID, 0.1, t0) })
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if k, _ := GetKeg(db, k.ID); k.RemainingL != 13.93 {
		t.Fatalf("after 50 pours of 0.1 L remaining = %v, want 13.93", k.RemainingL)
	}
}

func TestSetLevel(t *testing.T) {
	db := openDB(t)
	k := mustCreate(t, db, KegInput{BeerID: insertBeer(t, db, "Pale", nil)})
	k, err := UpdateKeg(db, k.ID, KegPatch{RemainingL: set(10.0)}, t0)
	if err != nil || k.RemainingL != 10 {
		t.Fatalf("set 10 = %+v (err %v)", k, err)
	}
	for _, x := range []float64{20, -1} {
		if _, err := UpdateKeg(db, k.ID, KegPatch{RemainingL: set(x)}, t0.Add(time.Hour)); fieldKeys(err) != "remaining_l" {
			t.Errorf("set %v = %v", x, err)
		}
	}
	if got, _ := GetKeg(db, k.ID); got.RemainingL != 10 || !got.UpdatedAt.Equal(t0) {
		t.Fatalf("refused level changed the keg: %+v", got)
	}
}

func TestKick(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", nil)
	tap := newTap(t, db, "Left")
	k := mustCreate(t, db, KegInput{BeerID: beer, TapID: set(tap)})
	k, _ = Pour(db, k.ID, 1, t0)
	t1, t2 := t0.Add(time.Hour), t0.Add(2*time.Hour)

	k, err := UpdateKeg(db, k.ID, KegPatch{Kicked: set(true)}, t1)
	if err != nil || k.State != "kicked" || !k.KickedAt.Equal(t1) || k.RemainingL != 17.93 || *k.TapID != tap || *k.TapLabel != "Left" {
		t.Fatalf("kick on tap = %+v (err %v)", k, err)
	}
	if tp, _ := GetTap(db, tap); tp.Keg != nil {
		t.Fatalf("tap after kick = %+v", tp.Keg)
	}
	next := mustCreate(t, db, KegInput{BeerID: beer, TapID: set(tap)})
	if again, err := UpdateKeg(db, k.ID, KegPatch{Kicked: set(true)}, t2); err != nil || !again.KickedAt.Equal(t1) || !again.UpdatedAt.Equal(t1) {
		t.Fatalf("repeat kick = %+v (err %v)", again, err)
	}
	for name, p := range map[string]KegPatch{
		"remaining":   {RemainingL: set(1.0)},
		"type":        {Type: set("sanke_half")},
		"beer":        {BeerID: set(beer)},
		"size":        {SizeL: set(10.0)},
		"tap off":     {TapID: null[int64]()},
		"tap on":      {TapID: set(int64(1))},
		"kick + size": {Kicked: set(true), SizeL: set(10.0)},
	} {
		if _, err := UpdateKeg(db, k.ID, p, t2); !errors.Is(err, ErrKegKicked) {
			t.Errorf("%s on a kicked keg = %v, want ErrKegKicked", name, err)
		}
	}
	if _, err := Pour(db, k.ID, 0.5, t2); !errors.Is(err, ErrKegKicked) {
		t.Fatalf("pour on a kicked keg = %v", err)
	}

	waiting := mustCreate(t, db, KegInput{BeerID: beer})
	waiting, err = UpdateKeg(db, waiting.ID, KegPatch{Kicked: set(true)}, t2)
	if err != nil || waiting.State != "kicked" || waiting.TapID != nil || waiting.TapLabel != nil || waiting.TappedAt != nil {
		t.Fatalf("kick waiting = %+v (err %v)", waiting, err)
	}

	history, err := ListKegs(db, KegFilter{State: "kicked"})
	if err != nil || len(history) != 2 || history[0].ID != waiting.ID || history[1].ID != k.ID {
		t.Fatalf("history = %+v (err %v)", history, err)
	}
	if ks, _ := ListKegs(db, KegFilter{State: "kicked", TapID: tap}); len(ks) != 1 || ks[0].ID != k.ID {
		t.Fatalf("history on tap = %+v", ks)
	}
	if _, err := UpdateTap(db, tap, TapPatch{Label: set("Renamed")}, t2); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetKeg(db, k.ID); *got.TapLabel != "Left" {
		t.Fatalf("history label after rename = %q", *got.TapLabel)
	}
	if err := DeleteKeg(db, next.ID); err != nil {
		t.Fatal(err)
	}
	if err := DeleteTap(db, tap); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetKeg(db, k.ID); *got.TapLabel != "Left" || got.TapID != nil {
		t.Fatalf("history after tap delete = %+v", got)
	}
	if ks, _ := ListKegs(db, KegFilter{State: "kicked", TapID: tap}); len(ks) != 0 {
		t.Fatalf("history on removed tap = %+v", ks)
	}
	if err := DeleteKeg(db, k.ID); err != nil {
		t.Fatalf("delete kicked keg = %v", err)
	}
}

// TestScale lists 20 taps and 1,000 kicked kegs within SC-005's 1 s budget.
func TestScale(t *testing.T) {
	db := openDB(t)
	beer := insertBeer(t, db, "Pale", 5.0)
	for i := range 19 {
		newTap(t, db, fmt.Sprintf("Tap %d", i+2))
	}
	exec(t, db, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < 1000)
		INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, tap_label, tapped_at, kicked_at, created_at, updated_at)
		SELECT ?, 18.93, 1, (i % 20) + 1, 'Tap', i, i + 1, i, i FROM n`, beer)
	for i := range 20 {
		mustCreate(t, db, KegInput{BeerID: beer, TapID: set(int64(i + 1))})
	}
	// SC-005's budget is 1 s on the home server; a tighter bound flakes on slow or -race CI.
	start := time.Now()
	ts, err := ListTaps(db)
	if d := time.Since(start); err != nil || len(ts) != 20 || ts[19].Keg == nil || d > time.Second {
		t.Fatalf("ListTaps = %d taps in %v (err %v)", len(ts), d, err)
	}
	start = time.Now()
	ks, err := ListKegs(db, KegFilter{State: "kicked"})
	if d := time.Since(start); err != nil || len(ks) != 1000 || d > time.Second {
		t.Fatalf("ListKegs kicked = %d kegs in %v (err %v)", len(ks), d, err)
	}
}
