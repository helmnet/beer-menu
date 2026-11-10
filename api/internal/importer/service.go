package importer

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/helmnet/beer-menu/api/internal/secrets"
)

// sealPurpose binds the sealed API key to its column, so a sealed value moved elsewhere does
// not open.
const sealPurpose = providerName + ".api_key"

// interruptedMessage is the status of a run whose process stopped before it finished.
const interruptedMessage = "Sync was interrupted"

// Errors from Service methods besides the Provider errors. None carries a credential.
var (
	// ErrNotConfigured means the server has no encryption key, so nothing can be connected.
	ErrNotConfigured = errors.New("brewfather import is not configured")
	// ErrNotConnected means no account is connected.
	ErrNotConnected = errors.New("no account connected")
	// ErrReconnectNeeded means the stored key does not open with the server's current key.
	ErrReconnectNeeded = errors.New("stored credentials cannot be opened; reconnect")
	// ErrSyncRunning means a sync was asked for while another runs.
	ErrSyncRunning = errors.New("a sync is already running")
	// ErrNotImported means a re-sync was asked for a beer that did not come from the provider.
	ErrNotImported = errors.New("beer was not imported")
)

// Service is the import core: connection, status, rate gate, batch cache, import, re-sync and
// scheduled sync. It is safe for concurrent use.
type Service struct {
	DB *sql.DB
	// Box seals the API key; nil means the server has no encryption key (not configured).
	Box *secrets.Box
	// Factory builds the provider for the stored account.
	Factory Factory
	// Now is the clock; tests move it.
	Now func() time.Time

	mu sync.Mutex
	// blockedUntil is the rate gate: provider calls fail at once before it (research R4). In
	// memory only; a restart costs at most one more 429.
	blockedUntil time.Time
	// syncing is the sync lock, taken with CompareAndSwap so a second sync fails at once and
	// the status can read it without blocking.
	syncing atomic.Bool
	// background counts syncs started by StartSync that have not ended.
	background sync.WaitGroup
	// cache is the last ListBatches result, fetched at cacheAt (research R5). cacheGen counts
	// cache drops, so a list that was in flight across a connect or disconnect is not stored.
	cache    []Batch
	cacheAt  time.Time
	cacheGen uint64
}

// New returns a Service on db. box may be nil when no encryption key is configured.
func New(db *sql.DB, box *secrets.Box, factory Factory, now func() time.Time) *Service {
	return &Service{DB: db, Box: box, Factory: factory, Now: now}
}

// Status is the connection and sync status the API returns. It never holds the user ID or
// any part of the key.
type Status struct {
	Configured      bool       `json:"configured"`
	Connected       bool       `json:"connected"`
	ReconnectNeeded bool       `json:"reconnect_needed"`
	Interval        string     `json:"interval"`
	SyncRunning     bool       `json:"sync_running"`
	RetryAt         *time.Time `json:"retry_at"`
	LastSync        *LastSync  `json:"last_sync"`
}

// Status reads the current status. A stored run that never finished and is not running in
// this process reads as failed with "Sync was interrupted".
func (s *Service) Status(ctx context.Context) (Status, error) {
	st := Status{Configured: s.Box != nil, SyncRunning: s.syncing.Load()}
	if _, _, err := s.credentials(ctx); err == nil {
		st.Connected = true
	} else if errors.Is(err, ErrReconnectNeeded) {
		st.ReconnectNeeded = true
	} else if !errors.Is(err, ErrNotConfigured) && !errors.Is(err, ErrNotConnected) {
		return Status{}, err
	}
	var err error
	if st.Interval, err = loadInterval(s.DB); err != nil {
		return Status{}, err
	}
	if until := s.retryAt(); !until.IsZero() {
		st.RetryAt = &until
	}
	if st.LastSync, err = loadRun(s.DB); err != nil {
		return Status{}, err
	}
	if run := st.LastSync; run != nil && run.FinishedAt == nil && !st.SyncRunning {
		failed, msg := false, interruptedMessage
		run.OK, run.Message = &failed, &msg
	}
	return st, nil
}

// Connect checks userID and apiKey with one provider call and, only if it succeeds, stores
// them with the key sealed, replacing any earlier account. It returns ErrNotConfigured without
// a provider call when no encryption key is set, beers.FieldErrors for credentials outside the
// limits, a RateLimitedError while the rate gate is closed, and the provider's error when the
// check fails; on any error the stored account is unchanged.
func (s *Service) Connect(ctx context.Context, userID, apiKey string) error {
	if s.Box == nil {
		return ErrNotConfigured
	}
	userID, apiKey, err := cleanCredentials(userID, apiKey)
	if err != nil {
		return err
	}
	if err := s.gate(); err != nil {
		return err
	}
	if err := s.Factory(userID, apiKey).Check(ctx); err != nil {
		return s.observe(err)
	}
	sealed, err := s.Box.Seal(apiKey, sealPurpose)
	if err != nil {
		return err
	}
	if err := saveConnection(s.DB, userID, sealed, s.Now()); err != nil {
		return err
	}
	s.dropCache()
	slog.InfoContext(ctx, "brewfather connected", "provider", providerName)
	return nil
}

// Disconnect forgets the stored account. Imported beers, the interval and the last sync stay.
// Disconnecting with no account is not an error.
func (s *Service) Disconnect(ctx context.Context) error {
	if err := deleteConnection(s.DB); err != nil {
		return err
	}
	s.dropCache()
	slog.InfoContext(ctx, "brewfather disconnected", "provider", providerName)
	return nil
}

// dropCache forgets the cached batch list.
func (s *Service) dropCache() {
	s.mu.Lock()
	s.cache, s.cacheAt = nil, time.Time{}
	s.cacheGen++
	s.mu.Unlock()
}

// provider builds the Provider for the stored account, or returns ErrNotConfigured,
// ErrNotConnected or ErrReconnectNeeded.
func (s *Service) provider(ctx context.Context) (Provider, error) {
	userID, key, err := s.credentials(ctx)
	if err != nil {
		return nil, err
	}
	return s.Factory(userID, key), nil
}

// credentials loads and opens the stored account, or returns ErrNotConfigured,
// ErrNotConnected or ErrReconnectNeeded. Callers must never log or return what it gives.
func (s *Service) credentials(ctx context.Context) (userID, key string, err error) {
	if s.Box == nil {
		return "", "", ErrNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	userID, sealed, ok, err := loadConnection(s.DB)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", ErrNotConnected
	}
	if key, err = s.Box.Open(sealed, sealPurpose); err != nil {
		return "", "", ErrReconnectNeeded
	}
	return userID, key, nil
}

// gate returns a RateLimitedError while the provider's rate limit holds. Call it before every
// provider call.
func (s *Service) gate() error {
	if until := s.retryAt(); !until.IsZero() {
		return &RateLimitedError{Until: until}
	}
	return nil
}

// observe closes the gate when err is a RateLimitedError, and returns err unchanged. Call it
// on every provider error.
func (s *Service) observe(err error) error {
	var rl *RateLimitedError
	if errors.As(err, &rl) {
		s.mu.Lock()
		if rl.Until.After(s.blockedUntil) {
			s.blockedUntil = rl.Until
		}
		s.mu.Unlock()
	}
	return err
}

// retryAt returns when the gate opens, or the zero time when it is open.
func (s *Service) retryAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Now().Before(s.blockedUntil) {
		return s.blockedUntil
	}
	return time.Time{}
}
