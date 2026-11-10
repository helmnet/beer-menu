package brewfather

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/importer"
)

// bodyMarker is in every error body the fake server sends, to prove bodies never reach errors.
const bodyMarker = "BODY-MARKER-do-not-leak"

// fakeBF is a TLS Brewfather stand-in. Batches are listed by status in _id order, paged with
// limit and start_after as the real API does. handle, when set, answers instead.
type fakeBF struct {
	t        *testing.T
	srv      *httptest.Server
	user     string
	key      string
	batches  map[string][]map[string]any
	handle   func(w http.ResponseWriter, r *http.Request)
	delay    time.Duration
	mu       sync.Mutex
	requests []*http.Request
	inFlight int
	maxIn    int
	badAuth  int
}

func newFake(t *testing.T) *fakeBF {
	t.Helper()
	f := &fakeBF{t: t, user: "fake-user-" + rand.Text(), key: "fake-key-" + rand.Text(), batches: map[string][]map[string]any{}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeBF) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.inFlight++
	f.maxIn = max(f.maxIn, f.inFlight)
	if u, k, ok := r.BasicAuth(); !ok || u != f.user || k != f.key {
		f.badAuth++
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inFlight--
		f.mu.Unlock()
	}()
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-r.Context().Done():
			return
		}
	}
	if f.handle != nil {
		f.handle(w, r)
		return
	}
	if r.URL.Path != "/v2/batches" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	list := f.batches[q.Get("status")]
	start := 0
	if after := q.Get("start_after"); after != "" {
		start = slices.IndexFunc(list, func(b map[string]any) bool { return b["_id"] == after }) + 1
	}
	end := min(start+limit, len(list))
	json.NewEncoder(w).Encode(list[start:end])
}

// provider returns a client for f with timeout and the fake's credentials.
func (f *fakeBF) provider(t *testing.T, timeout time.Duration) importer.Provider {
	t.Helper()
	hc := f.srv.Client()
	hc.Timeout = timeout
	p, err := NewWith(f.srv.URL+"/v2", hc, f.user, f.key)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// noLeak fails if err's text holds a credential, the Basic header or a response body.
func (f *fakeBF) noLeak(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	basic := base64.StdEncoding.EncodeToString([]byte(f.user + ":" + f.key))
	for _, s := range []string{f.user, f.key, basic, bodyMarker} {
		if strings.Contains(err.Error(), s) {
			t.Fatalf("error %q leaks %q", err, s)
		}
	}
}

// batches returns n minimal batches with ids prefix-000 upward.
func batches(prefix, status string, n int) []map[string]any {
	out := make([]map[string]any, n)
	for i := range out {
		out[i] = map[string]any{"_id": fmt.Sprintf("%s-%03d", prefix, i), "name": fmt.Sprintf("%s %d", prefix, i), "status": status}
	}
	return out
}

func refs(bs []importer.Batch) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Ref
	}
	slices.Sort(out)
	return out
}

func TestListBatches(t *testing.T) {
	f := newFake(t)
	f.batches["Planning"] = batches("plan", "Planning", 1)
	f.batches["Fermenting"] = batches("ferm", "Fermenting", 50)
	f.batches["Completed"] = batches("done", "Completed", 120)
	f.batches["Archived"] = batches("arch", "Archived", 3)
	got, err := f.provider(t, 5*time.Second).ListBatches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, s := range []string{"Planning", "Fermenting", "Completed"} {
		for _, b := range f.batches[s] {
			want = append(want, b["_id"].(string))
		}
	}
	slices.Sort(want)
	if !reflect.DeepEqual(refs(got), want) {
		t.Fatalf("got %d batches, want %d", len(got), len(want))
	}

	calls := map[string][]string{}
	for _, r := range f.requests {
		if r.URL.Path != "/v2/batches" {
			t.Errorf("request to %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("limit") != "50" {
			t.Errorf("limit = %q, want 50", q.Get("limit"))
		}
		if q.Get("complete") != "" {
			t.Errorf("complete = %q, want unset", q.Get("complete"))
		}
		inc := strings.Split(q.Get("include"), ",")
		for _, field := range []string{"recipe.style.name", "recipe.abv", "measuredAbv", "estimatedIbu", "estimatedColor", "batchNotes", "tasteNotes"} {
			if !slices.Contains(inc, field) {
				t.Errorf("include %v lacks %s", inc, field)
			}
		}
		calls[q.Get("status")] = append(calls[q.Get("status")], q.Get("start_after"))
	}
	if f.badAuth != 0 {
		t.Fatalf("%d requests without the right Basic auth", f.badAuth)
	}
	wantCalls := map[string][]string{
		"Planning":     {""},
		"Brewing":      {""},
		"Fermenting":   {"", "ferm-049"},
		"Conditioning": {""},
		"Completed":    {"", "done-049", "done-099"},
	}
	for s := range calls {
		slices.Sort(calls[s])
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls by status = %v, want %v", calls, wantCalls)
	}
}

func TestListBatchesConcurrency(t *testing.T) {
	f := newFake(t)
	f.delay = 30 * time.Millisecond
	for _, s := range activeStatuses {
		f.batches[s] = batches(s, s, 101)
	}
	got, err := f.provider(t, 5*time.Second).ListBatches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5*101 {
		t.Fatalf("got %d batches, want %d", len(got), 5*101)
	}
	if f.maxIn > 5 || f.maxIn < 2 {
		t.Fatalf("max in flight = %d, want 2 to 5", f.maxIn)
	}
	if len(f.requests) != 15 {
		t.Fatalf("requests = %d, want 15 (3 pages for each of 5 statuses)", len(f.requests))
	}
}

// TestListBatchesPageCap: a server that always sends a full page of new IDs stops after
// maxPages pages per status instead of looping forever.
func TestListBatchesPageCap(t *testing.T) {
	f := newFake(t)
	var n atomic.Int64
	limit := int64(len(activeStatuses) * maxPages)
	f.handle = func(w http.ResponseWriter, r *http.Request) {
		i := n.Add(1)
		if i > limit { // a short page ends the listing, so a regressed cap fails here instead of hanging
			fmt.Fprint(w, "[]")
			return
		}
		json.NewEncoder(w).Encode(batches(fmt.Sprintf("p%d", i), r.URL.Query().Get("status"), pageSize))
	}
	_, err := f.provider(t, 5*time.Second).ListBatches(context.Background())
	if !errors.Is(err, importer.ErrUnreachable) {
		t.Fatalf("ListBatches = %v, want ErrUnreachable", err)
	}
	if got := n.Load(); got > limit {
		t.Fatalf("requests = %d, want at most %d", got, limit)
	}
}

func TestCheck(t *testing.T) {
	f := newFake(t)
	f.batches["Completed"] = batches("done", "Completed", 3)
	if err := f.provider(t, 5*time.Second).Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 || f.requests[0].URL.Query().Get("limit") != "1" || f.requests[0].URL.Path != "/v2/batches" || f.badAuth != 0 {
		t.Fatalf("Check sent %d requests: %v", len(f.requests), f.requests[0].URL)
	}
}

func TestGetBatchPath(t *testing.T) {
	f := newFake(t)
	var path string
	f.handle = func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		fmt.Fprint(w, `{"_id":"a/b c?d","name":"X","status":"Completed"}`)
	}
	b, err := f.provider(t, 5*time.Second).GetBatch(context.Background(), "a/b c?d")
	if err != nil || b.Ref != "a/b c?d" {
		t.Fatalf("GetBatch = %+v (err %v)", b, err)
	}
	if path != "/v2/batches/a%2Fb%20c%3Fd" {
		t.Fatalf("path = %q", path)
	}
	for _, id := range []string{"", ".", ".."} {
		if _, err := f.provider(t, time.Second).GetBatch(context.Background(), id); !errors.Is(err, importer.ErrBatchNotFound) {
			t.Fatalf("GetBatch(%q) = %v, want ErrBatchNotFound", id, err)
		}
	}
	if len(f.requests) != 1 {
		t.Fatalf("requests = %d, want 1 (empty and dot ids never sent)", len(f.requests))
	}
}

func TestMapping(t *testing.T) {
	ms := func(y int, m time.Month, d, h int) int64 {
		return time.Date(y, m, d, h, 30, 0, 0, time.UTC).UnixMilli()
	}
	str := func(s string) *string { return &s }
	num := func(x float64) *float64 { return &x }
	n := func(x int64) *int64 { return &x }
	cases := []struct {
		name string
		json string
		want importer.Batch
	}{
		{"full",
			fmt.Sprintf(`{"_id":"b1","name":"Pale","batchNo":12,"status":"Fermenting","brewDate":%d,"measuredAbv":5.4,"estimatedIbu":35.2,
				"estimatedColor":6.1,"batchNotes":"batch","tasteNotes":"taste","recipe":{"name":"R","abv":5.9,"style":{"name":"APA"}}}`, ms(2026, 10, 1, 23)),
			importer.Batch{Ref: "b1", Name: "Pale", Number: n(12), Status: "fermenting", Style: str("APA"), Notes: str("taste"),
				BrewDate: str("2026-10-01"), ABV: num(5.4), IBU: num(35.2), SRM: num(6.1)}},
		{"fallbacks",
			`{"_id":"b2","name":"  ","batchNo":7,"status":"Planning","brewDate":0,"measuredAbv":null,"batchNotes":"batch","tasteNotes":" ",
				"recipe":{"name":"Recipe name","abv":6.2}}`,
			importer.Batch{Ref: "b2", Name: "Recipe name", Number: n(7), Status: "planned", Notes: str("batch"), ABV: num(6.2)}},
		{"batch number name",
			`{"_id":"b3","batchNo":9,"status":"Brewing","recipe":{"name":""}}`,
			importer.Batch{Ref: "b3", Name: "Batch 9", Number: n(9), Status: "fermenting"}},
		{"default name uses recipe", `{"_id":"d","name":"Batch","recipe":{"name":"Hazy IPA"}}`,
			importer.Batch{Ref: "d", Name: "Hazy IPA", Status: "planned"}},
		{"default name any case and spaces", `{"_id":"d","name":" batch ","recipe":{"name":"Hazy IPA"}}`,
			importer.Batch{Ref: "d", Name: "Hazy IPA", Status: "planned"}},
		{"default name no recipe name", `{"_id":"d","name":"Batch","batchNo":124}`,
			importer.Batch{Ref: "d", Name: "Batch 124", Number: n(124), Status: "planned"}},
		{"custom name kept", `{"_id":"d","name":"Summer Saison","recipe":{"name":"Hazy IPA"}}`,
			importer.Batch{Ref: "d", Name: "Summer Saison", Status: "planned"}},
		{"name starting with default kept", `{"_id":"d","name":"Batch 7 Stout","recipe":{"name":"Hazy IPA"}}`,
			importer.Batch{Ref: "d", Name: "Batch 7 Stout", Status: "planned"}},
		{"nothing",
			`{"_id":"b4"}`,
			importer.Batch{Ref: "b4", Name: "Brewfather batch", Status: "planned"}},
		{"conditioning", `{"_id":"c","name":"C","status":"Conditioning"}`, importer.Batch{Ref: "c", Name: "C", Status: "conditioning"}},
		{"completed", `{"_id":"c","name":"C","status":"Completed"}`, importer.Batch{Ref: "c", Name: "C", Status: "ready"}},
		{"archived", `{"_id":"c","name":"C","status":"Archived"}`, importer.Batch{Ref: "c", Name: "C", Status: "ready", Archived: true}},
		{"unknown status", `{"_id":"c","name":"C","status":"Brewed"}`, importer.Batch{Ref: "c", Name: "C", Status: "planned"}},
		{"negative date", `{"_id":"c","name":"C","brewDate":-5}`, importer.Batch{Ref: "c", Name: "C", Status: "planned"}},
	}
	f := newFake(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f.handle = func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, c.json) }
			got, err := f.provider(t, 5*time.Second).GetBatch(context.Background(), "x")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				gj, _ := json.Marshal(got)
				wj, _ := json.Marshal(c.want)
				t.Fatalf("got  %s\nwant %s", gj, wj)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	status := func(code int) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			fmt.Fprint(w, `{"message":"`+bodyMarker+`"}`)
		}
	}
	cases := []struct {
		name   string
		handle func(http.ResponseWriter, *http.Request)
		get    error // GetBatch's error
		list   error // ListBatches' and Check's error
	}{
		{"401", status(401), importer.ErrCredentialsRefused, importer.ErrCredentialsRefused},
		{"403", status(403), importer.ErrNoBatchAccess, importer.ErrNoBatchAccess},
		{"404", status(404), importer.ErrBatchNotFound, importer.ErrUnreachable},
		{"500", status(500), importer.ErrUnreachable, importer.ErrUnreachable},
		{"malformed", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `[{"_id": `+bodyMarker) }, importer.ErrUnreachable, importer.ErrUnreachable},
		{"oversized", func(w http.ResponseWriter, r *http.Request) {
			// Valid JSON over 4 MiB, so only the size cap can refuse it.
			fmt.Fprint(w, `{"_id":"x","batchNotes":"`+strings.Repeat("a", 4<<20)+`"}`)
		}, importer.ErrUnreachable, importer.ErrUnreachable},
		{"redirect", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/elsewhere" {
				t.Errorf("redirect followed to %s", r.URL)
				return
			}
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}, importer.ErrUnreachable, importer.ErrUnreachable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.handle = c.handle
			p := f.provider(t, 5*time.Second)
			_, err := p.GetBatch(context.Background(), "b1")
			if !errors.Is(err, c.get) {
				t.Errorf("GetBatch = %v, want %v", err, c.get)
			}
			f.noLeak(t, err)
			_, err = p.ListBatches(context.Background())
			if !errors.Is(err, c.list) {
				t.Errorf("ListBatches = %v, want %v", err, c.list)
			}
			f.noLeak(t, err)
			err = p.Check(context.Background())
			if !errors.Is(err, c.list) {
				t.Errorf("Check = %v, want %v", err, c.list)
			}
			f.noLeak(t, err)
		})
	}
}

func TestRateLimited(t *testing.T) {
	future := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Second)
	cases := []struct {
		header string
		want   time.Duration // from now; 0 means the exact future date
	}{
		{"120", 120 * time.Second},
		{future.Format(http.TimeFormat), 0},
		{"", 60 * time.Second},
		{"soon", 60 * time.Second},
		{"-5", 60 * time.Second},
		{"86400", time.Hour},
		{time.Now().Add(48 * time.Hour).UTC().Format(http.TimeFormat), time.Hour},
	}
	for _, c := range cases {
		t.Run(c.header, func(t *testing.T) {
			f := newFake(t)
			f.handle = func(w http.ResponseWriter, r *http.Request) {
				if c.header != "" {
					w.Header().Set("Retry-After", c.header)
				}
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, bodyMarker)
			}
			before := time.Now()
			_, err := f.provider(t, 5*time.Second).GetBatch(context.Background(), "b1")
			var rl *importer.RateLimitedError
			if !errors.As(err, &rl) {
				t.Fatalf("err = %v, want RateLimitedError", err)
			}
			f.noLeak(t, err)
			want := before.Add(c.want)
			if c.want == 0 {
				want = future
			}
			if d := rl.Until.Sub(want); d < -2*time.Second || d > 2*time.Second {
				t.Fatalf("Until = %v, want about %v", rl.Until, want)
			}
		})
	}
}

func TestTimeoutAndCancel(t *testing.T) {
	f := newFake(t)
	f.delay = 5 * time.Second
	start := time.Now()
	_, err := f.provider(t, 50*time.Millisecond).ListBatches(context.Background())
	if !errors.Is(err, importer.ErrUnreachable) {
		t.Fatalf("slow server = %v, want ErrUnreachable", err)
	}
	f.noLeak(t, err)

	p := f.provider(t, 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := p.GetBatch(ctx, "b1"); !errors.Is(err, importer.ErrUnreachable) {
		t.Fatalf("expired context = %v, want ErrUnreachable", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	if err := p.Check(ctx); !errors.Is(err, importer.ErrUnreachable) {
		t.Fatalf("cancelled context = %v, want ErrUnreachable", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v; requests were not stopped", d)
	}
}

func TestNewWithRefusesPlainHTTP(t *testing.T) {
	for _, u := range []string{"http://api.brewfather.app/v2", "api.brewfather.app/v2", "https://", "ftp://x/v2", ":"} {
		if _, err := NewWith(u, http.DefaultClient, "u", "fake-key-"+rand.Text()); err == nil {
			t.Errorf("NewWith(%q) accepted", u)
		}
	}
}

func TestNewDefaults(t *testing.T) {
	c, ok := New("u", "fake-key-"+rand.Text()).(*client)
	if !ok {
		t.Fatal("New did not return *client")
	}
	if c.baseURL != "https://api.brewfather.app/v2" || c.http.Timeout != 15*time.Second || c.http.CheckRedirect == nil {
		t.Fatalf("defaults = %q timeout %v redirect %v", c.baseURL, c.http.Timeout, c.http.CheckRedirect != nil)
	}
	tr, ok := c.http.Transport.(*http.Transport)
	if !ok || tr.TLSClientConfig == nil || tr.TLSClientConfig.MinVersion < 0x0303 || tr.Proxy == nil {
		t.Fatalf("transport = %+v", c.http.Transport)
	}
}
