package kegs

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/beers"
)

// tapLabels returns the taps as "position:label" in list order.
func tapLabels(ts []Tap) string {
	s := ""
	for _, tp := range ts {
		s += fmt.Sprintf("%d:%s ", tp.Position, tp.Label)
	}
	return s
}

func TestListAndCreateTaps(t *testing.T) {
	db := openDB(t)
	ts, err := ListTaps(db)
	if err != nil || len(ts) != 1 || ts[0].Label != "Tap 1" || ts[0].Position != 1 || !ts[0].Enabled || ts[0].Keg != nil {
		t.Fatalf("seeded taps = %+v (err %v)", ts, err)
	}
	tp, err := CreateTap(db, TapInput{}, t0)
	if err != nil || tp.Position != 2 || tp.Label != "Tap 2" || !tp.Enabled || tp.Keg != nil || !tp.CreatedAt.Equal(t0) || !tp.UpdatedAt.Equal(t0) {
		t.Fatalf("CreateTap {} = %+v (err %v)", tp, err)
	}
	tp, err = CreateTap(db, TapInput{Label: ptr("  Nitro "), Enabled: ptr(false)}, t0)
	if err != nil || tp.Position != 3 || tp.Label != "Nitro" || tp.Enabled {
		t.Fatalf("CreateTap Nitro = %+v (err %v)", tp, err)
	}
	if _, err := CreateTap(db, TapInput{Label: ptr(" ")}, t0); fieldKeys(err) != "label" {
		t.Fatalf("blank label err = %v", err)
	}
	ts, err = ListTaps(db)
	if err != nil || tapLabels(ts) != "1:Tap 1 2:Tap 2 3:Nitro " {
		t.Fatalf("taps = %s (err %v)", tapLabels(ts), err)
	}
	got, err := GetTap(db, tp.ID)
	if err != nil || got.Label != "Nitro" {
		t.Fatalf("GetTap = %+v (err %v)", got, err)
	}
	if _, err := GetTap(db, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetTap missing = %v", err)
	}
}

// fieldKeys returns the sorted FieldErrors keys of err joined by commas, or err's text.
func fieldKeys(err error) string {
	var fe beers.FieldErrors
	if !errors.As(err, &fe) {
		return fmt.Sprint(err)
	}
	return keys(fe)
}

func TestUpdateTap(t *testing.T) {
	db := openDB(t)
	tap := int64(1)
	beer := insertBeer(t, db, "Pale", nil)
	keg := insertKeg(t, db, beer, tap, nil)
	t1 := t0.Add(time.Hour)

	tp, err := UpdateTap(db, tap, TapPatch{Label: beers.Optional[string]{Set: true, Value: ptr(" Left ")}}, t1)
	if err != nil || tp.Label != "Left" || !tp.Enabled || !tp.UpdatedAt.Equal(t1) {
		t.Fatalf("rename = %+v (err %v)", tp, err)
	}
	tp, err = UpdateTap(db, tap, TapPatch{Enabled: beers.Optional[bool]{Set: true, Value: ptr(false)}}, t1)
	if err != nil || tp.Label != "Left" || tp.Enabled {
		t.Fatalf("disable = %+v (err %v)", tp, err)
	}
	var kegTap int64
	var kicked *int64
	if err := db.QueryRow("SELECT tap_id, kicked_at FROM kegs WHERE id = ?", keg).Scan(&kegTap, &kicked); err != nil || kegTap != tap || kicked != nil {
		t.Fatalf("keg after disable: tap %d kicked %v (err %v)", kegTap, kicked, err)
	}
	if _, err := UpdateTap(db, 999, TapPatch{Enabled: beers.Optional[bool]{Set: true, Value: ptr(true)}}, t1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing tap = %v", err)
	}
	if _, err := UpdateTap(db, tap, TapPatch{}, t1); !errors.Is(err, beers.ErrEmptyPatch) {
		t.Fatalf("empty patch = %v", err)
	}
	if _, err := UpdateTap(db, tap, TapPatch{Label: beers.Optional[string]{Set: true}, Enabled: beers.Optional[bool]{Set: true}}, t1); fieldKeys(err) != "enabled,label" {
		t.Fatalf("null fields = %v", err)
	}
}

func TestReorderTaps(t *testing.T) {
	db := openDB(t)
	a := int64(1)
	tp, _ := CreateTap(db, TapInput{}, t0)
	b := tp.ID
	tp, _ = CreateTap(db, TapInput{}, t0)
	c := tp.ID
	ts, err := ReorderTaps(db, []int64{c, a, b}, t0.Add(time.Hour))
	if err != nil || tapLabels(ts) != "1:Tap 3 2:Tap 1 3:Tap 2 " {
		t.Fatalf("reorder = %s (err %v)", tapLabels(ts), err)
	}
	for _, bad := range [][]int64{nil, {c, a}, {c, a, a}, {c, a, b, 99}, {c, a, 99}} {
		if _, err := ReorderTaps(db, bad, t0); fieldKeys(err) != "ids" {
			t.Errorf("reorder %v = %v, want ids error", bad, err)
		}
	}
	if ts, _ := ListTaps(db); tapLabels(ts) != "1:Tap 3 2:Tap 1 3:Tap 2 " {
		t.Fatalf("bad reorder changed order: %s", tapLabels(ts))
	}
}

func TestDeleteTap(t *testing.T) {
	db := openDB(t)
	if err := DeleteTap(db, 1); !errors.Is(err, ErrLastTap) {
		t.Fatalf("delete only tap = %v", err)
	}
	t2, _ := CreateTap(db, TapInput{}, t0)
	t3, _ := CreateTap(db, TapInput{}, t0)
	beer := insertBeer(t, db, "Pale", nil)
	insertKeg(t, db, beer, t3.ID, nil)
	if err := DeleteTap(db, t3.ID); !errors.Is(err, ErrTapHasKeg) {
		t.Fatalf("delete tap with keg = %v", err)
	}
	if err := DeleteTap(db, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing = %v", err)
	}
	kicked := insertKeg(t, db, beer, t2.ID, t0.Unix())
	exec(t, db, "UPDATE kegs SET tap_label = 'Tap 2' WHERE id = ?", kicked)
	if err := DeleteTap(db, t2.ID); err != nil {
		t.Fatalf("delete tap with only a kicked keg = %v", err)
	}
	if ts, _ := ListTaps(db); tapLabels(ts) != "1:Tap 1 2:Tap 3 " {
		t.Fatalf("after delete = %s", tapLabels(ts))
	}
	var label string
	var tap *int64
	if err := db.QueryRow("SELECT tap_label, tap_id FROM kegs WHERE id = ?", kicked).Scan(&label, &tap); err != nil || label != "Tap 2" || tap != nil {
		t.Fatalf("kicked keg after tap delete: label %q tap %v (err %v)", label, tap, err)
	}
}

func TestTapPositionsStayContiguous(t *testing.T) {
	db := openDB(t)
	rng := rand.New(rand.NewPCG(1, 2))
	for i := range 20 {
		ts, err := ListTaps(db)
		if err != nil {
			t.Fatal(err)
		}
		switch rng.IntN(3) {
		case 0:
			_, err = CreateTap(db, TapInput{}, t0)
		case 1:
			err = DeleteTap(db, ts[rng.IntN(len(ts))].ID)
			if errors.Is(err, ErrLastTap) {
				err = nil
			}
		case 2:
			ids := make([]int64, len(ts))
			for j, tp := range ts {
				ids[j] = tp.ID
			}
			rng.Shuffle(len(ids), func(a, b int) { ids[a], ids[b] = ids[b], ids[a] })
			_, err = ReorderTaps(db, ids, t0)
		}
		if err != nil {
			t.Fatalf("op %d: %v", i, err)
		}
		ts, _ = ListTaps(db)
		var pos []int64
		for _, tp := range ts {
			pos = append(pos, tp.Position)
		}
		for j, p := range pos {
			if p != int64(j+1) {
				t.Fatalf("op %d: positions %v", i, pos)
			}
		}
	}
}
