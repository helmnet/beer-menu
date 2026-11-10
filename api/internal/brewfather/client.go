// Package brewfather is the only code that knows Brewfather's API: its URLs, JSON fields and
// batch statuses. It implements importer.Provider over HTTPS (research R1, R4) and maps
// batches to importer.Batch. Credentials, the Basic auth header and response bodies never
// appear in its errors.
package brewfather

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/helmnet/beer-menu/api/internal/importer"
)

const (
	defaultBaseURL = "https://api.brewfather.app/v2"
	pageSize       = 50
	// maxPages bounds one status's listing (10,000 batches), so a provider that never sends a
	// short page cannot keep a background sync looping and growing memory.
	maxPages = 200
	maxBody  = 4 << 20
	// include adds the mapped fields the list leaves out by default; the owner's real-account
	// probe confirmed every mapped field comes back with it (research R1).
	include = "recipe.style.name,recipe.abv,measuredAbv,estimatedIbu,estimatedColor,batchNotes,tasteNotes"
	// defaultRetry and maxRetry bound a 429's Retry-After (research R4).
	defaultRetry = time.Minute
	maxRetry     = time.Hour
)

// activeStatuses are listed one call each; Archived is never listed (spec clarification).
var activeStatuses = []string{"Planning", "Brewing", "Fermenting", "Conditioning", "Completed"}

var errUnreadable = fmt.Errorf("brewfather sent an unreadable response: %w", importer.ErrUnreachable)

// transport is shared by every client New builds, so reconnecting does not leak idle
// connection pools.
var transport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return t
}()

// client is an importer.Provider for one Brewfather account.
type client struct {
	baseURL      string
	http         *http.Client
	user, apiKey string
}

// New returns a Provider for the account userID with apiKey, calling
// https://api.brewfather.app/v2 with a 15 s timeout per request, TLS 1.2 or newer and no
// redirects. It matches importer.Factory.
func New(userID, apiKey string) importer.Provider {
	return newClient(defaultBaseURL, &http.Client{Timeout: 15 * time.Second, Transport: transport}, userID, apiKey)
}

// NewWith is New with another base URL and HTTP client, for tests against a TLS test server.
// It refuses a base URL that is not https, and never follows redirects whatever hc says.
func NewWith(baseURL string, hc *http.Client, userID, apiKey string) (importer.Provider, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("brewfather base URL must be https")
	}
	return newClient(baseURL, hc, userID, apiKey), nil
}

func newClient(baseURL string, hc *http.Client, userID, apiKey string) *client {
	h := *hc
	// The API never redirects; refusing keeps the Authorization header from going anywhere else.
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client{baseURL: baseURL, http: &h, user: userID, apiKey: apiKey}
}

// Check lists one batch to prove the credentials can read batches.
func (c *client) Check(ctx context.Context) error {
	var page []json.RawMessage
	return c.get(ctx, "/batches", url.Values{"limit": {"1"}}, &page)
}

// ListBatches returns every batch that is not archived: one paged listing per active status,
// all statuses at once (at most 5 requests in flight) so a slow server costs one timeout, not
// five. The first error stops the other listings.
func (c *client) ListBatches(ctx context.Context) ([]importer.Batch, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make([][]importer.Batch, len(activeStatuses))
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	for i, status := range activeStatuses {
		wg.Go(func() {
			bs, err := c.listStatus(ctx, status)
			if err != nil {
				once.Do(func() { firstErr = err; cancel() })
				return
			}
			results[i] = bs
		})
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	var all []importer.Batch
	for _, bs := range results {
		all = append(all, bs...)
	}
	return all, nil
}

// listStatus pages through one status with start_after until a short page, at most maxPages
// pages.
func (c *client) listStatus(ctx context.Context, status string) ([]importer.Batch, error) {
	var out []importer.Batch
	after := ""
	for range maxPages {
		q := url.Values{"status": {status}, "limit": {strconv.Itoa(pageSize)}, "include": {include}}
		if after != "" {
			q.Set("start_after", after)
		}
		var page []batchJSON
		if err := c.get(ctx, "/batches", q, &page); err != nil {
			return nil, err
		}
		for _, b := range page {
			out = append(out, b.toBatch())
		}
		if len(page) < pageSize {
			return out, nil
		}
		last := page[len(page)-1].ID
		// A full page that does not advance would loop forever and burn the quota.
		if last == "" || last == after {
			return nil, errUnreadable
		}
		after = last
	}
	return nil, errUnreadable
}

// GetBatch reads one batch, archived or not. An empty or dot id is not a batch ID and is never
// sent, since it would name another resource.
func (c *client) GetBatch(ctx context.Context, id string) (importer.Batch, error) {
	if id == "" || id == "." || id == ".." {
		return importer.Batch{}, importer.ErrBatchNotFound
	}
	var b batchJSON
	err := c.get(ctx, "/batches/"+url.PathEscape(id), url.Values{"include": {include}}, &b)
	if errors.Is(err, errNotFound) {
		return importer.Batch{}, importer.ErrBatchNotFound
	}
	if err != nil {
		return importer.Batch{}, err
	}
	return b.toBatch(), nil
}

var errNotFound = errors.New("brewfather: 404")

// get sends one authenticated GET and decodes at most maxBody bytes of JSON into dst. It
// returns importer errors, or errNotFound for a 404; never the body or the URL.
func (c *client) get(ctx context.Context, path string, q url.Values, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return importer.ErrUnreachable
	}
	req.SetBasicAuth(c.user, c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// The error names the URL and the cause; neither is useful to the owner.
		return importer.ErrUnreachable
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized:
		return importer.ErrCredentialsRefused
	case resp.StatusCode == http.StatusForbidden:
		return importer.ErrNoBatchAccess
	case resp.StatusCode == http.StatusNotFound && path != "/batches":
		return errNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		return &importer.RateLimitedError{Until: retryAt(resp.Header.Get("Retry-After"), time.Now())}
	default:
		return importer.ErrUnreachable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return importer.ErrUnreachable
	}
	if len(body) > maxBody || json.Unmarshal(body, dst) != nil {
		return errUnreadable
	}
	return nil
}

// retryAt reads Retry-After as seconds or an HTTP date. Absent, unreadable or not in the
// future gives one minute; more than an hour is capped at one hour.
func retryAt(header string, now time.Time) time.Time {
	d := defaultRetry
	if secs, err := strconv.Atoi(header); err == nil {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(header); err == nil {
		d = t.Sub(now)
	}
	if d <= 0 {
		d = defaultRetry
	}
	return now.Add(min(d, maxRetry))
}
