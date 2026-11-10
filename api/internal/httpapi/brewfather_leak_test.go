package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/importer"
	"github.com/helmnet/beer-menu/api/internal/store"
)

// lockedBuffer is a bytes.Buffer safe for the concurrent writes of a slog handler.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestBrewfatherSecretsNeverLeak runs connect, status, a failed connect, disconnect and
// reconnect, then list, import, re-sync, schedule and syncs, with failures, then checks that
// no response, log record, database file or backup holds the API key, and no response or log
// record holds the user ID or the Basic auth value (FR-005, FR-037, SC-004).
func TestBrewfatherSecretsNeverLeak(t *testing.T) {
	var logs lockedBuffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	deps, clock := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	fake := fakeProvider(deps)
	user, key := "fake-user-"+rand.Text(), "fake-key-"+rand.Text()
	secrets := map[string]string{"key": key, "user": user, "basic": base64.StdEncoding.EncodeToString([]byte(user + ":" + key))}
	body := `{"user_id":"` + user + `","api_key":"` + key + `"}`

	var recs []*httptest.ResponseRecorder
	send := func(method, path, body string, want int) {
		t.Helper()
		rec := do(mux, method, path, body, c.session...)
		if rec.Code != want {
			t.Fatalf("%s %s = %d %s, want %d", method, path, rec.Code, rec.Body, want)
		}
		recs = append(recs, rec)
	}
	send(http.MethodPut, "/api/brewfather/connection", body, 200)
	send(http.MethodGet, "/api/brewfather", "", 200)
	fake.SetErr("Check", importer.ErrCredentialsRefused)
	send(http.MethodPut, "/api/brewfather/connection", body, 400)
	fake.SetErr("Check", nil)
	send(http.MethodPut, "/api/brewfather/connection", `{"user_id":"`+user+`","api_key":"`+key+"\\u0000"+`"}`, 400)
	send(http.MethodDelete, "/api/brewfather/connection", "", 204)
	send(http.MethodPut, "/api/brewfather/connection", body, 200)
	send(http.MethodGet, "/api/brewfather", "", 200)

	// Every other route that opens the stored key: list, import, re-sync, schedule, a sync that
	// works and one that fails, and the rate-limited and unreachable answers.
	fake.SetBatches(importer.Batch{Ref: "leak-a", Name: "Pale", Status: "fermenting"},
		importer.Batch{Ref: "leak-b", Name: "Stout", Status: "conditioning"})
	send(http.MethodGet, "/api/brewfather/batches?refresh=true", "", 200)
	send(http.MethodPost, "/api/brewfather/imports", `{"batch_ids":["leak-a"]}`, 200)
	var imported struct {
		Results []struct {
			Beer struct{ ID int64 } `json:"beer"`
		} `json:"results"`
	}
	if err := json.Unmarshal(recs[len(recs)-1].Body.Bytes(), &imported); err != nil || len(imported.Results) != 1 {
		t.Fatalf("import answer: %v", err)
	}
	resync := "/api/beers/" + strconv.FormatInt(imported.Results[0].Beer.ID, 10) + "/sync"
	send(http.MethodPost, resync, "", 200)
	send(http.MethodPut, "/api/brewfather/schedule", `{"interval":"15m"}`, 200)
	send(http.MethodPost, "/api/brewfather/sync", "", 202)
	deps.Importer.Wait()
	fake.SetErr("ListBatches", importer.ErrCredentialsRefused)
	send(http.MethodPost, "/api/brewfather/sync", "", 202)
	deps.Importer.Wait()
	send(http.MethodGet, "/api/brewfather", "", 200)
	fake.SetErr("GetBatch", importer.ErrUnreachable)
	send(http.MethodPost, resync, "", 502)
	fake.SetErr("ListBatches", &importer.RateLimitedError{Until: clock.Now().Add(time.Minute)})
	send(http.MethodGet, "/api/brewfather/batches?refresh=true", "", 429)
	send(http.MethodPost, "/api/brewfather/sync", "", 429)

	for i, rec := range recs {
		var headers strings.Builder
		rec.Header().Write(&headers)
		for name, s := range secrets {
			if strings.Contains(rec.Body.String(), s) || strings.Contains(headers.String(), s) {
				t.Errorf("response %d holds the %s: %s", i, name, rec.Body)
			}
		}
	}
	captured := logs.String()
	for _, msg := range []string{"brewfather connected", "brewfather import", "brewfather sync"} {
		if !strings.Contains(captured, msg) {
			t.Fatalf("log capture saw no %q record: %s", msg, captured)
		}
	}
	for name, s := range secrets {
		if strings.Contains(captured, s) {
			t.Errorf("a log record holds the %s", name)
		}
	}

	var dbPath string
	if err := deps.DB.QueryRow("SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&dbPath); err != nil {
		t.Fatal(err)
	}
	backup, err := store.Backup(deps.DB, filepath.Dir(dbPath), 5, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	var all []byte
	for _, path := range []string{dbPath, dbPath + "-wal", backup} {
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) && path != backup {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte(key)) {
			t.Errorf("%s holds the API key", filepath.Base(path))
		}
		all = append(all, raw...)
	}
	// The user ID is stored in plain text, so finding it proves the scan read the stored row.
	if !bytes.Contains(all, []byte(user)) {
		t.Fatal("the scanned files do not hold the connection row")
	}
}
