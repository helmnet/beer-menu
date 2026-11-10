// Package importertest holds a fake importer.Provider for tests in importer and httpapi. It is
// imported only by _test.go files.
package importertest

import (
	"context"
	"slices"
	"sync"

	"github.com/helmnet/beer-menu/api/internal/importer"
)

// Fake is a programmable importer.Provider: batches to serve, an error per call, a count of
// calls, and the credentials the factory was given. It is safe for concurrent use.
type Fake struct {
	mu      sync.Mutex
	batches []importer.Batch
	errs    map[string]error
	calls   map[string]int
	block   bool
	userID  string
	apiKey  string
}

// New returns an empty Fake.
func New() *Fake { return &Fake{errs: map[string]error{}, calls: map[string]int{}} }

// Factory returns an importer.Factory that records the credentials and serves f.
func (f *Fake) Factory() importer.Factory {
	return func(userID, apiKey string) importer.Provider {
		f.mu.Lock()
		f.userID, f.apiKey = userID, apiKey
		f.mu.Unlock()
		return f
	}
}

// Credentials returns the user ID and key the factory last got.
func (f *Fake) Credentials() (userID, apiKey string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.userID, f.apiKey
}

// SetBatches replaces the batches f serves, archived ones included.
func (f *Fake) SetBatches(bs ...importer.Batch) {
	f.mu.Lock()
	f.batches = slices.Clone(bs)
	f.mu.Unlock()
}

// SetErr makes call fail with err until changed; nil clears it. call is "Check",
// "ListBatches", "GetBatch" (every id) or "GetBatch:<id>" (one id, checked first).
func (f *Fake) SetErr(call string, err error) {
	f.mu.Lock()
	f.errs[call] = err
	f.mu.Unlock()
}

// SetBlock makes every call wait until its context ends and return importer.ErrUnreachable.
func (f *Fake) SetBlock(block bool) {
	f.mu.Lock()
	f.block = block
	f.mu.Unlock()
}

// Calls returns how many times call ("Check", "ListBatches" or "GetBatch") ran.
func (f *Fake) Calls(call string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[call]
}

// TotalCalls returns how many provider calls of any kind ran.
func (f *Fake) TotalCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

// start counts call and returns its programmed error, blocking first if asked.
func (f *Fake) start(ctx context.Context, call, id string) error {
	f.mu.Lock()
	f.calls[call]++
	block := f.block
	err := f.errs[call]
	if e, ok := f.errs[call+":"+id]; ok && id != "" {
		err = e
	}
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return importer.ErrUnreachable
	}
	return err
}

// Check implements importer.Provider.
func (f *Fake) Check(ctx context.Context) error { return f.start(ctx, "Check", "") }

// ListBatches implements importer.Provider: every batch not archived.
func (f *Fake) ListBatches(ctx context.Context) ([]importer.Batch, error) {
	if err := f.start(ctx, "ListBatches", ""); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []importer.Batch
	for _, b := range f.batches {
		if !b.Archived {
			out = append(out, b)
		}
	}
	return out, nil
}

// GetBatch implements importer.Provider: the batch with Ref id, or importer.ErrBatchNotFound.
func (f *Fake) GetBatch(ctx context.Context, id string) (importer.Batch, error) {
	if err := f.start(ctx, "GetBatch", id); err != nil {
		return importer.Batch{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.batches {
		if b.Ref == id {
			return b, nil
		}
	}
	return importer.Batch{}, importer.ErrBatchNotFound
}
